package fs

import (
	"errors"
	"path/filepath"

	"golang.org/x/sys/windows"
)

var (
	// ErrInvalidPath is returned when the location of a file path doesn't begin with a driver letter.
	ErrInvalidPath = errors.New("the path provided to GetFileSystemType must start with a drive letter")
)

// GetFileSystemType obtains the type of a file system through GetVolumeInformation.
// https://msdn.microsoft.com/en-us/library/windows/desktop/aa364993(v=vs.85).aspx
func GetFileSystemType(path string) (fsType string, err error) {
	drive := filepath.VolumeName(path)
	if len(drive) != 2 {
		return "", ErrInvalidPath
	}

	var (
		buf  = make([]uint16, 255)
		size = uint32(windows.MAX_PATH + 1)
	)
	drive += `\`
	err = windows.GetVolumeInformation(windows.StringToUTF16Ptr(drive), nil, 0, nil, nil, nil, &buf[0], size)
	fsType = windows.UTF16ToString(buf)
	return fsType, err
}
