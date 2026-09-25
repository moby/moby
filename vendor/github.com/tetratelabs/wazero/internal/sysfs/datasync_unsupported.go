//go:build !linux

package sysfs

import (
	"os"

	"github.com/tetratelabs/wazero/experimental/sys"
)

func datasync(f *os.File) sys.Errno {
	// Attempt to sync everything, even if we only need to sync the data.
	return fsync(f)
}
