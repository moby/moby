package container

import (
	"io"
	"os"
	"time"

	"github.com/docker/docker/api/types/mount"
)

// PruneReport contains the response for Engine API:
// POST "/containers/prune"
type PruneReport struct {
	ContainersDeleted []string
	SpaceReclaimed    uint64
}

// PathStat is used to encode the header from
// GET "/containers/{name:.*}/archive"
// "Name" is the file or directory name.
type PathStat struct {
	Name       string      `json:"name"`
	Size       int64       `json:"size"`
	Mode       os.FileMode `json:"mode"`
	Mtime      time.Time   `json:"mtime"`
	LinkTarget string      `json:"linkTarget"`
}

// CopyToContainerOptions holds information
// about files to copy into a container
type CopyToContainerOptions struct {
	AllowOverwriteDirWithFile bool
	CopyUIDGID                bool
}

// StatsResponseReader wraps an io.ReadCloser to read (a stream of) stats
// for a container, as produced by the GET "/stats" endpoint.
//
// The OSType field is set to the server's platform to allow
// platform-specific handling of the response.
//
// TODO(thaJeztah): remove this wrapper, and make OSType part of [StatsResponse].
type StatsResponseReader struct {
	Body   io.ReadCloser `json:"body"`
	OSType string        `json:"ostype"`
}

// MountPoint represents a mount point configuration inside the container.
// This is used for reporting the mountpoints in use by a container.
type MountPoint struct {
	// Type is the type of mount, see `Type<foo>` definitions in
	// github.com/docker/docker/api/types/mount.Type
	Type mount.Type `json:",omitempty"`

	// Name is the name reference to the underlying data defined by `Source`
	// e.g., the volume name.
	Name string `json:",omitempty"`

	// Source is the source location of the mount.
	//
	// For volumes, this contains the storage location of the volume (within
	// `/var/lib/docker/volumes/`). For bind-mounts, and `npipe`, this contains
	// the source (host) part of the bind-mount. For `tmpfs` mount points, this
	// field is empty.
	Source string

	// Destination is the path relative to the container root (`/`) where the
	// Source is mounted inside the container.
	Destination string

	// Driver is the volume driver used to create the volume (if it is a volume).
	Driver string `json:",omitempty"`

	// Mode is a comma separated list of options supplied by the user when
	// creating the bind/volume mount.
	//
	// The default is platform-specific (`"z"` on Linux, empty on Windows).
	Mode string

	// RW indicates whether the mount is mounted writable (read-write).
	RW bool

	// Propagation describes how mounts are propagated from the host into the
	// mount point, and vice-versa. Refer to the Linux kernel documentation
	// for details:
	// https://www.kernel.org/doc/Documentation/filesystems/sharedsubtree.txt
	//
	// This field is not used on Windows.
	Propagation mount.Propagation
}
