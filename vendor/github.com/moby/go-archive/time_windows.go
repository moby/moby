package archive

import (
	"os"
	"time"

	"golang.org/x/sys/windows"
)

// chtimes changes the access and modification time of a file at the given
// path.
//
// Callers must use boundTime to ensure timestamps are within the range
// supported by os.Chtimes.
func chtimes(name string, atime time.Time, mtime time.Time) error {
	if err := os.Chtimes(name, atime, mtime); err != nil {
		return err
	}

	pathp, err := windows.UTF16PtrFromString(name)
	if err != nil {
		return err
	}
	h, err := windows.CreateFile(pathp,
		windows.FILE_WRITE_ATTRIBUTES, windows.FILE_SHARE_WRITE, nil,
		windows.OPEN_EXISTING, windows.FILE_FLAG_BACKUP_SEMANTICS, 0)
	if err != nil {
		return err
	}
	defer windows.Close(h)
	c := windows.NsecToFiletime(mtime.UnixNano())
	return windows.SetFileTime(h, &c, nil, nil)
}

func lchtimes(root *os.Root, name string, atime time.Time, mtime time.Time) error {
	return nil
}
