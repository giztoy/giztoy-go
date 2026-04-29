// Package storage provides a configuration-driven registry for physical
// storage backends. Logical stores can build scoped views on top of these
// backend instances.
package storage

import (
	"database/sql"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/GizClaw/gizclaw-go/pkg/store/depotstore"
	"github.com/GizClaw/gizclaw-go/pkg/store/kv"
	"github.com/GizClaw/gizclaw-go/pkg/store/vecstore"

	_ "github.com/lib/pq"
	_ "modernc.org/sqlite"
)

// Kind constants for physical storage categories.
const (
	KindKeyValue   = "keyvalue"
	KindVecStore   = "vecstore"
	KindFilesystem = "filesystem"
	KindDepotStore = "depotstore"
	KindSQL        = "sql"

	// KindFS is the legacy one-layer firmware store kind.
	KindFS = "filestore"
)

// Config is the YAML representation of a physical storage backend.
//
//	storage:
//	  main-kv:
//	    kind: keyvalue
//	    badger:
//	      dir: data/kv
type Config struct {
	Kind          string         `yaml:"kind"`
	Memory        *MemoryConfig  `yaml:"memory"`
	Badger        *BadgerConfig  `yaml:"badger"`
	FS            *FSConfig      `yaml:"fs"`
	SQLite        *SQLConfig     `yaml:"sqlite"`
	Postgres      *SQLConfig     `yaml:"postgres"`
	DepotFS       *DepotFSConfig `yaml:"depot-fs"`
	LegacyDepotFS *DepotFSConfig `yaml:"depot-kvfs"`
	Backend       string         `yaml:"backend"` // legacy driver field
	Dir           string         `yaml:"dir"`     // legacy driver dir field
	Dim           int            `yaml:"dim"`     // legacy vecstore dimension field
	DSN           string         `yaml:"dsn"`     // legacy sql connection string field
}

type MemoryConfig struct{}

type BadgerConfig struct {
	Dir string `yaml:"dir"`
}

type FSConfig struct {
	Dir string `yaml:"dir"`
}

type SQLConfig struct {
	DSN string `yaml:"dsn"`
	Dir string `yaml:"dir"`
}

type FilesystemRef struct {
	Storage string `yaml:"storage"`
	BaseDir string `yaml:"base-dir"`
}

type DepotFSConfig struct{}

// Storage holds physical backend instances created eagerly by New.
type Storage struct {
	kvs          map[string]kv.Store
	vecs         map[string]vecstore.Index
	sqls         map[string]*sql.DB
	fss          map[string]depotstore.Dir
	depots       map[string]depotstore.Store
	depotDrivers map[string]string
	closers      []io.Closer
}

// New creates a Storage registry and eagerly instantiates every configured
// physical backend. Dir fields are used as provided by the caller.
func New(configs map[string]Config) (*Storage, error) {
	s := &Storage{
		kvs:          make(map[string]kv.Store),
		vecs:         make(map[string]vecstore.Index),
		sqls:         make(map[string]*sql.DB),
		fss:          make(map[string]depotstore.Dir),
		depots:       make(map[string]depotstore.Store),
		depotDrivers: make(map[string]string),
	}
	ok := false
	defer func() {
		if !ok {
			s.Close()
		}
	}()

	states := make(map[string]buildState, len(configs))
	for name := range configs {
		if err := s.build(name, configs, states); err != nil {
			return nil, err
		}
	}

	ok = true
	return s, nil
}

// KV returns the named physical key-value backend.
func (s *Storage) KV(name string) (kv.Store, error) {
	st, ok := s.kvs[name]
	if !ok {
		return nil, fmt.Errorf("storage: kv %q not found", name)
	}
	return st, nil
}

// VecStore returns the named physical vector store backend.
func (s *Storage) VecStore(name string) (vecstore.Index, error) {
	st, ok := s.vecs[name]
	if !ok {
		return nil, fmt.Errorf("storage: vecstore %q not found", name)
	}
	return st, nil
}

// SQL returns the named physical SQL backend.
func (s *Storage) SQL(name string) (*sql.DB, error) {
	st, ok := s.sqls[name]
	if !ok {
		return nil, fmt.Errorf("storage: sql %q not found", name)
	}
	return st, nil
}

// FS returns the named physical file store backend.
func (s *Storage) FS(name string) (depotstore.Dir, error) {
	return s.Filesystem(name)
}

