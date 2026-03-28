// Package stores provides a configuration-driven registry that creates and
// manages store instances (kv, vecstore, graph, sql, fs). Named stores are
// defined in config.yaml under the top-level "stores:" key and eagerly
// instantiated in New. The Stores guarantees that each named store is created
// at most once and provides a single Close to release all resources.
package stores

import (
	"database/sql"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/haivivi/giztoy/go/pkg/firmware"
	"github.com/haivivi/giztoy/go/pkg/graph"
	"github.com/haivivi/giztoy/go/pkg/kv"
	"github.com/haivivi/giztoy/go/pkg/vecstore"
)

// Kind constants for the store category.
const (
	KindKeyValue = "keyvalue"
	KindVecStore = "vecstore"
	KindGraph    = "graph"
	KindFS       = "filestore"
	KindSQL      = "sql"
)

// Config is the YAML representation of a single named store entry.
//
//	stores:
//	  my-badger:
//	    kind: keyvalue
//	    backend: badger
//	    dir: badger-data
type Config struct {
	Kind    string `yaml:"kind"`
	Backend string `yaml:"backend"`
	Dir     string `yaml:"dir"`
	Store   string `yaml:"store"` // graph backend "kv": reference to a keyvalue store
	Dim     int    `yaml:"dim"`   // vecstore: vector dimension (reserved)
	DSN     string `yaml:"dsn"`   // sql: connection string (reserved)
}

// Stores holds named store instances created eagerly by New.
type Stores struct {
	kvs     map[string]kv.Store
	vecs    map[string]vecstore.Index
	graphs  map[string]graph.Graph
	sqls    map[string]*sql.DB
	fss     map[string]*firmware.Store
	closers []io.Closer
}

// New creates a Stores instance and eagerly instantiates every configured
// store. baseDir must be a non-empty absolute path; relative Dir fields in
// configs are resolved against it. Graph stores are created after all other
// stores so that kv references are available.
func New(baseDir string, configs map[string]Config) (*Stores, error) {
	if baseDir == "" {
		return nil, fmt.Errorf("stores: baseDir must not be empty")
	}
	s := &Stores{
		kvs:    make(map[string]kv.Store),
		vecs:   make(map[string]vecstore.Index),
		graphs: make(map[string]graph.Graph),
		sqls:   make(map[string]*sql.DB),
		fss:    make(map[string]*firmware.Store),
	}
	ok := false
	defer func() {
		if !ok {
			s.Close()
		}
	}()

	var graphCfgs []struct {
		name string
		cfg  Config
	}
	for name, cfg := range configs {
		switch cfg.Kind {
		case KindKeyValue:
			st, err := s.newKV(baseDir, name, cfg)
			if err != nil {
				return nil, err
			}
			s.kvs[name] = st
			s.closers = append(s.closers, st)
		case KindVecStore:
			st, err := s.newVecStore(name, cfg)
			if err != nil {
				return nil, err
			}
			s.vecs[name] = st
			s.closers = append(s.closers, st)
		case KindSQL:
			st, err := s.newSQL(baseDir, name, cfg)
			if err != nil {
				return nil, err
			}
			s.sqls[name] = st
			s.closers = append(s.closers, st)
		case KindFS:
			st, err := s.newFS(baseDir, name, cfg)
			if err != nil {
				return nil, err
			}
			s.fss[name] = st
			s.closers = append(s.closers, st)
		case KindGraph:
			graphCfgs = append(graphCfgs, struct {
				name string
				cfg  Config
			}{name, cfg})
		default:
			return nil, fmt.Errorf("stores: %q has unknown kind %q", name, cfg.Kind)
		}
	}

	for _, g := range graphCfgs {
		st, err := s.newGraph(g.name, g.cfg)
		if err != nil {
			return nil, err
		}
		s.graphs[g.name] = st
		s.closers = append(s.closers, st)
	}

	ok = true
	return s, nil
}

// KV returns the named kv.Store.
func (r *Stores) KV(name string) (kv.Store, error) {
	s, ok := r.kvs[name]
	if !ok {
		return nil, fmt.Errorf("stores: kv %q not found", name)
	}
	return s, nil
}

// VecStore returns the named vecstore.Index.
func (r *Stores) VecStore(name string) (vecstore.Index, error) {
	s, ok := r.vecs[name]
	if !ok {
		return nil, fmt.Errorf("stores: vecstore %q not found", name)
	}
	return s, nil
}

