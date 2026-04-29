package storage

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"path/filepath"
	"testing"

	"github.com/GizClaw/gizclaw-go/pkg/store/kv"
)

type fakeDriver struct{}

func (fakeDriver) Open(_ string) (driver.Conn, error) { return fakeConn{}, nil }

type fakeConn struct{}

func (fakeConn) Prepare(_ string) (driver.Stmt, error) { return nil, nil }
func (fakeConn) Close() error                          { return nil }
func (fakeConn) Begin() (driver.Tx, error)             { return nil, nil }
func (fakeConn) Ping(_ context.Context) error          { return nil }

type fakePingFailDriver struct{}

func (fakePingFailDriver) Open(_ string) (driver.Conn, error) {
	return fakePingFailConn{}, nil
}

type fakePingFailConn struct{}

func (fakePingFailConn) Prepare(_ string) (driver.Stmt, error) { return nil, nil }
func (fakePingFailConn) Close() error                          { return nil }
func (fakePingFailConn) Begin() (driver.Tx, error)             { return nil, nil }
func (fakePingFailConn) Ping(_ context.Context) error          { return errors.New("ping refused") }

func init() {
	sql.Register("storage_fake", fakeDriver{})
	sql.Register("storage_fake_ping_fail", fakePingFailDriver{})
}

func TestNewNilConfigs(t *testing.T) {
	s, err := New(nil)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer s.Close()
	if _, err := s.KV("anything"); err == nil {
		t.Fatal("expected error for empty storage registry")
	}
}

func TestNewUnknownKind(t *testing.T) {
	if _, err := New(map[string]Config{
		"x": {Kind: "nosql", Backend: "magic"},
	}); err == nil {
		t.Fatal("expected error for unknown kind")
	}
}

