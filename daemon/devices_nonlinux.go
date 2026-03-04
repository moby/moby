//go:build !linux

package daemon

import "tags.cncf.io/container-device-interface/pkg/cdi"

// RegisterGPUDeviceDrivers is a no-op on non-Linux platforms.
func RegisterGPUDeviceDrivers(_ *cdi.Cache) {}
