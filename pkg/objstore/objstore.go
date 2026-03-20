// Package objstore provides a writable object storage interface that extends
// [io/fs.FS] with Put, DeleteFile, and DeleteDir operations.
//
// The read side (Open) follows the standard [fs.FS] contract, so stored
// objects can be served directly by [net/http.FileServer], parsed by
// [html/template.ParseFS], and consumed by any code that accepts [fs.FS].
//
// Open returns an [fs.File] whose Stat gives file metadata; for directories
// the file also implements [fs.ReadDirFile] to list entries.
//
// The package ships with a local-filesystem backend ([NewFS]).
package objstore

import (
	"context"
	"errors"
	"io"
	"io/fs"
)

// ErrIsDirectory is returned by [Store.DeleteFile] when the target path
// is a directory rather than a regular file.
var ErrIsDirectory = errors.New("is a directory")

// Store is a writable object storage that extends [fs.FS].
//
// All implementations must be safe for concurrent use.
type Store interface {
	fs.FS

	// Put writes a file at the given path, creating intermediate directories
	// as needed. If the file already exists it is overwritten.
	// The path must be valid per [fs.ValidPath] and must not be ".".
	Put(ctx context.Context, name string, r io.Reader) error

	// DeleteFile removes a single file at the given path.
	// It returns an error if the path is a directory.
	// No error is returned if the path does not exist.
	// The path must be valid per [fs.ValidPath] and must not be ".".
	DeleteFile(ctx context.Context, name string) error

	// DeleteDir recursively removes the directory at the given path and
	// all of its children. No error is returned if the path does not exist.
	// The path must be valid per [fs.ValidPath] and must not be ".".
	DeleteDir(ctx context.Context, name string) error

	// Close releases resources held by the store.
	Close() error
}
