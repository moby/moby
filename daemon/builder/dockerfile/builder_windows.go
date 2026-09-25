package dockerfile

import (
	"os"
	"path/filepath"
	"strings"
)

func defaultShellForOS(os string) []string {
	if os == "linux" {
		return []string{"/bin/sh", "-c"}
	}
	return []string{"cmd", "/S", "/C"}
}

// isAbs is a platform-agnostic wrapper for filepath.IsAbs.
//
// On Windows, golang filepath.IsAbs does not consider a path \windows\system32
// as absolute as it doesn't start with a drive-letter/colon combination. However,
// in docker we need to verify things such as WORKDIR /windows/system32 in
// a Dockerfile (which gets translated to \windows\system32 when being processed
// by the daemon). This SHOULD be treated as absolute from a docker processing
// perspective.
func isAbs(path string) bool {
	return filepath.IsAbs(path) || strings.HasPrefix(path, string(os.PathSeparator))
}
