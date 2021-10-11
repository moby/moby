// Copyright 2012 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build aix || linux || darwin || dragonfly || freebsd || openbsd || netbsd || solaris
// +build aix linux darwin dragonfly freebsd openbsd netbsd solaris

package tar

import (
	"io/fs"
	"runtime"
	"syscall"
)

func init() {
	sysStat = statUnix
}

func statUnix(fi fs.FileInfo, h *Header) error {
	sys, ok := fi.Sys().(*syscall.Stat_t)
	if !ok {
		return nil
	}
	h.Uid = int(sys.Uid)
	h.Gid = int(sys.Gid)

	// TODO(bradfitz): populate username & group.  os/user
	// doesn't cache LookupId lookups, and lacks group
	// lookup functions.
	h.AccessTime = statAtime(sys)
	h.ChangeTime = statCtime(sys)

	// Best effort at populating Devmajor and Devminor.
	if h.Typeflag == TypeChar || h.Typeflag == TypeBlock {
		dev := uint64(sys.Rdev) // May be int32 or uint32
		switch runtime.GOOS {
		case "aix":
			var major, minor uint32
			major = uint32((dev & 0x3fffffff00000000) >> 32)
			minor = uint32((dev & 0x00000000ffffffff) >> 0)
			h.Devmajor, h.Devminor = int64(major), int64(minor)
		case "linux":
			// Copied from golang.org/x/sys/unix/dev_linux.go.
			major := uint32((dev & 0x00000000000fff00) >> 8)
			major |= uint32((dev & 0xfffff00000000000) >> 32)
			minor := uint32((dev & 0x00000000000000ff) >> 0)
			minor |= uint32((dev & 0x00000ffffff00000) >> 12)
			h.Devmajor, h.Devminor = int64(major), int64(minor)
		case "darwin", "ios":
			// Copied from golang.org/x/sys/unix/dev_darwin.go.
			major := uint32((dev >> 24) & 0xff)
			minor := uint32(dev & 0xffffff)
			h.Devmajor, h.Devminor = int64(major), int64(minor)
		case "dragonfly":
			// Copied from golang.org/x/sys/unix/dev_dragonfly.go.
			major := uint32((dev >> 8) & 0xff)
			minor := uint32(dev & 0xffff00ff)
			h.Devmajor, h.Devminor = int64(major), int64(minor)
		case "freebsd":
			// Copied from golang.org/x/sys/unix/dev_freebsd.go.
			major := uint32((dev >> 8) & 0xff)
			minor := uint32(dev & 0xffff00ff)
			h.Devmajor, h.Devminor = int64(major), int64(minor)
		case "netbsd":
			// Copied from golang.org/x/sys/unix/dev_netbsd.go.
			major := uint32((dev & 0x000fff00) >> 8)
			minor := uint32((dev & 0x000000ff) >> 0)
			minor |= uint32((dev & 0xfff00000) >> 12)
			h.Devmajor, h.Devminor = int64(major), int64(minor)
		case "openbsd":
			// Copied from golang.org/x/sys/unix/dev_openbsd.go.
			major := uint32((dev & 0x0000ff00) >> 8)
			minor := uint32((dev & 0x000000ff) >> 0)
			minor |= uint32((dev & 0xffff0000) >> 8)
			h.Devmajor, h.Devminor = int64(major), int64(minor)
		default:
			// TODO: Implement solaris (see https://golang.org/issue/8106)
		}
	}
	return nil
}
