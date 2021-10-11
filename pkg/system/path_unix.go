//go:build !windows
// +build !windows

package system // import "github.com/docker/docker/pkg/system"

// GetLongPathName converts Windows short pathnames to full pathnames.
// For example C:\Users\ADMIN~1 --> C:\Users\Administrator.
// It is a no-op on non-Windows platforms
func GetLongPathName(path string) (string, error) {
	return path, nil
}

// checkSystemDriveAndRemoveDriveLetter is the non-Windows implementation
// of CheckSystemDriveAndRemoveDriveLetter
func checkSystemDriveAndRemoveDriveLetter(path string, driver PathVerifier) (string, error) {
	return path, nil
}
