package objstore_test

import (
	"context"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"testing/fstest"

	"github.com/haivivi/giztoy/go/pkg/objstore"
)

func newTestStore(t *testing.T) (*objstore.FS, string) {
	t.Helper()
	dir := t.TempDir()
	s, err := objstore.NewFS(dir)
	if err != nil {
		t.Fatalf("NewFS: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s, dir
}

func TestPutAndOpen(t *testing.T) {
	ctx := context.Background()
	s, _ := newTestStore(t)

	if err := s.Put(ctx, "hello.txt", strings.NewReader("hello world")); err != nil {
		t.Fatalf("Put: %v", err)
	}

	f, err := s.Open("hello.txt")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer f.Close()

	data, err := io.ReadAll(f)
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if string(data) != "hello world" {
		t.Errorf("content = %q, want %q", data, "hello world")
	}

	info, err := f.Stat()
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	if info.Name() != "hello.txt" {
		t.Errorf("Name = %q, want %q", info.Name(), "hello.txt")
	}
	if info.Size() != 11 {
		t.Errorf("Size = %d, want 11", info.Size())
	}
	if info.IsDir() {
		t.Error("IsDir = true, want false")
	}
}

func TestPutOverwrite(t *testing.T) {
	ctx := context.Background()
	s, _ := newTestStore(t)

	if err := s.Put(ctx, "f.txt", strings.NewReader("v1")); err != nil {
		t.Fatal(err)
	}
	if err := s.Put(ctx, "f.txt", strings.NewReader("version2")); err != nil {
		t.Fatal(err)
	}

	f, err := s.Open("f.txt")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	data, _ := io.ReadAll(f)
	if string(data) != "version2" {
		t.Errorf("content = %q, want %q", data, "version2")
	}
	info, _ := f.Stat()
	if info.Size() != 8 {
		t.Errorf("Size = %d, want 8", info.Size())
	}
}

func TestPutNestedPath(t *testing.T) {
	ctx := context.Background()
	s, _ := newTestStore(t)

	if err := s.Put(ctx, "a/b/c.txt", strings.NewReader("nested")); err != nil {
		t.Fatalf("Put nested: %v", err)
	}

	f, err := s.Open("a/b/c.txt")
	if err != nil {
		t.Fatalf("Open nested: %v", err)
	}
	defer f.Close()
	data, _ := io.ReadAll(f)
	if string(data) != "nested" {
		t.Errorf("content = %q, want %q", data, "nested")
	}
}

func TestOpenDirectory(t *testing.T) {
	ctx := context.Background()
	s, _ := newTestStore(t)

	if err := s.Put(ctx, "dir/a.txt", strings.NewReader("aaa")); err != nil {
		t.Fatal(err)
	}
	if err := s.Put(ctx, "dir/b.txt", strings.NewReader("bbb")); err != nil {
		t.Fatal(err)
	}

	f, err := s.Open("dir")
	if err != nil {
		t.Fatalf("Open dir: %v", err)
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		t.Fatal(err)
	}
	if !info.IsDir() {
		t.Fatal("expected directory")
	}

	dirFile, ok := f.(fs.ReadDirFile)
	if !ok {
		t.Fatal("expected fs.ReadDirFile")
	}
	entries, err := dirFile.ReadDir(-1)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("ReadDir: got %d entries, want 2", len(entries))
	}
}

func TestOpenRoot(t *testing.T) {
	ctx := context.Background()
	s, _ := newTestStore(t)

	if err := s.Put(ctx, "x.txt", strings.NewReader("x")); err != nil {
		t.Fatal(err)
	}

	f, err := s.Open(".")
	if err != nil {
		t.Fatalf("Open root: %v", err)
	}
	defer f.Close()

	info, _ := f.Stat()
	if !info.IsDir() {
		t.Fatal("root should be a directory")
	}

	dirFile := f.(fs.ReadDirFile)
	entries, err := dirFile.ReadDir(-1)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) == 0 {
		t.Fatal("expected at least one entry in root")
	}
}

