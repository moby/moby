//go:build !windows

package platform

import (
	"golang.org/x/sys/unix"
)

// runtimeArchitecture gets the name of the current architecture (x86, x86_64, i86pc, sun4v, ...)
func runtimeArchitecture() (string, error) {
	utsname := &unix.Utsname{}
	if err := unix.Uname(utsname); err != nil {
		return "", err
	}
	return unix.ByteSliceToString(utsname.Machine[:]), nil
}

// NumProcs returns the number of processors on the system
//
// Deprecated: temporary stub for non-Windows to provide an alias for the deprecated github.com/docker/docker/pkg/platform package.
//
// FIXME(thaJeztah): remove once we remove  github.com/docker/docker/pkg/platform
func NumProcs() uint32 {
	return 0
}
