// Copyright 2016 the Go-FUSE Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package fuse

import (
	"fmt"
)

func init() {
	initFlagNames[CAP_XTIMES] = "XTIMES"
	initFlagNames[CAP_VOL_RENAME] = "VOL_RENAME"
	initFlagNames[CAP_CASE_INSENSITIVE] = "CASE_INSENSITIVE"
}

func (a *Attr) string() string {
	return fmt.Sprintf(
		"{M0%o SZ=%d L=%d "+
			"%d:%d "+
			"%d %d:%d "+
			"A %f "+
			"M %f "+
			"C %f}",
		a.Mode, a.Size, a.Nlink,
		a.Uid, a.Gid,
		a.Blocks,
		a.Rdev, a.Ino, ft(a.Atime, a.Atimensec), ft(a.Mtime, a.Mtimensec),
		ft(a.Ctime, a.Ctimensec))
}

func (me *CreateIn) string() string {
	return fmt.Sprintf(
		"{0%o [%s]}", me.Mode,
		FlagString(OpenFlagNames, int64(me.Flags), "O_RDONLY"))
}

func (me *GetAttrIn) string() string { return "" }

func (me *MknodIn) string() string {
	return fmt.Sprintf("{0%o, %d}", me.Mode, me.Rdev)
}

func (me *ReadIn) string() string {
	return fmt.Sprintf("{Fh %d [%d +%d) %s}",
		me.Fh, me.Offset, me.Size,
		FlagString(readFlagNames, int64(me.ReadFlags), ""))
}

func (me *WriteIn) string() string {
	return fmt.Sprintf("{Fh %d [%d +%d) %s}",
		me.Fh, me.Offset, me.Size,
		FlagString(writeFlagNames, int64(me.WriteFlags), ""))
}
