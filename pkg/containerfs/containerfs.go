package containerfs // import "github.com/docker/docker/pkg/containerfs"

import (
	"path/filepath"

	"github.com/moby/sys/symlink"
)

// CleanScopedPath prepares the given path to be combined with a mount path or
// a drive-letter. On Windows, it removes any existing driveletter (e.g. "C:").
// The returned path is always prefixed with a [filepath.Separator].
func CleanScopedPath(path string) string {
	if len(path) >= 2 {
		if v := filepath.VolumeName(path); len(v) > 0 {
			path = path[len(v):]
		}
	}
	return filepath.Join(string(filepath.Separator), path)
}

// ResolveScopedPath evaluates the given path scoped to the root.
// For example, if root=/a, and path=/b/c, then this function would return /a/b/c.
//
// Deprecated: use [symlink.FollowSymlinkInScope].
func ResolveScopedPath(root, path string) (string, error) {
	return symlink.FollowSymlinkInScope(filepath.Join(root, path), root)
}
