package system

import (
	"os"
	"path/filepath"
	"strings"
)

// IsAbs is a platform-agnostic wrapper for filepath.IsAbs.
//
// On Windows, golang filepath.IsAbs does not consider a path \windows\system32
// as absolute as it doesn't start with a drive-letter/colon combination. However,
// in docker we need to verify things such as WORKDIR /windows/system32 in
// a Dockerfile (which gets translated to \windows\system32 when being processed
// by the daemon). This SHOULD be treated as absolute from a docker processing
// perspective.
func IsAbs(path string) bool {
	return filepath.IsAbs(path) || strings.HasPrefix(path, string(os.PathSeparator))
}

// MkdirAll creates a directory named path along with any necessary parents,
// with permission specified by attribute perm for all dir created.
//
// Deprecated: [os.MkdirAll] now natively supports Windows GUID volume paths, and should be used instead. This alias will be removed in the next release.
func MkdirAll(path string, perm os.FileMode) error {
	return os.MkdirAll(path, perm)
}
