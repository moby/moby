package containerfs // import "github.com/docker/docker/pkg/containerfs"

import (
	"path/filepath"

	"github.com/moby/sys/symlink"
)

// ContainerFS is that represents a root file system
type ContainerFS string

// NewLocalContainerFS is a helper function to implement daemon's Mount interface
// when the graphdriver mount point is a local path on the machine.
func NewLocalContainerFS(path string) ContainerFS {
	return ContainerFS(path)
}

// ResolveScopedPath evaluates the given path scoped to the root.
// For example, if root=/a, and path=/b/c, then this function would return /a/b/c.
func ResolveScopedPath(root, path string) (string, error) {
	return symlink.FollowSymlinkInScope(filepath.Join(root, path), root)
}
