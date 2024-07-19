package containerfs

import (
	"path/filepath"

	"github.com/docker/docker/internal/containerfs"
)

// Deprecated: will be removed in the next release.
func CleanScopedPath(path string) string {
	if len(path) >= 2 {
		if v := filepath.VolumeName(path); len(v) > 0 {
			path = path[len(v):]
		}
	}
	return filepath.Join(string(filepath.Separator), path)
}

// Deprecated: will be removed in the next release.
func EnsureRemoveAll(dir string) error {
	return containerfs.EnsureRemoveAll(dir)
}
