//go:build linux || freebsd || openbsd

package kernel // import "github.com/docker/docker/pkg/parsers/kernel"

import (
	"context"

	"github.com/containerd/log"
	"golang.org/x/sys/unix"
)

// GetKernelVersion gets the current kernel version.
func GetKernelVersion() (*VersionInfo, error) {
	uts, err := uname()
	if err != nil {
		return nil, err
	}

	// Remove the \x00 from the release for Atoi to parse correctly
	return ParseRelease(unix.ByteSliceToString(uts.Release[:]))
}

// CheckKernelVersion checks if current kernel is newer than (or equal to)
// the given version.
func CheckKernelVersion(k, major, minor int) bool {
	if v, err := GetKernelVersion(); err != nil {
		log.G(context.TODO()).Warnf("error getting kernel version: %s", err)
	} else {
		if CompareKernelVersion(*v, VersionInfo{Kernel: k, Major: major, Minor: minor}) < 0 {
			return false
		}
	}
	return true
}
