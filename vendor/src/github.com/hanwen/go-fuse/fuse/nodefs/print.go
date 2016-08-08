package nodefs

import (
	"fmt"

	"github.com/hanwen/go-fuse/fuse"
)

// String provides a debug string for the given file.
func (f *WithFlags) String() string {
	return fmt.Sprintf("File %s (%s) %s %s",
		f.File, f.Description, fuse.FlagString(fuse.OpenFlagNames, int64(f.OpenFlags), "O_RDONLY"),
		fuse.FlagString(fuse.FuseOpenFlagNames, int64(f.FuseFlags), ""))
}
