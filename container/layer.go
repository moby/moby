package container

import "context"

// Layer represents a layer useable by container.
type Layer interface {
	Writable() bool

	// Mount mounts the Layer and returns the filesystem path
	// to the writable layer.
	Mount(ctx context.Context, mountLabel string) (string, error)

	// Unmount unmounts the Layer. This should be called
	// for every mount. If there are multiple mount calls
	// this operation will only decrement the internal mount counter.
	Unmount(ctx context.Context) error

	// Metadata returns the low level metadata for the mutable layer
	Metadata() (map[string]string, error)
}
