package system

import (
	"os"
)

const BuildkitTmpDirEnvVar = "BUILDKIT_TMPDIR"

func MkdirTemp(dir string, pattern string) (string, error) {
	if dir == "" {
		tempDirRootPath := os.Getenv(BuildkitTmpDirEnvVar)
		fileInfo, err := os.Stat(tempDirRootPath)
		if err == nil && fileInfo.IsDir() {
			dir = tempDirRootPath
		}
	}
	return os.MkdirTemp(dir, pattern)
}
