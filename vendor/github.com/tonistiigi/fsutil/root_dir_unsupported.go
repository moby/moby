//go:build !linux && !darwin && !freebsd && !netbsd && !openbsd && !dragonfly

package fsutil

import "os"

type rootDirState struct {
	rootDir *os.File
}
