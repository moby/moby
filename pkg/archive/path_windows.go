package archive

import (
	"fmt"
	"path/filepath"
	"strings"
)

// checkSystemDriveAndRemoveDriveLetter is the Windows implementation
// of CheckSystemDriveAndRemoveDriveLetter
func checkSystemDriveAndRemoveDriveLetter(path string) (string, error) {
	if len(path) == 2 && string(path[1]) == ":" {
		return "", fmt.Errorf("no relative path specified in %q", path)
	}
	if !filepath.IsAbs(path) || len(path) < 2 {
		return filepath.FromSlash(path), nil
	}
	if string(path[1]) == ":" && !strings.EqualFold(string(path[0]), "c") {
		return "", fmt.Errorf("the specified path is not on the system drive (C:)")
	}
	return filepath.FromSlash(path[2:]), nil
}

// lgetxattr is not supported on Windows.
func lgetxattr(path string, attr string) ([]byte, error) {
	return nil, nil
}

// lsetxattr is not supported on Windows.
func lsetxattr(path string, attr string, data []byte, flags int) error {
	return nil
}
