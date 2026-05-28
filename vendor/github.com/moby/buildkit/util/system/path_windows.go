package system

import (
	"path/filepath"
	"strings"
)

// DefaultSystemVolumeName is the default system volume label on Windows
const DefaultSystemVolumeName = "C:"

// IsAbsolutePath prepends the default system volume label
// to the path that is presumed absolute, and then calls filepath.IsAbs
func IsAbsolutePath(path string) bool {
	path = filepath.Clean(path)
	if strings.HasPrefix(path, "\\") {
		path = DefaultSystemVolumeName + path
	}
	return filepath.IsAbs(path)
}

// GetAbsolutePath returns an absolute path rooted
// to C:\\ on Windows.
func GetAbsolutePath(path string) string {
	path = filepath.Clean(path)
	if len(path) >= 2 && strings.EqualFold(path[:2], DefaultSystemVolumeName) {
		return path
	}
	return DefaultSystemVolumeName + path
}
