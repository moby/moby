package daemon

import "tags.cncf.io/container-device-interface/pkg/cdi"

// RegisterGPUDeviceDrivers registers GPU device drivers.
// If the cdiCache is provided, it is used to detect presence of CDI specs for AMD GPUs.
// For NVIDIA GPUs, presence of CDI specs is detected by checking for the nvidia-cdi-hook binary.
func RegisterGPUDeviceDrivers(cdiCache *cdi.Cache) {
	// Register NVIDIA device drivers.
	if nvidiaDrivers := getNVIDIADeviceDrivers(); len(nvidiaDrivers) > 0 {
		for name, driver := range nvidiaDrivers {
			registerDeviceDriver(name, driver)
		}
		return
	}

	// Register AMD driver if AMD CDI spec or helper binary is present.
	if amdDriver := getAMDDeviceDrivers(cdiCache); amdDriver != nil {
		registerDeviceDriver("amd", amdDriver)
		return
	}
}
