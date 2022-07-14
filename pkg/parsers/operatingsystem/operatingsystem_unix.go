//go:build freebsd || darwin
// +build freebsd darwin

package operatingsystem // import "github.com/docker/docker/pkg/parsers/operatingsystem"

import (
	"errors"

	"golang.org/x/sys/unix"
)

// GetOperatingSystem gets the name of the current operating system.
func GetOperatingSystem() (string, error) {
	utsname := &unix.Utsname{}
	if err := unix.Uname(utsname); err != nil {
		return "", err
	}
	return unix.ByteSliceToString(utsname.Machine[:]), nil
}

// GetOperatingSystemVersion gets the version of the current operating system, as a string.
func GetOperatingSystemVersion() (string, error) {
	// there's no standard unix way of getting this, sadly...
	return "", errors.New("Unsupported on generic unix")
}

// IsContainerized returns true if we are running inside a container.
// No-op on FreeBSD and Darwin, always returns false.
func IsContainerized() (bool, error) {
	// TODO: Implement jail detection for freeBSD
	return false, errors.New("Cannot detect if we are in container")
}
