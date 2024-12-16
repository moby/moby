package container

import (
	"io"

	"github.com/docker/docker/layer"
)

// RWLayer represents a writable layer for a container.
type RWLayer interface {
	layer.TarStreamer

	// Name of mounted layer
	Name() string

	// Mount mounts the RWLayer and returns the filesystem path
	// to the writable layer.
	Mount(mountLabel string) (string, error)

	// Unmount unmounts the RWLayer. This should be called
	// for every mount. If there are multiple mount calls
	// this operation will only decrement the internal mount counter.
	Unmount() error

	// Size represents the size of the writable layer
	// as calculated by the total size of the files
	// changed in the mutable layer.
	Size() (int64, error)

	// Metadata returns the low level metadata for the mutable layer
	Metadata() (map[string]string, error)

	// ApplyDiff applies the diff to the RW layer
	ApplyDiff(diff io.Reader) (int64, error)
}
