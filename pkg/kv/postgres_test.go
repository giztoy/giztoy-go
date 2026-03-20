package kv_test

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os/exec"
	"slices"
	"strings"
	"testing"

	_ "github.com/lib/pq"

	"github.com/haivivi/giztoy/go/pkg/kv"
)

// postgresAvailable checks whether a local PostgreSQL is reachable.
// It looks for psql on PATH and tries to connect to the default socket.
func postgresAvailable() bool {
	if _, err := exec.LookPath("psql"); err != nil {
		return false
	}
	db, err := sql.Open("postgres", "host=/tmp dbname=giztoy_test sslmode=disable")
	if err != nil {
		return false
	}
	defer db.Close()
	return db.Ping() == nil
}

const postgresDSN = "host=/tmp dbname=giztoy_test sslmode=disable"

func truncatePostgres(t *testing.T) {
	t.Helper()
	db, err := sql.Open("postgres", postgresDSN)
	if err != nil {
		t.Fatalf("truncatePostgres open: %v", err)
	}
	defer db.Close()
	if _, err := db.Exec("TRUNCATE TABLE kv"); err != nil {
		t.Logf("truncatePostgres exec (table may not exist yet): %v", err)
	}
}

func newPostgresStore(t *testing.T, opts *kv.Options) kv.Store {
	t.Helper()
	if !postgresAvailable() {
		t.Skip("PostgreSQL not available locally, skipping")
	}
	truncatePostgres(t)
	s, err := kv.NewPostgres(postgresDSN, opts)
	if err != nil {
		t.Fatalf("NewPostgres: %v", err)
	}
	t.Cleanup(func() {
		s.Close()
		truncatePostgres(t)
	})
	return s
}

func TestPostgresGetSetDelete(t *testing.T) {
	ctx := context.Background()
	s := newPostgresStore(t, nil)

	key := kv.Key{"user", "profile", "123"}
	val := []byte("hello")

	_, err := s.Get(ctx, key)
	if !errors.Is(err, kv.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}

	if err := s.Set(ctx, key, val); err != nil {
		t.Fatalf("Set: %v", err)
	}
	got, err := s.Get(ctx, key)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if string(got) != string(val) {
		t.Fatalf("Get = %q, want %q", got, val)
	}

	val2 := []byte("world")
	if err := s.Set(ctx, key, val2); err != nil {
		t.Fatalf("Set overwrite: %v", err)
	}
	got, err = s.Get(ctx, key)
	if err != nil {
		t.Fatalf("Get after overwrite: %v", err)
	}
	if string(got) != string(val2) {
		t.Fatalf("Get = %q, want %q", got, val2)
	}

	if err := s.Delete(ctx, key); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	_, err = s.Get(ctx, key)
	if !errors.Is(err, kv.ErrNotFound) {
		t.Fatalf("expected ErrNotFound after delete, got %v", err)
	}

	if err := s.Delete(ctx, kv.Key{"no", "such", "key"}); err != nil {
		t.Fatalf("Delete non-existent: %v", err)
	}
}

func TestPostgresList(t *testing.T) {
	ctx := context.Background()
	s := newPostgresStore(t, nil)

	entries := []kv.Entry{
		{Key: kv.Key{"m1", "g", "e", "Alice"}, Value: []byte("a")},
		{Key: kv.Key{"m1", "g", "e", "Bob"}, Value: []byte("b")},
		{Key: kv.Key{"m1", "g", "r", "Alice", "knows", "Bob"}, Value: []byte("r1")},
		{Key: kv.Key{"m1", "seg", "20260101", "1"}, Value: []byte("s1")},
		{Key: kv.Key{"m2", "g", "e", "Charlie"}, Value: []byte("c")},
	}
	if err := s.BatchSet(ctx, entries); err != nil {
		t.Fatalf("BatchSet: %v", err)
	}

	var got []string
	for entry, err := range s.List(ctx, kv.Key{"m1", "g", "e"}) {
		if err != nil {
			t.Fatalf("List: %v", err)
		}
		got = append(got, entry.Key.String()+"="+string(entry.Value))
	}
	want := []string{
		"m1:g:e:Alice=a",
		"m1:g:e:Bob=b",
	}
	if !slices.Equal(got, want) {
		t.Fatalf("List m1:g:e = %v, want %v", got, want)
	}

	got = nil
	for entry, err := range s.List(ctx, kv.Key{"m1"}) {
		if err != nil {
			t.Fatalf("List: %v", err)
		}
		got = append(got, entry.Key.String())
	}
	if len(got) != 4 {
		t.Fatalf("List m1: got %d entries, want 4: %v", len(got), got)
	}

	got = nil
	for entry, err := range s.List(ctx, nil) {
		if err != nil {
			t.Fatalf("List: %v", err)
		}
		got = append(got, entry.Key.String())
	}
	if len(got) != 5 {
		t.Fatalf("List all: got %d entries, want 5: %v", len(got), got)
	}
}

