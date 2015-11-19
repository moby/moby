package operatingsystem

import (
	"errors"
)

// GetOperatingSystem gets the name of the current operating system.
func GetOperatingSystem() (string, error) {
	// TODO: Implement OS detection
	return "", errors.New("cannot detect OS version")
}

// IsContainerized returns true if we are running inside a container.
// No-op on FreeBSD, always returns false.
func IsContainerized() (bool, error) {
	// TODO: Implement jail detection
	return false, errors.New("cannot detect if we are in container")
}
