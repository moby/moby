//go:build !windows
// +build !windows

package system // import "github.com/docker/docker/pkg/system"

// checkSystemDriveAndRemoveDriveLetter is the non-Windows implementation
// of CheckSystemDriveAndRemoveDriveLetter
func checkSystemDriveAndRemoveDriveLetter(path string) (string, error) {
	return path, nil
}
