package sysfs

import (
	"os"

	"github.com/tetratelabs/wazero/experimental/sys"
)

func fsync(f *os.File) sys.Errno {
	errno := sys.UnwrapOSError(f.Sync())
	// Coerce error performing stat on a directory to 0, as it won't work
	// on Windows.
	switch errno {
	case sys.EACCES /* Go 1.20 */, sys.EBADF /* Go 1.19 */ :
		if st, err := f.Stat(); err == nil && st.IsDir() {
			errno = 0
		}
	}
	return errno
}