// Filesystem returns the named physical filesystem backend.
func (s *Storage) Filesystem(name string) (depotstore.Dir, error) {
	st, ok := s.fss[name]
	if !ok {
		return "", fmt.Errorf("storage: filesystem %q not found", name)
	}
	return st, nil
}

// DepotStore returns the named physical depot store backend.
// It is kept for legacy filestore configs. New depotstore configs are assembled
// by the logical stores layer from a depot driver plus backend refs.
func (s *Storage) DepotStore(name string) (depotstore.Store, error) {
	st, ok := s.depots[name]
	if !ok {
		return nil, fmt.Errorf("storage: depotstore %q not found", name)
	}
	return st, nil
}

func (s *Storage) DepotDriver(name string) (string, error) {
	driver, ok := s.depotDrivers[name]
	if !ok {
		return "", fmt.Errorf("storage: depotstore %q not found", name)
	}
	return driver, nil
}

// Close releases all opened physical backends in reverse creation order.
func (s *Storage) Close() error {
	var errs []error
	for i := len(s.closers) - 1; i >= 0; i-- {
		if err := s.closers[i].Close(); err != nil {
			errs = append(errs, err)
		}
	}
	s.closers = nil
	return errors.Join(errs...)
}

type buildState uint8

const (
	building buildState = 1
	built    buildState = 2
)

func (s *Storage) build(name string, configs map[string]Config, states map[string]buildState) error {
	switch states[name] {
	case built:
		return nil
	case building:
		return fmt.Errorf("storage: dependency cycle at %q", name)
	}
	cfg, ok := configs[name]
	if !ok {
		return fmt.Errorf("storage: %q not configured", name)
	}
	states[name] = building
	var err error
	switch cfg.Kind {
	case KindKeyValue:
		var st kv.Store
		st, err = newKV(name, cfg)
		if err == nil {
			s.kvs[name] = st
			s.closers = append(s.closers, st)
		}
	case KindVecStore:
		var st vecstore.Index
		st, err = newVecStore(name, cfg)
		if err == nil {
			s.vecs[name] = st
			s.closers = append(s.closers, st)
		}
	case KindFilesystem:
		var st depotstore.Dir
		st, err = newFilesystem(name, cfg)
		if err == nil {
			s.fss[name] = st
		}
	case KindDepotStore:
		var driver string
		driver, err = newDepotDriver(name, cfg)
		if err == nil {
			s.depotDrivers[name] = driver
		}
	case KindFS:
		var st depotstore.Dir
		st, err = newLegacyFS(name, cfg)
		if err == nil {
			s.fss[name] = st
			s.depots[name] = st
		}
	case KindSQL:
		var st *sql.DB
		st, err = newSQL(name, cfg)
		if err == nil {
			s.sqls[name] = st
			s.closers = append(s.closers, st)
		}
	default:
		err = fmt.Errorf("storage: %q has unknown kind %q", name, cfg.Kind)
	}
	if err != nil {
		return err
	}
	states[name] = built
	return nil
}

func newKV(name string, cfg Config) (kv.Store, error) {
	if blocks := driverBlocks(cfg); len(blocks) > 0 {
		if err := validateDriverBlocks(name, KindKeyValue, blocks, "memory", "badger"); err != nil {
			return nil, err
		}
		switch {
		case cfg.Memory != nil:
			return kv.NewBadgerInMemory(nil)
		case cfg.Badger != nil:
			return newBadgerKV(name, cfg.Badger.Dir)
		}
	}
	switch cfg.Backend {
	case "memory":
		return kv.NewBadgerInMemory(nil)
	case "badger":
		return newBadgerKV(name, cfg.Dir)
	case "":
		return nil, fmt.Errorf("storage: keyvalue %q requires driver", name)
	default:
		return nil, fmt.Errorf("storage: keyvalue %q unknown backend %q", name, cfg.Backend)
	}
}

func newBadgerKV(name, dir string) (kv.Store, error) {
	if dir == "" {
		return nil, fmt.Errorf("storage: keyvalue %q (badger) requires dir", name)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("storage: keyvalue %q mkdir: %w", name, err)
	}
	return kv.NewBadger(dir, nil)
}

func newVecStore(name string, cfg Config) (vecstore.Index, error) {
	if blocks := driverBlocks(cfg); len(blocks) > 0 {
		if err := validateDriverBlocks(name, KindVecStore, blocks, "memory"); err != nil {
			return nil, err
		}
		return vecstore.NewMemory(), nil
	}
	switch cfg.Backend {
	case "memory":
		return vecstore.NewMemory(), nil
	case "":
		return nil, fmt.Errorf("storage: vecstore %q requires driver", name)
	default:
		return nil, fmt.Errorf("storage: vecstore %q unknown backend %q", name, cfg.Backend)
	}
}

