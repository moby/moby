package system

import (
	"errors"
	"runtime"
	"strings"
)

// ErrNotSupportedOperatingSystem means the operating system is not supported.
//
// Deprecated: use [github.com/docker/docker/image.CheckOS] and check the error returned.
var ErrNotSupportedOperatingSystem = errors.New("operating system is not supported")

// IsOSSupported determines if an operating system is supported by the host.
//
// Deprecated: use [github.com/docker/docker/image.CheckOS] and check the error returned.
func IsOSSupported(os string) bool {
	return strings.EqualFold(runtime.GOOS, os)
}
