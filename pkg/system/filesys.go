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
