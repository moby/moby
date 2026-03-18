//go:build !windows
// +build !windows

package fs

import (
	"os"
	"syscall"

	"github.com/pkg/errors"

	"github.com/containerd/continuity/sysx"
)

// copyXAttrs requires xeh to be non-nil
func copyXAttrs(dst, src string, xeh XAttrErrorHandler) error {
	xattrKeys, err := sysx.LListxattr(src)
	if err != nil {
		return xeh(dst, src, "", errors.Wrapf(err, "failed to list xattrs on %s", src))
	}
	for _, xattr := range xattrKeys {
		data, err := sysx.LGetxattr(src, xattr)
		if err != nil {
			return xeh(dst, src, xattr, errors.Wrapf(err, "failed to get xattr %q on %s", xattr, src))
		}
		if err := sysx.LSetxattr(dst, xattr, data, 0); err != nil {
			return xeh(dst, src, xattr, errors.Wrapf(err, "failed to set xattr %q on %s", xattr, dst))
		}
	}

	return nil
}

func copyDevice(dst string, fi os.FileInfo) error {
	st, ok := fi.Sys().(*syscall.Stat_t)
	if !ok {
		return errors.New("unsupported stat type")
	}
	var rDev int
	if fi.Mode()&os.ModeDevice == os.ModeDevice || fi.Mode()&os.ModeCharDevice == os.ModeCharDevice {
		rDev = int(st.Rdev)
	}
	mode := st.Mode
	mode &^= syscall.S_IFSOCK // socket copied as stub
	return mknod(dst, uint32(mode), rDev)
}
