//go:build (amd64 || arm64) && (darwin || freebsd)

package sysfs

import (
	"io/fs"
	"os"

	experimentalsys "github.com/tetratelabs/wazero/experimental/sys"
	"github.com/tetratelabs/wazero/sys"
)

// dirNlinkIncludesDot is true because even though os.File filters out dot
// entries, the underlying syscall.Stat includes them.
//
// Note: this is only used in tests
const dirNlinkIncludesDot = true

func lstat(path string) (sys.Stat_t, experimentalsys.Errno) {
	if info, err := os.Lstat(path); err != nil {
		return sys.Stat_t{}, experimentalsys.UnwrapOSError(err)
	} else {
		return sys.NewStat_t(info), 0
	}
}

func stat(path string) (sys.Stat_t, experimentalsys.Errno) {
	if info, err := os.Stat(path); err != nil {
		return sys.Stat_t{}, experimentalsys.UnwrapOSError(err)
	} else {
		return sys.NewStat_t(info), 0
	}
}

func statFile(f fs.File) (sys.Stat_t, experimentalsys.Errno) {
	return defaultStatFile(f)
}
