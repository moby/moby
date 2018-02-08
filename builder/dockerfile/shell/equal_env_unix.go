// +build !windows

package shell // import "github.com/docker/docker/builder/dockerfile/shell"

// EqualEnvKeys compare two strings and returns true if they are equal. On
// Windows this comparison is case insensitive.
func EqualEnvKeys(from, to string) bool {
	return from == to
}