func TestOpenNotExist(t *testing.T) {
	s, _ := newTestStore(t)

	_, err := s.Open("no-such-file")
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("expected fs.ErrNotExist, got %v", err)
	}
}

func TestDeleteFile(t *testing.T) {
	ctx := context.Background()
	s, _ := newTestStore(t)

	if err := s.Put(ctx, "del.txt", strings.NewReader("bye")); err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteFile(ctx, "del.txt"); err != nil {
		t.Fatalf("DeleteFile: %v", err)
	}
	_, err := s.Open("del.txt")
	if !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("expected fs.ErrNotExist after delete, got %v", err)
	}
}

func TestDeleteFileNonExistent(t *testing.T) {
	ctx := context.Background()
	s, _ := newTestStore(t)

	if err := s.DeleteFile(ctx, "ghost"); err != nil {
		t.Fatalf("DeleteFile non-existent: %v", err)
	}
}

func TestDeleteFileRejectsDir(t *testing.T) {
	ctx := context.Background()
	s, _ := newTestStore(t)

	if err := s.Put(ctx, "d/a.txt", strings.NewReader("a")); err != nil {
		t.Fatal(err)
	}
	err := s.DeleteFile(ctx, "d")
	if err == nil {
		t.Fatal("expected error when DeleteFile targets a directory")
	}
	if !errors.Is(err, objstore.ErrIsDirectory) {
		t.Fatalf("expected ErrIsDirectory, got %v", err)
	}
}

func TestDeleteDir(t *testing.T) {
	ctx := context.Background()
	s, _ := newTestStore(t)

	if err := s.Put(ctx, "d/a.txt", strings.NewReader("a")); err != nil {
		t.Fatal(err)
	}
	if err := s.Put(ctx, "d/b.txt", strings.NewReader("b")); err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteDir(ctx, "d"); err != nil {
		t.Fatalf("DeleteDir: %v", err)
	}
	_, err := s.Open("d")
	if !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("expected fs.ErrNotExist after DeleteDir, got %v", err)
	}
}

func TestDeleteDirNonExistent(t *testing.T) {
	ctx := context.Background()
	s, _ := newTestStore(t)

	if err := s.DeleteDir(ctx, "ghost"); err != nil {
		t.Fatalf("DeleteDir non-existent: %v", err)
	}
}

func TestInvalidPaths(t *testing.T) {
	ctx := context.Background()
	s, _ := newTestStore(t)

	bad := []string{"", "/abs", "../escape", "a/../b", "a/", "./x"}
	for _, name := range bad {
		t.Run("put_"+name, func(t *testing.T) {
			if err := s.Put(ctx, name, strings.NewReader("x")); err == nil {
				t.Error("expected error")
			}
		})
		t.Run("deletefile_"+name, func(t *testing.T) {
			if err := s.DeleteFile(ctx, name); err == nil {
				t.Error("expected error")
			}
		})
		t.Run("deletedir_"+name, func(t *testing.T) {
			if err := s.DeleteDir(ctx, name); err == nil {
				t.Error("expected error")
			}
		})
		t.Run("open_"+name, func(t *testing.T) {
			if _, err := s.Open(name); err == nil {
				t.Error("expected error")
			}
		})
	}
}

func TestPutDotRejected(t *testing.T) {
	ctx := context.Background()
	s, _ := newTestStore(t)

	if err := s.Put(ctx, ".", strings.NewReader("x")); err == nil {
		t.Fatal("expected error for Put with name '.'")
	}
}

func TestDeleteFileDotRejected(t *testing.T) {
	ctx := context.Background()
	s, _ := newTestStore(t)

	if err := s.DeleteFile(ctx, "."); err == nil {
		t.Fatal("expected error for DeleteFile with name '.'")
	}
}

func TestDeleteDirDotRejected(t *testing.T) {
	ctx := context.Background()
	s, _ := newTestStore(t)

	if err := s.DeleteDir(ctx, "."); err == nil {
		t.Fatal("expected error for DeleteDir with name '.'")
	}
}

