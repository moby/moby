package daemon

import (
	"tags.cncf.io/container-device-interface/pkg/cdi"
)

// RegisterGPUDeviceDrivers registers GPU device drivers.
// If the cdiCache is provided, it is used to discover the vendor via available CDI specs
// and translate the GPU requests to CDI device requests.
func RegisterGPUDeviceDrivers(cdiCache *cdi.Cache) {
	// Register NVIDIA device drivers.
	if nvidiaDrivers := getNVIDIADeviceDrivers(cdiCache); len(nvidiaDrivers) > 0 {
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
