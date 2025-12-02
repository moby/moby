//go:build (!((amd64 || arm64 || riscv64) && linux) && !((amd64 || arm64) && (darwin || freebsd)) && !((amd64 || arm64) && windows)) || js

package sysfs

import (
	"io/fs"
	"os"

	experimentalsys "github.com/tetratelabs/wazero/experimental/sys"
	"github.com/tetratelabs/wazero/sys"
)

// Note: go:build constraints must be the same as /sys.stat_unsupported.go for
// the same reasons.

// dirNlinkIncludesDot might be true for some operating systems, which can have
// new stat_XX.go files as necessary.
//
// Note: this is only used in tests
const dirNlinkIncludesDot = false

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
