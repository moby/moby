// +build !windows

package contenthash

import (
	"os"
	"syscall"

	"github.com/containerd/continuity/sysx"
	fstypes "github.com/tonistiigi/fsutil/types"

	"golang.org/x/sys/unix"
)

func chmodWindowsTarEntry(perm os.FileMode) os.FileMode {
	return perm
}

func setUnixOpt(path string, fi os.FileInfo, stat *fstypes.Stat) error {
	s := fi.Sys().(*syscall.Stat_t)

	stat.Uid = s.Uid
	stat.Gid = s.Gid

	if !fi.IsDir() {
		if s.Mode&syscall.S_IFBLK != 0 ||
			s.Mode&syscall.S_IFCHR != 0 {
			stat.Devmajor = int64(unix.Major(uint64(s.Rdev)))
			stat.Devminor = int64(unix.Minor(uint64(s.Rdev)))
		}
	}

	attrs, err := sysx.LListxattr(path)
	if err != nil {
		return err
	}
	if len(attrs) > 0 {
		stat.Xattrs = map[string][]byte{}
		for _, attr := range attrs {
			v, err := sysx.LGetxattr(path, attr)
			if err == nil {
				stat.Xattrs[attr] = v
			}
		}
	}
	return nil
}
