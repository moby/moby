// +build darwin dragonfly freebsd linux netbsd openbsd

package utils

import (
	"os"
	"path/filepath"
)

// TempDir returns the default directory to use for temporary files.
func TempDir(rootdir string) string {

	var tmpDir string
	if tmpDir = os.Getenv("DOCKER_TMPDIR"); tmpDir == "" {
		tmpDir = filepath.Join(rootdir, "tmp")
	}
	os.MkdirAll(tmpDir, 0700)
	return tmpDir
}
