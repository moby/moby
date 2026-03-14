package daemon

import (
	"fmt"

	"github.com/opencontainers/runtime-spec/specs-go"
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

// a cdiCacheInjector uses the specified CDI cache to inject device requests
// into a CDI spec.
type cdiCacheInjector struct {
	cdiCache *cdi.Cache
	// optionalVendor allows a specific vendor to be specified.
	// If this is not specified, the first supported vendor will be chosen.
	optionalVendor string
}

func createCDIInjector(cdiCache *cdi.Cache, optionalVendor string) *cdiCacheInjector {
	return &cdiCacheInjector{
		cdiCache:       cdiCache,
		optionalVendor: optionalVendor,
	}
}

func (c *cdiCacheInjector) injectDevices(s *specs.Spec, dev *deviceInstance) error {
	vendor, err := getFirstAvailableVendor(c.cdiCache.ListVendors())
	if err != nil {
		return fmt.Errorf("failed to discover GPU vendor from CDI: %w", err)
	}
	if c.optionalVendor != "" && vendor != c.optionalVendor {
		return fmt.Errorf("CDI specs for required vendor %v not found", c.optionalVendor)
	}
	injector := &cdiDeviceInjector{
		defaultCDIDeviceKind: c.optionalVendor + "/gpu",
	}
	return injector.injectDevices(s, dev)
}
