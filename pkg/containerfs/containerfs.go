package containerfs // import "github.com/docker/docker/pkg/containerfs"

import (
	"path/filepath"

	"github.com/moby/sys/symlink"
)

// ResolveScopedPath evaluates the given path scoped to the root.
// For example, if root=/a, and path=/b/c, then this function would return /a/b/c.
func ResolveScopedPath(root, path string) (string, error) {
	return symlink.FollowSymlinkInScope(filepath.Join(root, path), root)
}
