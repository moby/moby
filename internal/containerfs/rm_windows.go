package containerfs

import "os"

// EnsureRemoveAll is an alias to [os.RemoveAll] on Windows.
func EnsureRemoveAll(path string) error {
	return os.RemoveAll(path)
}
