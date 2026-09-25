//go:build linux || darwin || freebsd || netbsd || openbsd || dragonfly

package fsutil

import (
	"os"
	"sync"
)

type rootDirState struct {
	rootDirOnce sync.Once
	rootDir     *os.File
	rootDirErr  error
}
