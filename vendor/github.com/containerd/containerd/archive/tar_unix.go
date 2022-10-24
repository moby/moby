//go:build !windows
// +build !windows

/*
   Copyright The containerd Authors.

   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
*/

package archive

import (
	"archive/tar"
	"errors"
	"fmt"
	"os"
	"runtime"
	"strings"
	"syscall"

	"github.com/containerd/containerd/pkg/userns"
	"github.com/containerd/continuity/fs"
	"github.com/containerd/continuity/sysx"
	"golang.org/x/sys/unix"
)

func tarName(p string) (string, error) {
	return p, nil
}

func chmodTarEntry(perm os.FileMode) os.FileMode {
	return perm
}

func setHeaderForSpecialDevice(hdr *tar.Header, name string, fi os.FileInfo) error {
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
		return errors.New("unsupported stat type")
	}

	// Rdev is int32 on darwin/bsd, int64 on linux/solaris
	rdev := uint64(s.Rdev) //nolint:unconvert

	// Currently go does not fill in the major/minors
	if s.Mode&syscall.S_IFBLK != 0 ||
		s.Mode&syscall.S_IFCHR != 0 {
		hdr.Devmajor = int64(unix.Major(rdev))
		hdr.Devminor = int64(unix.Minor(rdev))
	}

	return nil
}

func open(p string) (*os.File, error) {
	return os.Open(p)
}

func openFile(name string, flag int, perm os.FileMode) (*os.File, error) {
	f, err := os.OpenFile(name, flag, perm)
	if err != nil {
		return nil, err
	}
	// Call chmod to avoid permission mask
	if err := os.Chmod(name, perm); err != nil {
		f.Close()
		return nil, err
	}
	return f, err
}

func mkdir(path string, perm os.FileMode) error {
	if err := os.Mkdir(path, perm); err != nil {
		return err
	}
	// Only final created directory gets explicit permission
	// call to avoid permission mask
	return os.Chmod(path, perm)
}

func skipFile(hdr *tar.Header) bool {
	switch hdr.Typeflag {
	case tar.TypeBlock, tar.TypeChar:
		// cannot create a device if running in user namespace
		return userns.RunningInUserNS()
	default:
		return false
	}
}

// handleTarTypeBlockCharFifo is an OS-specific helper function used by
// createTarFile to handle the following types of header: Block; Char; Fifo.
// This function must not be called for Block and Char when running in userns.
// (skipFile() should return true for them.)
func handleTarTypeBlockCharFifo(hdr *tar.Header, path string) error {
	mode := uint32(hdr.Mode & 07777)
	switch hdr.Typeflag {
	case tar.TypeBlock:
		mode |= unix.S_IFBLK
	case tar.TypeChar:
		mode |= unix.S_IFCHR
	case tar.TypeFifo:
		mode |= unix.S_IFIFO
	}

	return mknod(path, mode, unix.Mkdev(uint32(hdr.Devmajor), uint32(hdr.Devminor)))
}

func getxattr(path, attr string) ([]byte, error) {
	b, err := sysx.LGetxattr(path, attr)
	if err == unix.ENOTSUP || err == sysx.ENODATA {
		return nil, nil
	}
	return b, err
}

func setxattr(path, key, value string) error {
	// Do not set trusted attributes
	if strings.HasPrefix(key, "trusted.") {
		return fmt.Errorf("admin attributes from archive not supported: %w", unix.ENOTSUP)
	}
	return unix.Lsetxattr(path, key, []byte(value), 0)
}

func copyDirInfo(fi os.FileInfo, path string) error {
	st := fi.Sys().(*syscall.Stat_t)
	if err := os.Lchown(path, int(st.Uid), int(st.Gid)); err != nil {
		if os.IsPermission(err) {
			// Normally if uid/gid are the same this would be a no-op, but some
			// filesystems may still return EPERM... for instance NFS does this.
			// In such a case, this is not an error.
			if dstStat, err2 := os.Lstat(path); err2 == nil {
				st2 := dstStat.Sys().(*syscall.Stat_t)
				if st.Uid == st2.Uid && st.Gid == st2.Gid {
					err = nil
				}
			}
		}
		if err != nil {
			return fmt.Errorf("failed to chown %s: %w", path, err)
		}
	}

	if err := os.Chmod(path, fi.Mode()); err != nil {
		return fmt.Errorf("failed to chmod %s: %w", path, err)
	}

	timespec := []unix.Timespec{
		unix.NsecToTimespec(syscall.TimespecToNsec(fs.StatAtime(st))),
		unix.NsecToTimespec(syscall.TimespecToNsec(fs.StatMtime(st))),
	}
	if err := unix.UtimesNanoAt(unix.AT_FDCWD, path, timespec, unix.AT_SYMLINK_NOFOLLOW); err != nil {
		return fmt.Errorf("failed to utime %s: %w", path, err)
	}

	return nil
}

func copyUpXAttrs(dst, src string) error {
	xattrKeys, err := sysx.LListxattr(src)
	if err != nil {
		if err == unix.ENOTSUP || err == sysx.ENODATA {
			return nil
		}
		return fmt.Errorf("failed to list xattrs on %s: %w", src, err)
	}
	for _, xattr := range xattrKeys {
		// Do not copy up trusted attributes
		if strings.HasPrefix(xattr, "trusted.") {
			continue
		}
		data, err := sysx.LGetxattr(src, xattr)
		if err != nil {
			if err == unix.ENOTSUP || err == sysx.ENODATA {
				continue
			}
			return fmt.Errorf("failed to get xattr %q on %s: %w", xattr, src, err)
		}
		if err := lsetxattrCreate(dst, xattr, data); err != nil {
			return fmt.Errorf("failed to set xattr %q on %s: %w", xattr, dst, err)
		}
	}

	return nil
}
