package system // import "github.com/docker/docker/pkg/system"

import (
	"fmt"
	"path/filepath"
	"strings"

	"golang.org/x/sys/windows"
)

// GetLongPathName converts Windows short pathnames to full pathnames.
// For example C:\Users\ADMIN~1 --> C:\Users\Administrator.
// It is a no-op on non-Windows platforms
func GetLongPathName(path string) (string, error) {
	// See https://groups.google.com/forum/#!topic/golang-dev/1tufzkruoTg
	p, err := windows.UTF16FromString(path)
	if err != nil {
		return "", err
	}
	b := p // GetLongPathName says we can reuse buffer
	n, err := windows.GetLongPathName(&p[0], &b[0], uint32(len(b)))
	if err != nil {
		return "", err
	}
	if n > uint32(len(b)) {
		b = make([]uint16, n)
		_, err = windows.GetLongPathName(&p[0], &b[0], uint32(len(b)))
		if err != nil {
			return "", err
		}
	}
	return windows.UTF16ToString(b), nil
}

// checkSystemDriveAndRemoveDriveLetter is the Windows implementation
// of CheckSystemDriveAndRemoveDriveLetter
func checkSystemDriveAndRemoveDriveLetter(path string, driver PathVerifier) (string, error) {
	if len(path) == 2 && string(path[1]) == ":" {
		return "", fmt.Errorf("No relative path specified in %q", path)
	}
	if !driver.IsAbs(path) || len(path) < 2 {
		return filepath.FromSlash(path), nil
	}
	if string(path[1]) == ":" && !strings.EqualFold(string(path[0]), "c") {
		return "", fmt.Errorf("The specified path is not on the system drive (C:)")
	}
	return filepath.FromSlash(path[2:]), nil
}
