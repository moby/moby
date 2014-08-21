// +build !darwin,!dragonfly,!freebsd,!linux,!netbsd,!openbsd

package utils

import (
	"os"
)

// TempDir returns the default directory to use for temporary files.
func TempDir(rootdir string) (string error) {
	return os.TempDir(), nil
}