func TestPutAtomicity(t *testing.T) {
	ctx := context.Background()
	s, _ := newTestStore(t)

	if err := s.Put(ctx, "atom.txt", strings.NewReader("original")); err != nil {
		t.Fatal(err)
	}

	err := s.Put(ctx, "atom.txt", &errReader{err: errors.New("boom")})
	if err == nil {
		t.Fatal("expected error")
	}

	f, err := s.Open("atom.txt")
	if err != nil {
		t.Fatalf("Open after failed Put: %v", err)
	}
	defer f.Close()
	data, _ := io.ReadAll(f)
	if string(data) != "original" {
		t.Errorf("content = %q, want %q (atomicity broken)", data, "original")
	}
}

func TestNewFSEmptyRoot(t *testing.T) {
	_, err := objstore.NewFS("")
	if err == nil {
		t.Fatal("expected error for empty root")
	}
}

func TestNewFSCreatesDir(t *testing.T) {
	base := t.TempDir()
	nested := filepath.Join(base, "deep", "nested")
	s, err := objstore.NewFS(nested)
	if err != nil {
		t.Fatal(err)
	}
	s.Close()

	info, err := os.Stat(nested)
	if err != nil {
		t.Fatal(err)
	}
	if !info.IsDir() {
		t.Fatal("expected directory")
	}
}

func TestNewFSReadOnlyParent(t *testing.T) {
	base := t.TempDir()
	locked := filepath.Join(base, "locked")
	if err := os.Mkdir(locked, 0o444); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(locked, 0o755) })

	_, err := objstore.NewFS(filepath.Join(locked, "sub"))
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestClose(t *testing.T) {
	s, _ := newTestStore(t)
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
}

// TestFSContract uses the standard testing/fstest suite to validate
// the fs.FS implementation.
func TestFSContract(t *testing.T) {
	ctx := context.Background()
	s, _ := newTestStore(t)

	files := map[string]string{
		"a.txt":      "alpha",
		"b.txt":      "beta",
		"sub/c.txt":  "charlie",
		"sub/d.txt":  "delta",
		"sub/deep/e": "echo",
	}
	for name, content := range files {
		if err := s.Put(ctx, name, strings.NewReader(content)); err != nil {
			t.Fatalf("Put %s: %v", name, err)
		}
	}

	expected := make([]string, 0, len(files))
	for name := range files {
		expected = append(expected, name)
	}
	if err := fstest.TestFS(s, expected...); err != nil {
		t.Fatal(err)
	}
}

func TestPutWriteError(t *testing.T) {
	ctx := context.Background()
	s, _ := newTestStore(t)

	err := s.Put(ctx, "fail.txt", &errReader{err: errors.New("read boom")})
	if err == nil {
		t.Fatal("expected error from failing reader")
	}
}

func TestOpenPermission(t *testing.T) {
	ctx := context.Background()
	s, dir := newTestStore(t)

	if err := s.Put(ctx, "secret.txt", strings.NewReader("data")); err != nil {
		t.Fatal(err)
	}

	full := filepath.Join(dir, "secret.txt")
	if err := os.Chmod(full, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(full, 0o644) })

	_, err := s.Open("secret.txt")
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, fs.ErrPermission) {
		t.Fatalf("expected fs.ErrPermission, got %v", err)
	}
}

func TestOpenSymlinkLoop(t *testing.T) {
	s, dir := newTestStore(t)

	// Create a symlink loop: a -> b, b -> a. Opening either
	// produces ELOOP, which is neither ENOENT nor EACCES and
	// exercises the passthrough branch of unwrapOSErr.
	if err := os.Symlink("b", filepath.Join(dir, "a")); err != nil {
		t.Skip("symlinks not supported:", err)
	}
	os.Symlink("a", filepath.Join(dir, "b"))

	_, err := s.Open("a")
	if err == nil {
		t.Fatal("expected error for symlink loop")
	}
	if errors.Is(err, fs.ErrNotExist) {
		t.Fatal("error should not be ErrNotExist")
	}
	if errors.Is(err, fs.ErrPermission) {
		t.Fatal("error should not be ErrPermission")
	}
}