func TestPostgresListPrefixBoundary(t *testing.T) {
	ctx := context.Background()
	s := newPostgresStore(t, nil)

	entries := []kv.Entry{
		{Key: kv.Key{"ab", "1"}, Value: []byte("yes")},
		{Key: kv.Key{"abc", "2"}, Value: []byte("no")},
		{Key: kv.Key{"ab", "3"}, Value: []byte("yes")},
	}
	if err := s.BatchSet(ctx, entries); err != nil {
		t.Fatalf("BatchSet: %v", err)
	}

	var got []string
	for entry, err := range s.List(ctx, kv.Key{"ab"}) {
		if err != nil {
			t.Fatalf("List: %v", err)
		}
		got = append(got, entry.Key.String())
	}
	want := []string{"ab:1", "ab:3"}
	if !slices.Equal(got, want) {
		t.Fatalf("List ab = %v, want %v", got, want)
	}
}

func TestPostgresBatchSetBatchDelete(t *testing.T) {
	ctx := context.Background()
	s := newPostgresStore(t, nil)

	entries := []kv.Entry{
		{Key: kv.Key{"a", "1"}, Value: []byte("v1")},
		{Key: kv.Key{"a", "2"}, Value: []byte("v2")},
		{Key: kv.Key{"a", "3"}, Value: []byte("v3")},
	}
	if err := s.BatchSet(ctx, entries); err != nil {
		t.Fatalf("BatchSet: %v", err)
	}

	for _, e := range entries {
		got, err := s.Get(ctx, e.Key)
		if err != nil {
			t.Fatalf("Get %v: %v", e.Key, err)
		}
		if string(got) != string(e.Value) {
			t.Fatalf("Get %v = %q, want %q", e.Key, got, e.Value)
		}
	}

	if err := s.BatchDelete(ctx, []kv.Key{{"a", "1"}, {"a", "2"}}); err != nil {
		t.Fatalf("BatchDelete: %v", err)
	}

	_, err := s.Get(ctx, kv.Key{"a", "1"})
	if !errors.Is(err, kv.ErrNotFound) {
		t.Fatalf("expected ErrNotFound for a:1, got %v", err)
	}
	_, err = s.Get(ctx, kv.Key{"a", "2"})
	if !errors.Is(err, kv.ErrNotFound) {
		t.Fatalf("expected ErrNotFound for a:2, got %v", err)
	}
	got, err := s.Get(ctx, kv.Key{"a", "3"})
	if err != nil {
		t.Fatalf("Get a:3: %v", err)
	}
	if string(got) != "v3" {
		t.Fatalf("Get a:3 = %q, want %q", got, "v3")
	}
}

func TestPostgresCustomSeparator(t *testing.T) {
	ctx := context.Background()
	s := newPostgresStore(t, &kv.Options{Separator: '/'})

	key := kv.Key{"path", "to", "value"}
	val := []byte("data")

	if err := s.Set(ctx, key, val); err != nil {
		t.Fatalf("Set: %v", err)
	}
	got, err := s.Get(ctx, key)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if string(got) != string(val) {
		t.Fatalf("Get = %q, want %q", got, val)
	}

	var keys []string
	for entry, err := range s.List(ctx, kv.Key{"path", "to"}) {
		if err != nil {
			t.Fatalf("List: %v", err)
		}
		keys = append(keys, entry.Key.String())
	}
	if len(keys) != 1 || keys[0] != "path:to:value" {
		t.Fatalf("List = %v, want [path:to:value]", keys)
	}
}

func TestPostgresPersistence(t *testing.T) {
	if !postgresAvailable() {
		t.Skip("PostgreSQL not available locally, skipping")
	}
	truncatePostgres(t)
	t.Cleanup(func() { truncatePostgres(t) })

	ctx := context.Background()

	s1, err := kv.NewPostgres(postgresDSN, nil)
	if err != nil {
		t.Fatalf("NewPostgres: %v", err)
	}
	if err := s1.Set(ctx, kv.Key{"persist", "key"}, []byte("value")); err != nil {
		t.Fatalf("Set: %v", err)
	}
	if err := s1.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	s2, err := kv.NewPostgres(postgresDSN, nil)
	if err != nil {
		t.Fatalf("NewPostgres reopen: %v", err)
	}
	defer s2.Close()

	got, err := s2.Get(ctx, kv.Key{"persist", "key"})
	if err != nil {
		t.Fatalf("Get after reopen: %v", err)
	}
	if string(got) != "value" {
		t.Fatalf("Get = %q, want %q", got, "value")
	}
}

func TestPostgresKeySegmentValidation(t *testing.T) {
	ctx := context.Background()
	s := newPostgresStore(t, nil)

	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic for key segment containing separator")
		}
		msg := fmt.Sprint(r)
		if !strings.Contains(msg, "contains separator") {
			t.Fatalf("unexpected panic: %v", r)
		}
	}()

	_ = s.Set(ctx, kv.Key{"bad:seg", "x"}, []byte("v"))
}

func TestNewPostgresError(t *testing.T) {
	_, err := kv.NewPostgres("postgres://localhost:1/nonexistent?sslmode=disable&connect_timeout=1", nil)
	if err == nil {
		t.Fatal("NewPostgres should fail with unreachable host")
	}
}
