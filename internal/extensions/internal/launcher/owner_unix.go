//go:build unix

package launcher

import (
	"io/fs"
	"syscall"
)

// fileUID returns the owner uid of info, and whether it could be determined from
// the underlying stat.
func fileUID(info fs.FileInfo) (int, bool) {
	st, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return 0, false
	}
	return int(st.Uid), true
}
