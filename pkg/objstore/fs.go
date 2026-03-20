package objstore

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// FS is a local-filesystem-backed object store.
//
// Files are stored directly under the root directory, preserving the path
// hierarchy. [FS.Open] returns standard [os.File] values that implement
// [fs.File] and [fs.ReadDirFile].
//
// FS is safe for concurrent use; file-level atomicity is provided by
// writing to a temporary file and renaming.
type FS struct {
	root string
}

// NewFS creates a filesystem-backed [Store] rooted at the given directory.
// The directory must be a non-empty path; it is created if it does not
// exist. Any leftover temporary files from incomplete Put operations are
// removed.
func NewFS(root string) (*FS, error) {
	if root == "" {
		return nil, fmt.Errorf("objstore: root must not be empty")
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("objstore: resolve root: %w", err)
	}
	if err := os.MkdirAll(abs, 0o755); err != nil {
		return nil, fmt.Errorf("objstore: create root: %w", err)
	}
	cleanTempFiles(abs)
	return &FS{root: abs}, nil
}

var _ Store = (*FS)(nil)

// Open implements [fs.FS]. The name must be valid per [fs.ValidPath];
// "." returns the root directory.
func (s *FS) Open(name string) (fs.File, error) {
	if !fs.ValidPath(name) {
		return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrInvalid}
	}
	f, err := os.Open(s.fullPath(name))
	if err != nil {
		return nil, &fs.PathError{Op: "open", Path: name, Err: unwrapOSErr(err)}
	}
	return f, nil
}

// Put writes a file at the given path, creating intermediate directories
// as needed. Writes are atomic: content is first written to a temporary
// file, then renamed into place.
func (s *FS) Put(_ context.Context, name string, r io.Reader) error {
	if !fs.ValidPath(name) || name == "." {
		return &fs.PathError{Op: "put", Path: name, Err: fs.ErrInvalid}
	}

	full := s.fullPath(name)
	dir := filepath.Dir(full)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("objstore: mkdir: %w", err)
	}

	f, err := os.CreateTemp(dir, filepath.Base(full)+".objstore.*.tmp")
	if err != nil {
		return fmt.Errorf("objstore: create temp: %w", err)
	}
	tmpName := f.Name()
	_, copyErr := io.Copy(f, r)
	if closeErr := f.Close(); closeErr != nil && copyErr == nil {
		copyErr = closeErr
	}
	if copyErr != nil {
		os.Remove(tmpName)
		return fmt.Errorf("objstore: write: %w", copyErr)
	}
	if err := os.Chmod(tmpName, 0o644); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("objstore: chmod: %w", err)
	}
	if err := os.Rename(tmpName, full); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("objstore: commit: %w", err)
	}
	return nil
}

// DeleteFile removes a single file. It returns an [ErrIsDirectory] error
// (wrapped in [fs.PathError]) if name is a directory.
// No error is returned if the path does not exist.
func (s *FS) DeleteFile(_ context.Context, name string) error {
	if !fs.ValidPath(name) || name == "." {
		return &fs.PathError{Op: "deletefile", Path: name, Err: fs.ErrInvalid}
	}
	full := s.fullPath(name)
	err := os.Remove(full)
	if err == nil || os.IsNotExist(err) {
		return nil
	}
	if info, statErr := os.Lstat(full); statErr == nil && info.IsDir() {
		return &fs.PathError{Op: "deletefile", Path: name, Err: ErrIsDirectory}
	}
	return fmt.Errorf("objstore: deletefile: %w", err)
}

// DeleteDir recursively removes a directory and all of its children.
// No error is returned if the path does not exist.
func (s *FS) DeleteDir(_ context.Context, name string) error {
	if !fs.ValidPath(name) || name == "." {
		return &fs.PathError{Op: "deletedir", Path: name, Err: fs.ErrInvalid}
	}
	err := os.RemoveAll(s.fullPath(name))
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("objstore: deletedir: %w", err)
	}
	return nil
}

func (s *FS) Close() error { return nil }

func (s *FS) fullPath(name string) string {
	return filepath.Join(s.root, filepath.FromSlash(name))
}

// cleanTempFiles walks root and removes any leftover .objstore.*.tmp files
// from incomplete Put operations. Errors are silently ignored.
func cleanTempFiles(root string) {
	filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if !d.IsDir() && isTempFile(d.Name()) {
			os.Remove(path)
		}
		return nil
	})
}

func isTempFile(name string) bool {
	return strings.HasSuffix(name, ".tmp") && strings.Contains(name, ".objstore.")
}

// unwrapOSErr converts OS-level errors to fs-package sentinels.
func unwrapOSErr(err error) error {
	if os.IsNotExist(err) {
		return fs.ErrNotExist
	}
	if os.IsPermission(err) {
		return fs.ErrPermission
	}
	return err
}