func TestPutReadOnlyDir(t *testing.T) {
	ctx := context.Background()
	s, dir := newTestStore(t)

	locked := filepath.Join(dir, "locked")
	if err := os.Mkdir(locked, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(locked, 0o755) })

	err := s.Put(ctx, "locked/sub/file.txt", strings.NewReader("x"))
	if err == nil {
		t.Fatal("expected error when intermediate dir is read-only")
	}
}

func TestPutConcurrentSameKey(t *testing.T) {
	ctx := context.Background()
	s, _ := newTestStore(t)

	const n = 20
	var wg sync.WaitGroup
	errs := make([]error, n)
	for i := range n {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			body := strings.Repeat(string(rune('a'+i%26)), 128)
			errs[i] = s.Put(ctx, "race.txt", strings.NewReader(body))
		}(i)
	}
	wg.Wait()

	var succeeded int
	for _, err := range errs {
		if err == nil {
			succeeded++
		}
	}
	if succeeded == 0 {
		t.Fatal("all concurrent Puts failed")
	}

	f, err := s.Open("race.txt")
	if err != nil {
		t.Fatalf("Open after concurrent Put: %v", err)
	}
	defer f.Close()
	data, _ := io.ReadAll(f)
	if len(data) != 128 {
		t.Errorf("content length = %d, want 128 (data corruption?)", len(data))
	}
	// Verify content is uniform (from a single writer, not mixed).
	for i := 1; i < len(data); i++ {
		if data[i] != data[0] {
			t.Fatalf("byte %d = %q, byte 0 = %q — content from multiple writers mixed", i, data[i], data[0])
		}
	}
}

func TestNewFSCleansTempFiles(t *testing.T) {
	dir := t.TempDir()
	s, err := objstore.NewFS(dir)
	if err != nil {
		t.Fatal(err)
	}
	s.Close()

	// Plant fake leftover temp files.
	leftover := filepath.Join(dir, "file.txt.objstore.1234567.tmp")
	if err := os.WriteFile(leftover, []byte("stale"), 0o644); err != nil {
		t.Fatal(err)
	}
	nested := filepath.Join(dir, "sub")
	if err := os.Mkdir(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	leftover2 := filepath.Join(nested, "other.objstore.999.tmp")
	if err := os.WriteFile(leftover2, []byte("stale"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Also plant a normal file that should NOT be removed.
	normal := filepath.Join(dir, "keep.txt")
	if err := os.WriteFile(normal, []byte("keep"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Re-open the store; cleanup should happen.
	s2, err := objstore.NewFS(dir)
	if err != nil {
		t.Fatal(err)
	}
	s2.Close()

	if _, err := os.Stat(leftover); !os.IsNotExist(err) {
		t.Error("expected leftover temp file to be cleaned up")
	}
	if _, err := os.Stat(leftover2); !os.IsNotExist(err) {
		t.Error("expected nested leftover temp file to be cleaned up")
	}
	if _, err := os.Stat(normal); err != nil {
		t.Error("normal file should not be removed")
	}
}

func TestPutRenameError(t *testing.T) {
	ctx := context.Background()
	s, dir := newTestStore(t)

	// Place a non-empty directory at the target path so that
	// os.Rename(tmpFile, dir) fails.
	if err := os.MkdirAll(filepath.Join(dir, "blocked.txt", "child"), 0o755); err != nil {
		t.Fatal(err)
	}

	err := s.Put(ctx, "blocked.txt", strings.NewReader("data"))
	if err == nil {
		t.Fatal("expected error when rename target is a non-empty directory")
	}
}

type errReader struct{ err error }

func (r *errReader) Read([]byte) (int, error) { return 0, r.err }
