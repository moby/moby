//go:build !windows && !tinygo

package sysfs

import (
	"io/fs"
	"os"

	"github.com/tetratelabs/wazero/experimental/sys"
)

// openFile is like os.OpenFile except it accepts a sys.Oflag and returns
// sys.Errno. A zero sys.Errno is success.
func openFile(path string, oflag sys.Oflag, perm fs.FileMode) (*os.File, sys.Errno) {
	f, err := os.OpenFile(path, toOsOpenFlag(oflag), perm)
	// Note: This does not return a sys.File because sys.FS that returns
	// one may want to hide the real OS path. For example, this is needed for
	// pre-opens.
	return f, sys.UnwrapOSError(err)
}
