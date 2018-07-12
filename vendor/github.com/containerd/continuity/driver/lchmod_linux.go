package driver

import (
	"os"

	"golang.org/x/sys/unix"
)

// Lchmod changes the mode of a file not following symlinks.
func (d *driver) Lchmod(path string, mode os.FileMode) error {
	// On Linux, file mode is not supported for symlinks,
	// and fchmodat() does not support AT_SYMLINK_NOFOLLOW,
	// so symlinks need to be skipped entirely.
	if st, err := os.Stat(path); err == nil && st.Mode()&os.ModeSymlink != 0 {
		return nil
	}

	return unix.Fchmodat(unix.AT_FDCWD, path, uint32(mode), 0)
}
