package archive

import "github.com/moby/go-archive"

// CheckSystemDriveAndRemoveDriveLetter verifies that a path is the system drive.
//
// Deprecated: use [archive.CheckSystemDriveAndRemoveDriveLetter] instead.
func CheckSystemDriveAndRemoveDriveLetter(path string) (string, error) {
	return archive.CheckSystemDriveAndRemoveDriveLetter(path)
}
