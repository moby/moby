package seccomp

import (
	"bytes"
	"fmt"
	"sync"

	"golang.org/x/sys/unix"
)

// kernelVersion holds information about the kernel.
type kernelVersion struct {
	kernel uint // Version of the kernel (i.e., the "4" in "4.1.2-generic")
	major  uint // Major revision of the kernel (i.e., the "1" in "4.1.2-generic")
}

var (
	currentKernelVersion *kernelVersion
	kernelVersionError   error
	once                 sync.Once
)

// getKernelVersion gets the current kernel version.
func getKernelVersion() (*kernelVersion, error) {
	once.Do(func() {
		var uts unix.Utsname
		if err := unix.Uname(&uts); err != nil {
			return
		}
		// Remove the \x00 from the release for Atoi to parse correctly
		currentKernelVersion, kernelVersionError = parseRelease(string(uts.Release[:bytes.IndexByte(uts.Release[:], 0)]))
	})
	return currentKernelVersion, kernelVersionError
}

// parseRelease parses a string and creates a kernelVersion based on it.
func parseRelease(release string) (*kernelVersion, error) {
	var version = kernelVersion{}

	// We're only make sure we get the "kernel" and "major revision". Sometimes we have
	// 3.12.25-gentoo, but sometimes we just have 3.12-1-amd64.
	_, err := fmt.Sscanf(release, "%d.%d", &version.kernel, &version.major)
	if err != nil {
		return nil, fmt.Errorf("failed to parse kernel version %q: %w", release, err)
	}
	return &version, nil
}

// kernelGreaterEqualThan checks if the host's kernel version is greater than, or
// equal to the given kernel version v. Only "kernel version" and "major revision"
// can be specified (e.g., "3.12") and will be taken into account, which means
// that 3.12.25-gentoo and 3.12-1-amd64 are considered equal (kernel: 3, major: 12).
func kernelGreaterEqualThan(v string) (bool, error) {
	minVersion, err := parseRelease(v)
	if err != nil {
		return false, err
	}
	kv, err := getKernelVersion()
	if err != nil {
		return false, err
	}
	if kv.kernel > minVersion.kernel {
		return true, nil
	}
	if kv.kernel == minVersion.kernel && kv.major >= minVersion.major {
		return true, nil
	}
	return false, nil
}
