package utils

import (
	"os"
	"path/filepath"
)

// TempDir returns the default directory to use for temporary files.
func TempDir(rootDir string) (string, error) {
	var tmpDir string
	if tmpDir = os.Getenv("DOCKER_TMPDIR"); tmpDir == "" {
		tmpDir = filepath.Join(rootDir, "tmp")
	}
	err := os.MkdirAll(tmpDir, 0700)
	return tmpDir, err
}
