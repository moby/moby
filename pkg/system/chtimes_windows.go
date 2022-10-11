package system // import "github.com/docker/docker/pkg/system"

import (
	"time"

	"golang.org/x/sys/windows"
)

// setCTime will set the create time on a file. On Windows, this requires
// calling SetFileTime and explicitly including the create time.
func setCTime(path string, ctime time.Time) error {
	ctimespec := windows.NsecToTimespec(ctime.UnixNano())
	pathp, e := windows.UTF16PtrFromString(path)
	if e != nil {
		return e
	}
	h, e := windows.CreateFile(pathp,
		windows.FILE_WRITE_ATTRIBUTES, windows.FILE_SHARE_WRITE, nil,
		windows.OPEN_EXISTING, windows.FILE_FLAG_BACKUP_SEMANTICS, 0)
	if e != nil {
		return e
	}
	defer windows.Close(h)
	c := windows.NsecToFiletime(windows.TimespecToNsec(ctimespec))
	return windows.SetFileTime(h, &c, nil, nil)
}

// setAMTimeNoFollow means to set access/modification time on a file,
// without following symbol link.
// The implementation returns ErrNotSupportedPlatform on windows, ATM.
func setAMTimeNoFollow(path string, atime time.Time, mtime time.Time) error {
	return ErrNotSupportedPlatform
}

// setCTimeNoFollow means to set creation time on a file,
// without following symbol link.
// The implementation returns ErrNotSupportedPlatform on windows, ATM.
func setCTimeNoFollow(path string, ctime time.Time) error {
	return ErrNotSupportedPlatform
}
