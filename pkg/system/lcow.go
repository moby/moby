package system // import "github.com/docker/docker/pkg/system"

import (
	"runtime"
)

// IsOSSupported determines if an operating system is supported by the host
func IsOSSupported(os string) bool {
	if runtime.GOOS == os {
		return true
	}
	if LCOWSupported() && os == "linux" {
		return true
	}
	return false
}
