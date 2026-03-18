package linux

import (
	"fmt"
	"sync"

	"github.com/cilium/ebpf/internal"
	"github.com/cilium/ebpf/internal/unix"
)

// KernelVersion returns the version of the currently running kernel.
var KernelVersion = sync.OnceValues(detectKernelVersion)

// detectKernelVersion returns the version of the running kernel.
func detectKernelVersion() (internal.Version, error) {
	vc, err := vdsoVersion()
	if err != nil {
		return internal.Version{}, err
	}
	return internal.NewVersionFromCode(vc), nil
}

// KernelRelease returns the release string of the running kernel.
// Its format depends on the Linux distribution and corresponds to directory
// names in /lib/modules by convention. Some examples are 5.15.17-1-lts and
// 4.19.0-16-amd64.
func KernelRelease() (string, error) {
	var uname unix.Utsname
	if err := unix.Uname(&uname); err != nil {
		return "", fmt.Errorf("uname failed: %w", err)
	}

	return unix.ByteSliceToString(uname.Release[:]), nil
}
