//go:build !windows

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

/*
   Portions from https://github.com/moby/moby/blob/v23.0.1/pkg/archive/archive_unix.go#L52-L70
   Copyright (C) Docker/Moby authors.
   Licensed under the Apache License, Version 2.0
   NOTICE: https://github.com/moby/moby/blob/v23.0.1/NOTICE
*/

package tarheader

import (
	"archive/tar"
	"os"
	"runtime"
	"syscall"

	"golang.org/x/sys/unix"
)

func init() {
	sysStat = statUnix
}

// statUnix populates hdr from system-dependent fields of fi without performing
// any OS lookups.
// From https://github.com/moby/moby/blob/2cccb1f02c83aaacef3c15fa43f3b64d57f315f8/pkg/archive/archive_unix.go#L46-L76
func statUnix(fi os.FileInfo, hdr *tar.Header) error {
	s, ok := fi.Sys().(*syscall.Stat_t)
	if !ok {
		return nil
	}

	hdr.Uid = int(s.Uid)
	hdr.Gid = int(s.Gid)

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

	if s.Mode&unix.S_IFBLK != 0 ||
		s.Mode&unix.S_IFCHR != 0 {
		hdr.Devmajor = int64(unix.Major(uint64(s.Rdev)))
		hdr.Devminor = int64(unix.Minor(uint64(s.Rdev)))
	}

	return nil
}
