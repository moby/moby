//go:build !windows
// +build !windows

package containerfs // import "github.com/docker/docker/pkg/containerfs"

import "path/filepath"

// CleanScopedPath preappends a to combine with a mnt path.
func CleanScopedPath(path string) string {
	return filepath.Join(string(filepath.Separator), path)
}
