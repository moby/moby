package ioutils // import "github.com/docker/docker/pkg/ioutils"

import (
	"os"
	"runtime"

	"github.com/docker/docker/pkg/longpath"
)

// TempDir is the equivalent of [os.MkdirTemp], except that on Windows
// the result is in Windows longpath format. On Unix systems it is
// equivalent to [os.MkdirTemp].
func TempDir(dir, prefix string) (string, error) {
	tempDir, err := os.MkdirTemp(dir, prefix)
	if err != nil {
		return "", err
	}
	if runtime.GOOS != "windows" {
		return tempDir, nil
	}
	return longpath.AddPrefix(tempDir), nil
}
