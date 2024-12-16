package container

// RWLayer represents a writable layer for a container.
type RWLayer interface {
	// Mount mounts the RWLayer and returns the filesystem path
	// to the writable layer.
	Mount(mountLabel string) (string, error)

	// Unmount unmounts the RWLayer. This should be called
	// for every mount. If there are multiple mount calls
	// this operation will only decrement the internal mount counter.
	Unmount() error

	// Metadata returns the low level metadata for the mutable layer
	Metadata() (map[string]string, error)
}
