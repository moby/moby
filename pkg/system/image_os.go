package system

import (
	"errors"
	"runtime"
	"strings"
)

// ErrNotSupportedOperatingSystem means the operating system is not supported.
var ErrNotSupportedOperatingSystem = errors.New("operating system is not supported")

// IsOSSupported determines if an operating system is supported by the host.
func IsOSSupported(os string) bool {
	return strings.EqualFold(runtime.GOOS, os)
}
