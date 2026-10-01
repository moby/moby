// SPDX-FileCopyrightText: Copyright The Moby Authors
// SPDX-License-Identifier: Apache-2.0

package seccomp

import (
	"fmt"
	"sync"

	"golang.org/x/sys/unix"
)

// getKernelVersion gets the current kernel version.
var getKernelVersion = sync.OnceValues(func() (*KernelVersion, error) {
	var uts unix.Utsname
	if err := unix.Uname(&uts); err != nil {
		return nil, err
	}
	// Convert the NUL-terminated release field before parsing.
	return parseRelease(unix.ByteSliceToString(uts.Release[:]))
})

// parseRelease parses a string and creates a KernelVersion based on it.
func parseRelease(release string) (*KernelVersion, error) {
	version := KernelVersion{}

	// We're only make sure we get the "kernel" and "major revision". Sometimes we have
	// 3.12.25-gentoo, but sometimes we just have 3.12-1-amd64.
	_, err := fmt.Sscanf(release, "%d.%d", &version.Kernel, &version.Major)
	if err != nil {
		return nil, fmt.Errorf("failed to parse kernel version %q: %w", release, err)
	}
	return &version, nil
}

// kernelGreaterEqualThan checks if the host's kernel version is greater than, or
// equal to the given kernel version v. Only "kernel version" and "major revision"
// can be specified (e.g., "3.12") and will be taken into account, which means
// that 3.12.25-gentoo and 3.12-1-amd64 are considered equal (kernel: 3, major: 12).
func kernelGreaterEqualThan(minVersion KernelVersion) (bool, error) {
	kv, err := getKernelVersion()
	if err != nil {
		return false, err
	}
	if kv.Kernel > minVersion.Kernel {
		return true, nil
	}
	if kv.Kernel == minVersion.Kernel && kv.Major >= minVersion.Major {
		return true, nil
	}
	return false, nil
}
