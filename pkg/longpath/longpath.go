// Package longpath introduces some constants and helper functions for handling
// long paths in Windows.
//
// Long paths are expected to be prepended with "\\?\" and followed by either a
// drive letter, a UNC server\share, or a volume identifier.
package longpath // import "github.com/docker/docker/pkg/longpath"

import (
	"os"
	"runtime"
	"strings"
)

// Prefix is the longpath prefix for Windows file paths.
const Prefix = `\\?\`

// AddPrefix adds the Windows long path prefix to the path provided if
// it does not already have it.
func AddPrefix(path string) string {
	if !strings.HasPrefix(path, Prefix) {
		if strings.HasPrefix(path, `\\`) {
			// This is a UNC path, so we need to add 'UNC' to the path as well.
			path = Prefix + `UNC` + path[1:]
		} else {
			path = Prefix + path
		}
	}
	return path
}

// MkdirTemp is the equivalent of [os.MkdirTemp], except that on Windows
// the result is in Windows longpath format. On Unix systems it is
// equivalent to [os.MkdirTemp].
func MkdirTemp(dir, prefix string) (string, error) {
	tempDir, err := os.MkdirTemp(dir, prefix)
	if err != nil {
		return "", err
	}
	if runtime.GOOS != "windows" {
		return tempDir, nil
	}
	return AddPrefix(tempDir), nil
}
