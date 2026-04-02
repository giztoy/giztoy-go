package stores

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"path/filepath"
	"testing"

	"github.com/goccy/go-yaml"
	"github.com/giztoy/giztoy-go/pkg/graph"
	"github.com/giztoy/giztoy-go/pkg/kv"
)

type fakeDriver struct{}

func (fakeDriver) Open(_ string) (driver.Conn, error) { return fakeConn{}, nil }

type fakeConn struct{}

func (fakeConn) Prepare(_ string) (driver.Stmt, error) { return nil, nil }
func (fakeConn) Close() error                          { return nil }
func (fakeConn) Begin() (driver.Tx, error)             { return nil, nil }
func (fakeConn) Ping(_ context.Context) error          { return nil }

type fakePingFailDriver struct{}

func (fakePingFailDriver) Open(_ string) (driver.Conn, error) { return fakePingFailConn{}, nil }

type fakePingFailConn struct{}

func (fakePingFailConn) Prepare(_ string) (driver.Stmt, error) { return nil, nil }
func (fakePingFailConn) Close() error                          { return nil }
func (fakePingFailConn) Begin() (driver.Tx, error)             { return nil, nil }
func (fakePingFailConn) Ping(_ context.Context) error          { return errors.New("ping refused") }

func init() {
	sql.Register("fake", fakeDriver{})
	sql.Register("fake_ping_fail", fakePingFailDriver{})
}

func mustStores(t *testing.T, dataDir string, yml []byte) *Stores {
	t.Helper()
	var wrapper struct {
		Stores map[string]Config `yaml:"stores"`
	}
	if err := yaml.Unmarshal(yml, &wrapper); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	s, err := New(dataDir, wrapper.Stores)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return s
}

// --- New ---

func TestNewEmptyBaseDir(t *testing.T) {
	if _, err := New("", nil); err == nil {
		t.Fatal("expected error for empty baseDir")
	}
}

func TestNewNilConfigs(t *testing.T) {
	s, err := New(t.TempDir(), nil)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer s.Close()
	if _, err := s.KV("anything"); err == nil {
		t.Fatal("expected error for empty stores")
	}
}

func TestNewUnknownKind(t *testing.T) {
	if _, err := New(t.TempDir(), map[string]Config{
		"x": {Kind: "nosql", Backend: "magic"},
	}); err == nil {
		t.Fatal("expected error for unknown kind")
	}
}

