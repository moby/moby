package system // import "github.com/docker/docker/pkg/system"

import (
	"time"

	"golang.org/x/sys/windows"
)

// setCTime will set the create time on a file. On Windows, this requires
// calling SetFileTime and explicitly including the create time.
func setCTime(path string, ctime time.Time) error {
	pathp, err := windows.UTF16PtrFromString(path)
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
	c := windows.NsecToFiletime(ctime.UnixNano())
	return windows.SetFileTime(h, &c, nil, nil)
}
