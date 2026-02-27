package daemon

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/moby/moby/v2/daemon/internal/capabilities"
	"github.com/opencontainers/runtime-spec/specs-go"
	"tags.cncf.io/container-device-interface/pkg/cdi"
)

const (
	amdContainerRuntimeExecutableName = "amd-container-runtime"
)

func setAMDGPUs(s *specs.Spec, dev *deviceInstance) error {
	req := dev.req
	if req.Count != 0 && len(req.DeviceIDs) > 0 {
		return errConflictCountDeviceIDs
	}

	switch {
	case len(req.DeviceIDs) > 0:
		s.Process.Env = append(s.Process.Env, "AMD_VISIBLE_DEVICES="+strings.Join(req.DeviceIDs, ","))
	case req.Count > 0:
		s.Process.Env = append(s.Process.Env, "AMD_VISIBLE_DEVICES="+strings.Join(countToDevices(req.Count), ","))
	case req.Count < 0:
		s.Process.Env = append(s.Process.Env, "AMD_VISIBLE_DEVICES=all")
	case req.Count == 0:
		s.Process.Env = append(s.Process.Env, "AMD_VISIBLE_DEVICES=void")
	}

	return nil
}

func createAMDCDIUpdater(vendorList []string) func(*specs.Spec, *deviceInstance) error {
	return func(s *specs.Spec, dev *deviceInstance) error {
		vendor, err := getFirstAvailableVendor(vendorList)
		if err != nil {
			return fmt.Errorf("failed to discover GPU vendor from CDI: %w", err)
		}

		if vendor != "amd.com" {
			return fmt.Errorf("AMD CDI spec not found")
		}

		injector := &cdiDeviceInjector{
			defaultCDIDeviceKind: "amd.com/gpu",
		}
		return injector.injectDevices(s, dev)
	}
}

func getAMDDeviceDrivers(cdiCache *cdi.Cache) *deviceDriver {
	var composite firstSuccessfulUpdater

	if cdiCache != nil {
		composite = append(composite, createAMDCDIUpdater(cdiCache.ListVendors()))
	}

	if _, err := exec.LookPath(amdContainerRuntimeExecutableName); err == nil {
		composite = append(composite, setAMDGPUs)
	}

	if len(composite) == 0 {
		return nil
	}

	// We do not support specifying driver with device requests for AMD GPUs.
	// Hence only use the composite updater and try cdi and runtime driver in sequence
	// based on availability.
	capset := capabilities.Set{"gpu": struct{}{}, "amd": struct{}{}}
	return &deviceDriver{
		capset:     capset,
		updateSpec: composite.updateSpec,
	}
}
