package containerfs // import "github.com/docker/docker/pkg/containerfs"

import "path/filepath"

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
