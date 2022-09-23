package containerfs // import "github.com/docker/docker/pkg/containerfs"

import (
	"path/filepath"

	"github.com/moby/sys/symlink"
)

// ContainerFS is that represents a root file system
type ContainerFS interface {
	// Path returns the path to the root. Note that this may not exist
	// on the local system, so the continuity operations must be used
	Path() string
}

// NewLocalContainerFS is a helper function to implement daemon's Mount interface
// when the graphdriver mount point is a local path on the machine.
func NewLocalContainerFS(path string) ContainerFS {
	return &local{
		path: path,
	}
}

type local struct {
	path string
}

func (l *local) Path() string {
	return l.path
}

// ResolveScopedPath evaluates the given path scoped to the root.
// For example, if root=/a, and path=/b/c, then this function would return /a/b/c.
func ResolveScopedPath(root, path string) (string, error) {
	return symlink.FollowSymlinkInScope(filepath.Join(root, path), root)
}
