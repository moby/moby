package daemon

import (
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

func createAMDCDIUpdater(cdiCache *cdi.Cache) func(*specs.Spec, *deviceInstance) error {
	return func(s *specs.Spec, dev *deviceInstance) error {
		injector := createCDIInjector(cdiCache, "amd.com")
		return injector.injectDevices(s, dev)
	}
}

func getAMDDeviceDrivers(cdiCache *cdi.Cache) *deviceDriver {
	var composite firstSuccessfulUpdater

	if cdiCache != nil {
		composite = append(composite, createAMDCDIUpdater(cdiCache))
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