// Graph returns the named graph.Graph.
func (r *Stores) Graph(name string) (graph.Graph, error) {
	s, ok := r.graphs[name]
	if !ok {
		return nil, fmt.Errorf("stores: graph %q not found", name)
	}
	return s, nil
}

// SQL returns the named *sql.DB.
func (r *Stores) SQL(name string) (*sql.DB, error) {
	s, ok := r.sqls[name]
	if !ok {
		return nil, fmt.Errorf("stores: sql %q not found", name)
	}
	return s, nil
}

// FS returns the named firmware.Store.
func (r *Stores) FS(name string) (*firmware.Store, error) {
	s, ok := r.fss[name]
	if !ok {
		return nil, fmt.Errorf("stores: filestore %q not found", name)
	}
	return s, nil
}

// Close releases all created stores in reverse creation order.
func (r *Stores) Close() error {
	var errs []error
	for i := len(r.closers) - 1; i >= 0; i-- {
		if err := r.closers[i].Close(); err != nil {
			errs = append(errs, err)
		}
	}
	r.closers = nil
	return errors.Join(errs...)
}

// --- factory methods ---

func (r *Stores) newKV(baseDir, name string, cfg Config) (kv.Store, error) {
	switch cfg.Backend {
	case "memory":
		return kv.NewMemory(nil), nil
	case "badger":
		if cfg.Dir == "" {
			return nil, fmt.Errorf("stores: keyvalue %q (badger) requires dir", name)
		}
		dir := resolveDir(baseDir, cfg.Dir)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, fmt.Errorf("stores: keyvalue %q mkdir: %w", name, err)
		}
		return kv.NewBadger(dir, nil)
	default:
		return nil, fmt.Errorf("stores: keyvalue %q unknown backend %q", name, cfg.Backend)
	}
}

func (r *Stores) newVecStore(name string, cfg Config) (vecstore.Index, error) {
	switch cfg.Backend {
	case "memory":
		return vecstore.NewMemory(), nil
	default:
		return nil, fmt.Errorf("stores: vecstore %q unknown backend %q", name, cfg.Backend)
	}
}

func (r *Stores) newGraph(name string, cfg Config) (graph.Graph, error) {
	switch cfg.Backend {
	case "kv":
		if cfg.Store == "" {
			return nil, fmt.Errorf("stores: graph %q (kv) requires store reference", name)
		}
		kvStore, err := r.kvByName(cfg.Store)
		if err != nil {
			return nil, fmt.Errorf("stores: graph %q resolve kv %q: %w", name, cfg.Store, err)
		}
		return graph.NewKVGraph(kvStore, kv.Key{name}), nil
	default:
		return nil, fmt.Errorf("stores: graph %q unknown backend %q", name, cfg.Backend)
	}
}

func (r *Stores) newFS(baseDir, name string, cfg Config) (*firmware.Store, error) {
	switch cfg.Backend {
	case "filesystem":
		if cfg.Dir == "" {
			return nil, fmt.Errorf("stores: filestore %q (filesystem) requires dir", name)
		}
		return firmware.NewStore(resolveDir(baseDir, cfg.Dir)), nil
	default:
		return nil, fmt.Errorf("stores: filestore %q unknown backend %q", name, cfg.Backend)
	}
}

func (r *Stores) newSQL(baseDir, name string, cfg Config) (*sql.DB, error) {
	if cfg.Backend == "" {
		return nil, fmt.Errorf("stores: sql %q requires backend (driver name)", name)
	}
	dsn := cfg.DSN
	if cfg.Backend == "sqlite" && dsn == "" && cfg.Dir != "" {
		dsn = resolveDir(baseDir, cfg.Dir)
	}
	if dsn == "" {
		return nil, fmt.Errorf("stores: sql %q requires dsn", name)
	}
	db, err := sql.Open(cfg.Backend, dsn)
	if err != nil {
		return nil, fmt.Errorf("stores: sql %q open: %w", name, err)
	}
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("stores: sql %q ping: %w", name, err)
	}
	return db, nil
}

func resolveDir(baseDir, dir string) string {
	if filepath.IsAbs(dir) {
		return dir
	}
	return filepath.Join(baseDir, dir)
}

func (r *Stores) kvByName(name string) (kv.Store, error) {
	s, ok := r.kvs[name]
	if !ok {
		return nil, fmt.Errorf("stores: kv %q not found", name)
	}
	return s, nil
}
