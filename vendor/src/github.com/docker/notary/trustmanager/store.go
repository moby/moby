package trustmanager

import (
	"errors"

	"github.com/docker/notary"
)

const (
	visible = notary.PubCertPerms
	private = notary.PrivKeyPerms
)

var (
	// ErrPathOutsideStore indicates that the returned path would be
	// outside the store
	ErrPathOutsideStore = errors.New("path outside file store")
)

// LimitedFileStore implements the bare bones primitives (no hierarchy)
type LimitedFileStore interface {
	// Add writes a file to the specified location, returning an error if this
	// is not possible (reasons may include permissions errors). The path is cleaned
	// before being made absolute against the store's base dir.
	Add(fileName string, data []byte) error

	// Remove deletes a file from the store relative to the store's base directory.
	// The path is cleaned before being made absolute to ensure no path traversal
	// outside the base directory is possible.
	Remove(fileName string) error

	// Get returns the file content found at fileName relative to the base directory
	// of the file store. The path is cleaned before being made absolute to ensure
	// path traversal outside the store is not possible. If the file is not found
	// an error to that effect is returned.
	Get(fileName string) ([]byte, error)

	// ListFiles returns a list of paths relative to the base directory of the
	// filestore. Any of these paths must be retrievable via the
	// LimitedFileStore.Get method.
	ListFiles() []string
}

// FileStore is the interface for full-featured FileStores
type FileStore interface {
	LimitedFileStore

	RemoveDir(directoryName string) error
	GetPath(fileName string) (string, error)
	ListDir(directoryName string) []string
	BaseDir() string
}
