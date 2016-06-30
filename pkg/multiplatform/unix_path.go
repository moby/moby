package multiplatform

import "strings"

// IsAbsUnix reports whether the path is absolute.
// This is taken from path/filepath
func IsAbsUnix(path string) bool {
	return strings.HasPrefix(path, "/")
}
