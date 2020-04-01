// +build !windows windows,no_lcow

package system // import "github.com/docker/docker/pkg/system"
import (
	"runtime"
	"strings"

	specs "github.com/opencontainers/image-spec/specs-go/v1"
)

// InitLCOW does nothing since LCOW is a windows only feature
func InitLCOW(_ bool) {}

// LCOWSupported returns true if Linux containers on Windows are supported.
func LCOWSupported() bool {
	return false
}

// ValidatePlatform determines if a platform structure is valid. This function
// is used for LCOW, and is a no-op on non-windows platforms.
func ValidatePlatform(_ specs.Platform) error {
	return nil
}

// IsOSSupported determines if an operating system is supported by the host.
func IsOSSupported(os string) bool {
	return strings.EqualFold(runtime.GOOS, os)
}
