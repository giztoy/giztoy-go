// Package filesystem defines a file-access interface for persistent storage.
//
// Concrete implementations (local disk, S3, etc.) live in separate packages
// and are wired at the application level.
package filesystem

import (
	"io"
)

// FS provides named-file access for persistent storage.
//
// Implementations must be safe for concurrent use.
type FS interface {
	// Open opens a named file for reading.
	// Returns an error wrapping os.ErrNotExist if the file does not exist.
	Open(name string) (io.ReadCloser, error)

	// Create creates or truncates a named file for writing.
	// Implementations should ensure atomicity where possible (e.g. write
	// to a temporary file and rename on Close).
	Create(name string) (io.WriteCloser, error)

	// Remove removes a named file. Returns nil if the file does not exist.
	Remove(name string) error
}
