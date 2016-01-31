package daemon

import "errors"

var (
	// ErrRootFSReadOnly is returned when a container
	// rootfs is marked readonly.
	ErrRootFSReadOnly = errors.New("container rootfs is marked read-only")
)
