// +build !linux

package system

import (
	"fmt"
	"runtime"
)

func CloseFdsFrom(minFd int) error {
	return fmt.Errorf("CloseFdsFrom is unsupported on this platform (%s/%s)", runtime.GOOS, runtime.GOARCH)
}
