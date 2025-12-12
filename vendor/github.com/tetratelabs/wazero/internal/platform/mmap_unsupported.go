//go:build !(linux || darwin || freebsd || netbsd || dragonfly || solaris || windows) || tinygo

package platform

import (
	"fmt"
	"runtime"
)

var errUnsupported = fmt.Errorf("mmap unsupported on GOOS=%s. Use interpreter instead.", runtime.GOOS)

func munmapCodeSegment(code []byte) error {
	panic(errUnsupported)
}

func mmapCodeSegment(size int) ([]byte, error) {
	panic(errUnsupported)
}

func MprotectRX(b []byte) (err error) {
	panic(errUnsupported)
}
