package containerfs // import "github.com/docker/docker/pkg/containerfs"

import (
	"path/filepath"

	"github.com/containerd/continuity/driver"
	"github.com/containerd/continuity/pathdriver"
	"github.com/moby/sys/symlink"
)

// ContainerFS is that represents a root file system
type ContainerFS interface {
	// Path returns the path to the root. Note that this may not exist
	// on the local system, so the continuity operations must be used
	Path() string

	// Driver & PathDriver provide methods to manipulate files & paths
	driver.Driver
	pathdriver.PathDriver
}

// NewLocalContainerFS is a helper function to implement daemon's Mount interface
// when the graphdriver mount point is a local path on the machine.
func NewLocalContainerFS(path string) ContainerFS {
	return &local{
		path:       path,
		Driver:     driver.LocalDriver,
		PathDriver: pathdriver.LocalPathDriver,
	}
}

type local struct {
	path string
	driver.Driver
	pathdriver.PathDriver
}

func (l *local) Path() string {
	return l.path
}

// ResolveScopedPath evaluates the given path scoped to the root.
// For example, if root=/a, and path=/b/c, then this function would return /a/b/c.
func ResolveScopedPath(root, path string) (string, error) {
	return symlink.FollowSymlinkInScope(filepath.Join(root, path), root)
}