func newFilesystem(name string, cfg Config) (depotstore.Dir, error) {
	if blocks := driverBlocks(cfg); len(blocks) > 0 {
		if err := validateDriverBlocks(name, KindFilesystem, blocks, "fs"); err != nil {
			return "", err
		}
	}
	if cfg.FS == nil {
		return "", fmt.Errorf("storage: filesystem %q requires fs driver", name)
	}
	return newFilesystemDir(name, cfg.FS.Dir)
}

func newLegacyFS(name string, cfg Config) (depotstore.Dir, error) {
	switch cfg.Backend {
	case "filesystem":
		return newFilesystemDir(name, cfg.Dir)
	case "":
		return "", fmt.Errorf("storage: filestore %q requires backend", name)
	default:
		return "", fmt.Errorf("storage: filestore %q unknown backend %q", name, cfg.Backend)
	}
}

func newFilesystemDir(name, dir string) (depotstore.Dir, error) {
	if dir == "" {
		return "", fmt.Errorf("storage: filesystem %q (fs) requires dir", name)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("storage: filesystem %q mkdir: %w", name, err)
	}
	return depotstore.Dir(dir), nil
}

func newDepotDriver(name string, cfg Config) (string, error) {
	if blocks := driverBlocks(cfg); len(blocks) > 0 {
		if err := validateDriverBlocks(name, KindDepotStore, blocks, "depot-fs"); err != nil {
			return "", err
		}
	}
	if cfg.DepotFS == nil && cfg.LegacyDepotFS == nil {
		return "", fmt.Errorf("storage: depotstore %q requires depot-fs driver", name)
	}
	return "depot-fs", nil
}

func newSQL(name string, cfg Config) (*sql.DB, error) {
	if blocks := driverBlocks(cfg); len(blocks) > 0 {
		if err := validateDriverBlocks(name, KindSQL, blocks, "sqlite", "postgres"); err != nil {
			return nil, err
		}
	}
	backend, dsn := sqlDriverConfig(cfg)
	if backend == "" {
		return nil, fmt.Errorf("storage: sql %q requires backend (driver name)", name)
	}
	if dsn == "" {
		return nil, fmt.Errorf("storage: sql %q requires dsn", name)
	}
	db, err := sql.Open(backend, dsn)
	if err != nil {
		return nil, fmt.Errorf("storage: sql %q open: %w", name, err)
	}
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("storage: sql %q ping: %w", name, err)
	}
	return db, nil
}

func driverBlocks(cfg Config) []string {
	var blocks []string
	if cfg.Memory != nil {
		blocks = append(blocks, "memory")
	}
	if cfg.Badger != nil {
		blocks = append(blocks, "badger")
	}
	if cfg.FS != nil {
		blocks = append(blocks, "fs")
	}
	if cfg.SQLite != nil {
		blocks = append(blocks, "sqlite")
	}
	if cfg.Postgres != nil {
		blocks = append(blocks, "postgres")
	}
	if cfg.LegacyDepotFS != nil {
		blocks = append(blocks, "depot-fs")
	}
	if cfg.DepotFS != nil {
		blocks = append(blocks, "depot-fs")
	}
	return blocks
}

func validateDriverBlocks(name, kind string, blocks []string, allowed ...string) error {
	allowedSet := make(map[string]struct{}, len(allowed))
	for _, driver := range allowed {
		allowedSet[driver] = struct{}{}
	}
	for _, driver := range blocks {
		if _, ok := allowedSet[driver]; !ok {
			return fmt.Errorf("storage: %s %q does not support %s driver", kind, name, driver)
		}
	}
	if len(blocks) != 1 {
		return fmt.Errorf("storage: %s %q requires exactly one driver, got %s", kind, name, strings.Join(blocks, ", "))
	}
	return nil
}

func sqlDriverConfig(cfg Config) (string, string) {
	if cfg.SQLite != nil {
		if cfg.SQLite.DSN != "" {
			return "sqlite", cfg.SQLite.DSN
		}
		return "sqlite", cfg.SQLite.Dir
	}
	if cfg.Postgres != nil {
		return "postgres", cfg.Postgres.DSN
	}
	dsn := cfg.DSN
	if cfg.Backend == "sqlite" && dsn == "" && cfg.Dir != "" {
		dsn = cfg.Dir
	}
	return cfg.Backend, dsn
}
