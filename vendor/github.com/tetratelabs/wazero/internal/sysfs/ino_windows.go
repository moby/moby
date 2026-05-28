package sysfs

import (
	"io/fs"
	"path"

	experimentalsys "github.com/tetratelabs/wazero/experimental/sys"
	"github.com/tetratelabs/wazero/sys"
)

// inoFromFileInfo uses stat to get the inode information of the file.
func inoFromFileInfo(dirPath string, info fs.FileInfo) (ino sys.Inode, errno experimentalsys.Errno) {
	if v, ok := info.Sys().(*sys.Stat_t); ok {
		return v.Ino, 0
	}
	if dirPath == "" {
		// This is a FS.File backed implementation which doesn't have access to
		// the original file path.
		return
	}
	// Ino is no not in Win32FileAttributeData
	inoPath := path.Clean(path.Join(dirPath, info.Name()))
	var st sys.Stat_t
	if st, errno = lstat(inoPath); errno == 0 {
		ino = st.Ino
	}
	return
}
