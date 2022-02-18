//go:build !linux && !freebsd
// +build !linux,!freebsd

package system // import "github.com/moby/moby/pkg/system"

import "syscall"

// LUtimesNano is only supported on linux and freebsd.
func LUtimesNano(path string, ts []syscall.Timespec) error {
	return ErrNotSupportedPlatform
}
