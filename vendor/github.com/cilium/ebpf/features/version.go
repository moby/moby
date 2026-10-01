package features

import "github.com/cilium/ebpf/internal/linux"

// LinuxVersionCode returns the version of the currently running kernel
// as defined in the LINUX_VERSION_CODE compile-time macro. It is represented
// in the format described by the KERNEL_VERSION macro from linux/version.h.
//
// Do not use the version to make assumptions about the presence of certain
// kernel features, always prefer feature probes in this package. Some
// distributions backport or disable eBPF features.
func LinuxVersionCode() (uint32, error) {
	v, err := linux.KernelVersion()
	if err != nil {
		return 0, err
	}
	return v.Kernel(), nil
}
