package linux

import (
	"fmt"
	"os"
)

// FindKConfig searches for a kconfig file on the host.
//
// It first reads from /boot/config- of the current running kernel and tries
// /proc/config.gz if nothing was found in /boot.
// If none of the file provide a kconfig, it returns an error.
func FindKConfig() (*os.File, error) {
	kernelRelease, err := KernelRelease()
	if err != nil {
		return nil, fmt.Errorf("cannot get kernel release: %w", err)
	}

	path := "/boot/config-" + kernelRelease
	f, err := os.Open(path)
	if err == nil {
		return f, nil
	}

	f, err = os.Open("/proc/config.gz")
	if err == nil {
		return f, nil
	}

	return nil, fmt.Errorf("neither %s nor /proc/config.gz provide a kconfig", path)
}