func TestKVMemory(t *testing.T) {
	reg, err := New(map[string]Config{
		"mem": {Kind: KindKeyValue, Backend: "memory"},
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
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

func TestKVMemoryDriverBlock(t *testing.T) {
	reg, err := New(map[string]Config{
		"mem": {Kind: KindKeyValue, Memory: &MemoryConfig{}},
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer reg.Close()
	if _, err := reg.KV("mem"); err != nil {
		t.Fatalf("KV(mem): %v", err)
	}
}

func TestKVBadger(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "badger")
	reg, err := New(map[string]Config{
		"bg": {Kind: KindKeyValue, Backend: "badger", Dir: dir},
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
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

func TestKVBadgerDriverBlock(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "badger")
	reg, err := New(map[string]Config{
		"bg": {Kind: KindKeyValue, Badger: &BadgerConfig{Dir: dir}},
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer reg.Close()
	if _, err := reg.KV("bg"); err != nil {
		t.Fatalf("KV(bg): %v", err)
	}
}

func TestNewRejectsWrongDriverBlockForKind(t *testing.T) {
	if _, err := New(map[string]Config{
		"bad": {Kind: KindKeyValue, FS: &FSConfig{Dir: t.TempDir()}},
	}); err == nil {
		t.Fatal("expected error for wrong driver block")
	}
	if _, err := New(map[string]Config{
		"bad": {Kind: KindKeyValue, Memory: &MemoryConfig{}, Badger: &BadgerConfig{Dir: t.TempDir()}},
	}); err == nil {
		t.Fatal("expected error for multiple driver blocks")
	}
	if _, err := New(map[string]Config{
		"bad": {Kind: KindFilesystem, Badger: &BadgerConfig{Dir: t.TempDir()}},
	}); err == nil {
		t.Fatal("expected error for filesystem with badger driver")
	}
	if _, err := New(map[string]Config{
		"bad": {Kind: KindDepotStore, FS: &FSConfig{Dir: t.TempDir()}},
	}); err == nil {
		t.Fatal("expected error for depotstore with fs driver")
	}
	if _, err := New(map[string]Config{
		"bad": {Kind: KindSQL, FS: &FSConfig{Dir: t.TempDir()}},
	}); err == nil {
		t.Fatal("expected error for sql with fs driver")
	}
}

func TestNewRejectsMissingDriverBlock(t *testing.T) {
	tests := []struct {
		name string
		cfg  Config
	}{
		{name: "keyvalue", cfg: Config{Kind: KindKeyValue}},
		{name: "vecstore", cfg: Config{Kind: KindVecStore}},
		{name: "filesystem", cfg: Config{Kind: KindFilesystem}},
		{name: "depotstore", cfg: Config{Kind: KindDepotStore}},
		{name: "sql", cfg: Config{Kind: KindSQL}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := New(map[string]Config{"bad": tc.cfg}); err == nil {
				t.Fatal("expected error for missing driver")
			}
		})
	}
}

func TestKVNotFound(t *testing.T) {
	reg, err := New(map[string]Config{
		"fs": {Kind: KindFS, Backend: "filesystem", Dir: t.TempDir()},
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer reg.Close()

	if _, err := reg.KV("missing"); err == nil {
		t.Fatal("expected error for missing backend")
	}
	if _, err := reg.KV("fs"); err == nil {
		t.Fatal("expected error for wrong kind lookup")
	}
}

func TestNewKVUnknownBackend(t *testing.T) {
	if _, err := New(map[string]Config{
		"x": {Kind: KindKeyValue, Backend: "redis"},
	}); err == nil {
		t.Fatal("expected error for unknown kv backend")
	}
}

func TestNewKVBadgerNoDir(t *testing.T) {
	if _, err := New(map[string]Config{
		"x": {Kind: KindKeyValue, Backend: "badger"},
	}); err == nil {
		t.Fatal("expected error for badger without dir")
	}
}

func TestVecStoreMemory(t *testing.T) {
	reg, err := New(map[string]Config{
		"vec": {Kind: KindVecStore, Backend: "memory"},
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
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
}

func TestVecStoreNotFound(t *testing.T) {
	reg, err := New(map[string]Config{
		"kv": {Kind: KindKeyValue, Backend: "memory"},
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer reg.Close()

	if _, err := reg.VecStore("missing"); err == nil {
		t.Fatal("expected error for missing backend")
	}
	if _, err := reg.VecStore("kv"); err == nil {
		t.Fatal("expected error for wrong kind lookup")
	}
}

func TestNewVecStoreUnknownBackend(t *testing.T) {
	if _, err := New(map[string]Config{
		"x": {Kind: KindVecStore, Backend: "qdrant"},
	}); err == nil {
		t.Fatal("expected error for unknown vecstore backend")
	}
}

func TestFS(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "firmware")
	reg, err := New(map[string]Config{
		"fw": {Kind: KindFS, Backend: "filesystem", Dir: dir},
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer reg.Close()

	s, err := reg.FS("fw")
	if err != nil {
		t.Fatalf("FS(fw): %v", err)
	}
	if string(s) != dir {
		t.Fatalf("Root = %q, want %q", string(s), dir)
	}
}

func TestFilesystemDriverBlock(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "files")
	reg, err := New(map[string]Config{
		"files": {Kind: KindFilesystem, FS: &FSConfig{Dir: dir}},
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer reg.Close()

	s, err := reg.Filesystem("files")
	if err != nil {
		t.Fatalf("Filesystem(files): %v", err)
	}
	if string(s) != dir {
		t.Fatalf("Root = %q, want %q", string(s), dir)
	}
}

func TestDepotStoreDepotFS(t *testing.T) {
	reg, err := New(map[string]Config{
		"firmware-depot": {
			Kind:    KindDepotStore,
			DepotFS: &DepotFSConfig{},
		},
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer reg.Close()

	driver, err := reg.DepotDriver("firmware-depot")
	if err != nil {
		t.Fatalf("DepotDriver(firmware-depot): %v", err)
	}
	if driver != "depot-fs" {
		t.Fatalf("driver = %q", driver)
	}
}

func TestDepotStoreDepotFSRejectsBadRefs(t *testing.T) {
	if _, err := New(map[string]Config{
		"depot": {
			Kind: KindDepotStore,
		},
	}); err == nil {
		t.Fatal("expected error for invalid depotstore config")
	}
}

func TestFSNotFound(t *testing.T) {
	reg, err := New(map[string]Config{
		"kv": {Kind: KindKeyValue, Backend: "memory"},
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer reg.Close()

	if _, err := reg.FS("missing"); err == nil {
		t.Fatal("expected error for missing backend")
	}
	if _, err := reg.FS("kv"); err == nil {
		t.Fatal("expected error for wrong kind lookup")
	}
}

func TestNewFSNoDir(t *testing.T) {
	if _, err := New(map[string]Config{
		"x": {Kind: KindFS, Backend: "filesystem"},
	}); err == nil {
		t.Fatal("expected error for filesystem without dir")
	}
}

func TestNewFSUnknownBackend(t *testing.T) {
	if _, err := New(map[string]Config{
		"x": {Kind: KindFS, Backend: "s3"},
	}); err == nil {
		t.Fatal("expected error for unknown fs backend")
	}
}

func TestSQL(t *testing.T) {
	reg, err := New(map[string]Config{
		"db": {Kind: KindSQL, Backend: "storage_fake", DSN: "test"},
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer reg.Close()

	db, err := reg.SQL("db")
	if err != nil {
		t.Fatalf("SQL(db): %v", err)
	}
	if db == nil {
		t.Fatal("expected non-nil *sql.DB")
	}
}

func TestSQLSQLiteUsesDirAsDSN(t *testing.T) {
	reg, err := New(map[string]Config{
		"db": {Kind: KindSQL, Backend: "sqlite", Dir: filepath.Join(t.TempDir(), "db.sqlite")},
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer reg.Close()

	if _, err := reg.SQL("db"); err != nil {
		t.Fatalf("SQL(db): %v", err)
	}
}

func TestSQLNotFound(t *testing.T) {
	reg, err := New(map[string]Config{
		"kv": {Kind: KindKeyValue, Backend: "memory"},
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer reg.Close()

	if _, err := reg.SQL("missing"); err == nil {
		t.Fatal("expected error for missing backend")
	}
	if _, err := reg.SQL("kv"); err == nil {
		t.Fatal("expected error for wrong kind lookup")
	}
}

func TestNewSQLNoBackend(t *testing.T) {
	if _, err := New(map[string]Config{
		"x": {Kind: KindSQL, DSN: "x"},
	}); err == nil {
		t.Fatal("expected error for empty backend")
	}
}

func TestNewSQLNoDSN(t *testing.T) {
	if _, err := New(map[string]Config{
		"x": {Kind: KindSQL, Backend: "storage_fake"},
	}); err == nil {
		t.Fatal("expected error for missing dsn")
	}
}

func TestNewSQLBadDriver(t *testing.T) {
	if _, err := New(map[string]Config{
		"x": {Kind: KindSQL, Backend: "nonexistent_driver", DSN: "x"},
	}); err == nil {
		t.Fatal("expected error for unregistered driver")
	}
}

func TestNewSQLPingFail(t *testing.T) {
	if _, err := New(map[string]Config{
		"x": {Kind: KindSQL, Backend: "storage_fake_ping_fail", DSN: "x"},
	}); err == nil {
		t.Fatal("expected error for ping failure")
	}
}

func TestCloseEmpty(t *testing.T) {
	s, err := New(nil)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if err := s.Close(); err != nil {
		t.Fatalf("Close empty: %v", err)
	}
}
