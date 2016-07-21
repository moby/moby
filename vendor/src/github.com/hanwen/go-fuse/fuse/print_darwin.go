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
			"A %d.%09d "+
			"M %d.%09d "+
			"C %d.%09d}",
		a.Mode, a.Size, a.Nlink,
		a.Uid, a.Gid,
		a.Blocks,
		a.Rdev, a.Ino, a.Atime, a.Atimensec, a.Mtime, a.Mtimensec,
		a.Ctime, a.Ctimensec)
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
	return fmt.Sprintf("{Fh %d off %d sz %d %s L %d %s}",
		me.Fh, me.Offset, me.Size,
		FlagString(readFlagNames, int64(me.ReadFlags), ""))
}

func (me *WriteIn) string() string {
	return fmt.Sprintf("{Fh %d off %d sz %d %s}",
		me.Fh, me.Offset, me.Size,
		FlagString(writeFlagNames, int64(me.WriteFlags), ""))
}
