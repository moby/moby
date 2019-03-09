// Copyright 2016 the Go-FUSE Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package fuse

import (
	"fmt"
	"syscall"
)

func init() {
	OpenFlagNames[syscall.O_DIRECT] = "DIRECT"
	OpenFlagNames[syscall.O_LARGEFILE] = "LARGEFILE"
	OpenFlagNames[syscall_O_NOATIME] = "NOATIME"

}

func (a *Attr) string() string {
	return fmt.Sprintf(
		"{M0%o SZ=%d L=%d "+
			"%d:%d "+
			"B%d*%d i%d:%d "+
			"A %f "+
			"M %f "+
			"C %f}",
		a.Mode, a.Size, a.Nlink,
		a.Uid, a.Gid,
		a.Blocks, a.Blksize,
		a.Rdev, a.Ino, ft(a.Atime, a.Atimensec), ft(a.Mtime, a.Mtimensec),
		ft(a.Ctime, a.Ctimensec))
}

func (me *CreateIn) string() string {
	return fmt.Sprintf(
		"{0%o [%s] (0%o)}", me.Mode,
		FlagString(OpenFlagNames, int64(me.Flags), "O_RDONLY"), me.Umask)
}

func (me *GetAttrIn) string() string {
	return fmt.Sprintf("{Fh %d}", me.Fh_)
}

func (me *MknodIn) string() string {
	return fmt.Sprintf("{0%o (0%o), %d}", me.Mode, me.Umask, me.Rdev)
}

func (me *ReadIn) string() string {
	return fmt.Sprintf("{Fh %d [%d +%d) %s L %d %s}",
		me.Fh, me.Offset, me.Size,
		FlagString(readFlagNames, int64(me.ReadFlags), ""),
		me.LockOwner,
		FlagString(OpenFlagNames, int64(me.Flags), "RDONLY"))
}

func (me *WriteIn) string() string {
	return fmt.Sprintf("{Fh %d [%d +%d) %s L %d %s}",
		me.Fh, me.Offset, me.Size,
		FlagString(writeFlagNames, int64(me.WriteFlags), ""),
		me.LockOwner,
		FlagString(OpenFlagNames, int64(me.Flags), "RDONLY"))
}
