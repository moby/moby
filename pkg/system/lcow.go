// +build windows,!no_lcow

package system // import "github.com/docker/docker/pkg/system"

import (
	"strings"

	"github.com/Microsoft/hcsshim/osversion"
	specs "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
)

var (
	// lcowSupported determines if Linux Containers on Windows are supported.
	lcowSupported = false
)

// InitLCOW sets whether LCOW is supported or not. Requires RS5+
func InitLCOW(experimental bool) {
	if experimental && osversion.Build() >= osversion.RS5 {
		lcowSupported = true
	}
}

func LCOWSupported() bool {
	return lcowSupported
}

// ValidatePlatform determines if a platform structure is valid.
// TODO This is a temporary windows-only function, should be replaced by
// comparison of worker capabilities
func ValidatePlatform(platform specs.Platform) error {
	if !IsOSSupported(platform.OS) {
		return errors.Errorf("unsupported os %s", platform.OS)
	}
	return nil
}

// IsOSSupported determines if an operating system is supported by the host
func IsOSSupported(os string) bool {
	if strings.EqualFold("windows", os) {
		return true
	}
	if LCOWSupported() && strings.EqualFold(os, "linux") {
		return true
	}
	return false
}