func TestNewRelativeDir(t *testing.T) {
	dir := t.TempDir()
	s, err := New(dir, map[string]Config{
		"bg": {Kind: KindKeyValue, Backend: "badger", Dir: "bg-data"},
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer s.Close()
	if _, err := s.KV("bg"); err != nil {
		t.Fatalf("KV(bg): %v", err)
	}
}

// --- KV ---

func TestKVMemory(t *testing.T) {
	reg := mustStores(t, t.TempDir(), []byte(`
stores:
  mem:
    kind: keyvalue
    backend: memory
`))
	defer reg.Close()

	s, err := reg.KV("mem")
	if err != nil {
		t.Fatalf("KV(mem): %v", err)
	}

	ctx := context.Background()
	if err := s.Set(ctx, kv.Key{"a"}, []byte("1")); err != nil {
		t.Fatalf("Set: %v", err)
	}
	got, err := s.Get(ctx, kv.Key{"a"})
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if string(got) != "1" {
		t.Fatalf("Get = %q, want %q", got, "1")
	}

	s2, err := reg.KV("mem")
	if err != nil {
		t.Fatalf("KV(mem) second call: %v", err)
	}
	if s != s2 {
		t.Fatal("expected same instance on second call")
	}
}

func TestKVBadger(t *testing.T) {
	dir := t.TempDir()
	reg := mustStores(t, dir, []byte(`
stores:
  bg:
    kind: keyvalue
    backend: badger
    dir: bg-data
`))
	defer reg.Close()

	s, err := reg.KV("bg")
	if err != nil {
		t.Fatalf("KV(bg): %v", err)
	}
	ctx := context.Background()
	if err := s.Set(ctx, kv.Key{"k"}, []byte("v")); err != nil {
		t.Fatalf("Set: %v", err)
	}
	got, err := s.Get(ctx, kv.Key{"k"})
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if string(got) != "v" {
		t.Fatalf("Get = %q", got)
	}
}

func TestKVBadgerAbsoluteDir(t *testing.T) {
	absDir := filepath.Join(t.TempDir(), "abs-badger")
	reg := mustStores(t, t.TempDir(), []byte(`
stores:
  bg-abs:
    kind: keyvalue
    backend: badger
    dir: `+absDir+`
`))
	defer reg.Close()

	if _, err := reg.KV("bg-abs"); err != nil {
		t.Fatalf("KV(bg-abs): %v", err)
	}
}

func TestKVNotFound(t *testing.T) {
	reg := mustStores(t, t.TempDir(), []byte(`
stores:
  mem:
    kind: keyvalue
    backend: memory
  fs:
    kind: filestore
    backend: filesystem
    dir: data
`))
	defer reg.Close()

	if _, err := reg.KV("missing"); err == nil {
		t.Fatal("expected error for missing store")
	}
	if _, err := reg.KV("fs"); err == nil {
		t.Fatal("expected error for wrong kind lookup")
	}
}

func TestNewKVUnknownBackend(t *testing.T) {
	if _, err := New(t.TempDir(), map[string]Config{
		"x": {Kind: KindKeyValue, Backend: "redis"},
	}); err == nil {
		t.Fatal("expected error for unknown kv backend")
	}
}

func TestNewKVBadgerNoDir(t *testing.T) {
	if _, err := New(t.TempDir(), map[string]Config{
		"x": {Kind: KindKeyValue, Backend: "badger"},
	}); err == nil {
		t.Fatal("expected error for badger without dir")
	}
}

// --- VecStore ---

func TestVecStoreMemory(t *testing.T) {
	reg := mustStores(t, t.TempDir(), []byte(`
stores:
  vec:
    kind: vecstore
    backend: memory
`))
	defer reg.Close()

	idx, err := reg.VecStore("vec")
	if err != nil {
		t.Fatalf("VecStore(vec): %v", err)
	}
	if err := idx.Insert("a", []float32{1, 0, 0}); err != nil {
		t.Fatalf("Insert: %v", err)
	}
	if idx.Len() != 1 {
		t.Fatalf("Len = %d", idx.Len())
	}

	idx2, err := reg.VecStore("vec")
	if err != nil {
		t.Fatalf("VecStore(vec) second: %v", err)
	}
	if idx != idx2 {
		t.Fatal("expected same instance")
	}
}

func TestVecStoreNotFound(t *testing.T) {
	reg := mustStores(t, t.TempDir(), []byte(`
stores:
  kv:
    kind: keyvalue
    backend: memory
`))
	defer reg.Close()

	if _, err := reg.VecStore("missing"); err == nil {
		t.Fatal("expected error for missing")
	}
	if _, err := reg.VecStore("kv"); err == nil {
		t.Fatal("expected error for wrong kind lookup")
	}
}

func TestNewVecStoreUnknownBackend(t *testing.T) {
	if _, err := New(t.TempDir(), map[string]Config{
		"x": {Kind: KindVecStore, Backend: "qdrant"},
	}); err == nil {
		t.Fatal("expected error for unknown vecstore backend")
	}
}

// --- Graph ---

func TestGraphKV(t *testing.T) {
	reg := mustStores(t, t.TempDir(), []byte(`
stores:
  mem:
    kind: keyvalue
    backend: memory
  g:
    kind: graph
    backend: kv
    store: mem
`))
	defer reg.Close()

	g, err := reg.Graph("g")
	if err != nil {
		t.Fatalf("Graph(g): %v", err)
	}
	ctx := context.Background()
	if err := g.SetEntity(ctx, graph.Entity{Label: "alice"}); err != nil {
		t.Fatalf("SetEntity: %v", err)
	}
	e, err := g.GetEntity(ctx, "alice")
	if err != nil {
		t.Fatalf("GetEntity: %v", err)
	}
	if e.Label != "alice" {
		t.Fatalf("Label = %q", e.Label)
	}

	g2, err := reg.Graph("g")
	if err != nil {
		t.Fatalf("Graph(g) second: %v", err)
	}
	if g != g2 {
		t.Fatal("expected same instance")
	}
}

func TestGraphNotFound(t *testing.T) {
	reg := mustStores(t, t.TempDir(), []byte(`
stores:
  kv:
    kind: keyvalue
    backend: memory
`))
	defer reg.Close()

	if _, err := reg.Graph("missing"); err == nil {
		t.Fatal("expected error for missing")
	}
	if _, err := reg.Graph("kv"); err == nil {
		t.Fatal("expected error for wrong kind lookup")
	}
}

func TestNewGraphNoStoreRef(t *testing.T) {
	if _, err := New(t.TempDir(), map[string]Config{
		"g": {Kind: KindGraph, Backend: "kv"},
	}); err == nil {
		t.Fatal("expected error for missing store reference")
	}
}

func TestNewGraphBadStoreRef(t *testing.T) {
	if _, err := New(t.TempDir(), map[string]Config{
		"g": {Kind: KindGraph, Backend: "kv", Store: "nonexistent"},
	}); err == nil {
		t.Fatal("expected error for undefined kv reference")
	}
}

func TestNewGraphWrongKindRef(t *testing.T) {
	if _, err := New(t.TempDir(), map[string]Config{
		"vec": {Kind: KindVecStore, Backend: "memory"},
		"g":   {Kind: KindGraph, Backend: "kv", Store: "vec"},
	}); err == nil {
		t.Fatal("expected error for kv ref pointing at non-kv store")
	}
}

func TestNewGraphUnknownBackend(t *testing.T) {
	if _, err := New(t.TempDir(), map[string]Config{
		"g": {Kind: KindGraph, Backend: "neo4j"},
	}); err == nil {
		t.Fatal("expected error for unknown graph backend")
	}
}

// --- FS ---

func TestFS(t *testing.T) {
	dir := t.TempDir()
	reg := mustStores(t, dir, []byte(`
stores:
  fw:
    kind: filestore
    backend: filesystem
    dir: firmware
`))
	defer reg.Close()

	s, err := reg.FS("fw")
	if err != nil {
		t.Fatalf("FS(fw): %v", err)
	}
	want := filepath.Join(dir, "firmware")
	if s.Root() != want {
		t.Fatalf("Root = %q, want %q", s.Root(), want)
	}

	s2, err := reg.FS("fw")
	if err != nil {
		t.Fatalf("FS(fw) second: %v", err)
	}
	if s != s2 {
		t.Fatal("expected same instance")
	}
}

func TestFSNotFound(t *testing.T) {
	reg := mustStores(t, t.TempDir(), []byte(`
stores:
  kv:
    kind: keyvalue
    backend: memory
`))
	defer reg.Close()

	if _, err := reg.FS("missing"); err == nil {
		t.Fatal("expected error for missing")
	}
	if _, err := reg.FS("kv"); err == nil {
		t.Fatal("expected error for wrong kind lookup")
	}
}

func TestNewFSNoDir(t *testing.T) {
	if _, err := New(t.TempDir(), map[string]Config{
		"x": {Kind: KindFS, Backend: "filesystem"},
	}); err == nil {
		t.Fatal("expected error for filesystem without dir")
	}
}

func TestNewFSUnknownBackend(t *testing.T) {
	if _, err := New(t.TempDir(), map[string]Config{
		"x": {Kind: KindFS, Backend: "s3"},
	}); err == nil {
		t.Fatal("expected error for unknown fs backend")
	}
}

// --- SQL ---

func TestSQL(t *testing.T) {
	reg := mustStores(t, t.TempDir(), []byte(`
stores:
  db:
    kind: sql
    backend: fake
    dsn: test
`))
	defer reg.Close()

	db, err := reg.SQL("db")
	if err != nil {
		t.Fatalf("SQL(db): %v", err)
	}
	if db == nil {
		t.Fatal("expected non-nil *sql.DB")
	}

	db2, err := reg.SQL("db")
	if err != nil {
		t.Fatalf("SQL(db) second: %v", err)
	}
	if db != db2 {
		t.Fatal("expected same instance")
	}
}

func TestSQLWithDSN(t *testing.T) {
	reg := mustStores(t, t.TempDir(), []byte(`
stores:
  db:
    kind: sql
    backend: fake
    dsn: mydb
`))
	defer reg.Close()

	db, err := reg.SQL("db")
	if err != nil {
		t.Fatalf("SQL(db) with dsn: %v", err)
	}
	if db == nil {
		t.Fatal("expected non-nil *sql.DB")
	}
}

func TestSQLNotFound(t *testing.T) {
	reg := mustStores(t, t.TempDir(), []byte(`
stores:
  kv:
    kind: keyvalue
    backend: memory
`))
	defer reg.Close()

	if _, err := reg.SQL("missing"); err == nil {
		t.Fatal("expected error for missing")
	}
	if _, err := reg.SQL("kv"); err == nil {
		t.Fatal("expected error for wrong kind lookup")
	}
}

func TestNewSQLNoBackend(t *testing.T) {
	if _, err := New(t.TempDir(), map[string]Config{
		"x": {Kind: KindSQL, DSN: "x"},
	}); err == nil {
		t.Fatal("expected error for empty backend")
	}
}

func TestNewSQLNoDSN(t *testing.T) {
	if _, err := New(t.TempDir(), map[string]Config{
		"x": {Kind: KindSQL, Backend: "fake"},
	}); err == nil {
		t.Fatal("expected error for missing dsn")
	}
}

func TestNewSQLBadDriver(t *testing.T) {
	if _, err := New(t.TempDir(), map[string]Config{
		"x": {Kind: KindSQL, Backend: "nonexistent_driver", DSN: "x"},
	}); err == nil {
		t.Fatal("expected error for unregistered driver")
	}
}

func TestNewSQLPingFail(t *testing.T) {
	if _, err := New(t.TempDir(), map[string]Config{
		"x": {Kind: KindSQL, Backend: "fake_ping_fail", DSN: "x"},
	}); err == nil {
		t.Fatal("expected error for ping failure")
	}
}

// --- Close ---

func TestCloseOrder(t *testing.T) {
	dir := t.TempDir()
	reg := mustStores(t, dir, []byte(`
stores:
  bg:
    kind: keyvalue
    backend: badger
    dir: close-test
  g:
    kind: graph
    backend: kv
    store: bg
`))

	if err := reg.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
}

func TestCloseEmpty(t *testing.T) {
	s, err := New(t.TempDir(), nil)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if err := s.Close(); err != nil {
		t.Fatalf("Close empty: %v", err)
	}
}

// --- resolveDir ---

func TestResolveDir(t *testing.T) {
	base := filepath.Join(string(filepath.Separator), "base")
	if got := resolveDir(base, "rel"); got != filepath.Join(base, "rel") {
		t.Fatalf("resolveDir relative = %q", got)
	}
	abs, err := filepath.Abs(filepath.Join("abs", "path"))
	if err != nil {
		t.Fatalf("Abs: %v", err)
	}
	if got := resolveDir(base, abs); got != abs {
		t.Fatalf("resolveDir absolute = %q", got)
	}
}
