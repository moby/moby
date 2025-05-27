//go:build !windows

package tarheader

import (
	"archive/tar"
	"os"
	"runtime"
	"syscall"

	"golang.org/x/sys/unix"
)

// sysStat populates hdr from system-dependent fields of fi without performing
// any OS lookups.
func sysStat(fi os.FileInfo, hdr *tar.Header) error {
	// Devmajor and Devminor are only needed for special devices.

	// In FreeBSD, RDev for regular files is -1 (unless overridden by FS):
	// https://cgit.freebsd.org/src/tree/sys/kern/vfs_default.c?h=stable/13#n1531
	// (NODEV is -1: https://cgit.freebsd.org/src/tree/sys/sys/param.h?h=stable/13#n241).

	// ZFS in particular does not override the default:
	// https://cgit.freebsd.org/src/tree/sys/contrib/openzfs/module/os/freebsd/zfs/zfs_vnops_os.c?h=stable/13#n2027

	// Since `Stat_t.Rdev` is uint64, the cast turns -1 into (2^64 - 1).
	// Such large values cannot be encoded in a tar header.
	if runtime.GOOS == "freebsd" && hdr.Typeflag != tar.TypeBlock && hdr.Typeflag != tar.TypeChar {
		return nil
	}
	s, ok := fi.Sys().(*syscall.Stat_t)
	if !ok {
		return nil
	}

	hdr.Uid = int(s.Uid)
	hdr.Gid = int(s.Gid)

	if s.Mode&unix.S_IFBLK != 0 ||
		s.Mode&unix.S_IFCHR != 0 {
		hdr.Devmajor = int64(unix.Major(uint64(s.Rdev))) //nolint: unconvert
		hdr.Devminor = int64(unix.Minor(uint64(s.Rdev))) //nolint: unconvert
	}

	return nil
}
