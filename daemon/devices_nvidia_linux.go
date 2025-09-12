package daemon

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/v2/daemon/internal/capabilities"
	"github.com/opencontainers/runtime-spec/specs-go"
)

// TODO: nvidia should not be hard-coded, and should be a device plugin instead on the daemon object.
// TODO: add list of device capabilities in daemon/node info

var errConflictCountDeviceIDs = errors.New("cannot set both Count and DeviceIDs on device request")

const (
	nvidiaContainerRuntimeHookExecutableName = "nvidia-container-runtime-hook"
	nvidiaCDIHookExecutableName              = "nvidia-cdi-hook"
	amdContainerRuntimeExecutableName        = "amd-container-runtime"
)

// These are NVIDIA-specific capabilities stolen from github.com/containerd/containerd/contrib/nvidia.allCaps
var allNvidiaCaps = map[string]struct{}{
	"compute":  {},
	"compat32": {},
	"graphics": {},
	"utility":  {},
	"video":    {},
	"display":  {},
}

func init() {
	// Register NVIDIA device drivers.
	if nvidiaDrivers := getNVIDIADeviceDrivers(); len(nvidiaDrivers) > 0 {
		for name, driver := range nvidiaDrivers {
			registerDeviceDriver(name, driver)
		}
		return
	}

	// Register AMD driver if AMD helper binary is present.
	if _, err := exec.LookPath(amdContainerRuntimeExecutableName); err == nil {
		registerDeviceDriver("amd", &deviceDriver{
			capset:     capabilities.Set{"gpu": struct{}{}, "amd": struct{}{}},
			updateSpec: setAMDGPUs,
		})
		return
	}

	// No "gpu" capability
}

func getNVIDIADeviceDrivers() map[string]*deviceDriver {
	hasNVIDIAExecutables := make(map[string]struct{})
	for _, e := range []string{nvidiaContainerRuntimeHookExecutableName, nvidiaCDIHookExecutableName} {
		if _, err := exec.LookPath(e); err != nil {
			continue
		}
		hasNVIDIAExecutables[e] = struct{}{}
	}

	if len(hasNVIDIAExecutables) == 0 {
		return nil
	}

	nvidiaDrivers := make(map[string]*deviceDriver)
	// Register a driver specific to the nvidia-container-runtime-hook if present.
	// This has no capabilities associated with it so as to not inadvertently
	// match requests.
	if _, ok := hasNVIDIAExecutables[nvidiaContainerRuntimeHookExecutableName]; ok {
		nvidiaDrivers["nvidia.runtime-hook"] = &deviceDriver{
			capset:     nil,
			updateSpec: setNvidiaGPUs,
		}
	}

	// Register a driver specific to CDI if present.
	// This has no capabilities associated with it so as to not inadvertently
	// match requests.
	if _, ok := hasNVIDIAExecutables[nvidiaCDIHookExecutableName]; ok {
		nvidiaDrivers["nvidia.cdi"] = &deviceDriver{
			capset: nil,
			updateSpec: (&cdiDeviceInjector{
				defaultCDIDeviceKind: "nvidia.com/gpu",
			}).injectDevices,
		}
	}

	var composite specUpdaters
	// We construct a composite handler that prefers nvidia.cdi over nvidia.runtime-hook
	for _, driverName := range []string{"nvidia.cdi", "nvidia.runtime-hook"} {
		d := nvidiaDrivers[driverName]
		if d == nil || d.updateSpec == nil {
			continue
		}
		composite = append(composite, d.updateSpec)
	}

	// We associate all NVIDIA capabilities with this composite driver.
	capset := capabilities.Set{"gpu": struct{}{}, "nvidia": struct{}{}}
	for c := range allNvidiaCaps {
		capset[c] = struct{}{}
	}
	nvidiaDrivers["nvidia"] = &deviceDriver{
		capset:     capset,
		updateSpec: composite.firstSuccessful,
	}

	return nvidiaDrivers
}

// specUpdaters refer to a list of functions used updated an OCI spec for a
// given device instance.
type specUpdaters []func(*specs.Spec, *deviceInstance) error

// firstSuccessful attempts to apply the list of spec updaters and returns on
// the first successful update.
func (h specUpdaters) firstSuccessful(s *specs.Spec, dev *deviceInstance) error {
	var errs error
	for _, handler := range h {
		if handler == nil {
			continue
		}
		if err := handler(s, dev); err != nil {
			errs = errors.Join(errs, err)
			continue
		}
		return nil
	}

	return errs
}

// setNvidiaGPUs handles requestes for NVIDIA GPUs.
// This is done by updating the OCI runtime spec to include the correct value
// for the NVIDIA_VISIBLE_DEVICES environment variable and injecting the
// NVIDIA Container Runtime Hook as a container prestart hook.
func setNvidiaGPUs(s *specs.Spec, dev *deviceInstance) error {
	req := dev.req
	if req.Count != 0 && len(req.DeviceIDs) > 0 {
		return errConflictCountDeviceIDs
	}

	deviceIDs := getRequestedDevicesIDs(req)
	if len(deviceIDs) == 0 {
		return nil
	}
	s.Process.Env = append(s.Process.Env, "NVIDIA_VISIBLE_DEVICES="+strings.Join(deviceIDs, ","))

	var nvidiaCaps []string
	// req.Capabilities contains device capabilities, some but not all are NVIDIA driver capabilities.
	for _, c := range dev.selectedCaps {
		if _, isNvidiaCap := allNvidiaCaps[c]; isNvidiaCap {
			nvidiaCaps = append(nvidiaCaps, c)
			continue
		}
		// TODO: nvidia.WithRequiredCUDAVersion
		// for now we let the prestart hook verify cuda versions but errors are not pretty.
	}

	if nvidiaCaps != nil {
		s.Process.Env = append(s.Process.Env, "NVIDIA_DRIVER_CAPABILITIES="+strings.Join(nvidiaCaps, ","))
	}

	path, err := exec.LookPath(nvidiaContainerRuntimeHookExecutableName)
	if err != nil {
		return err
	}

	if s.Hooks == nil {
		s.Hooks = &specs.Hooks{}
	}

	// This implementation uses prestart hooks, which are deprecated.
	// CreateRuntime is the closest equivalent, and executed in the same
	// locations as prestart-hooks, but depending on what these hooks do,
	// possibly one of the other hooks could be used instead (such as
	// CreateContainer or StartContainer).
	s.Hooks.Prestart = append(s.Hooks.Prestart, specs.Hook{ //nolint:staticcheck // FIXME(thaJeztah); replace prestart hook with a non-deprecated one.
		Path: path,
		Args: []string{
			nvidiaContainerRuntimeHookExecutableName,
			"prestart",
		},
		Env: os.Environ(),
	})

	return nil
}

// getRequestedDeviceIDs returns the list of requested devices by ID based on
// the device request.
func getRequestedDevicesIDs(req container.DeviceRequest) []string {
	switch {
	case len(req.DeviceIDs) > 0:
		return req.DeviceIDs
	case req.Count > 0:
		return countToDevices(req.Count)
	case req.Count < 0:
		return []string{"all"}
	case req.Count == 0:
		return nil
	}
	return nil
}

// countToDevices returns the list 0, 1, ... count-1 of deviceIDs.
func countToDevices(count int) []string {
	devices := make([]string, count)
	for i := range devices {
		devices[i] = strconv.Itoa(i)
	}
	return devices
}

// A cdiDeviceInjector is used to map regular device requests to CDI device
// requests.
type cdiDeviceInjector struct {
	defaultCDIDeviceKind string
}

// injectDevices converts an incoming device request to a request for devices
// using CDI.
// The requested device IDs are converted to CDI device names if required using
// the specified default kind.
func (i *cdiDeviceInjector) injectDevices(s *specs.Spec, dev *deviceInstance) error {
	var cdiDeviceIDs []string
	for _, deviceID := range getRequestedDevicesIDs(dev.req) {
		cdiDeviceIDs = append(cdiDeviceIDs, i.normalizeDeviceID(deviceID))
	}
	if len(cdiDeviceIDs) == 0 {
		return nil
	}

	// If the cdi device driver is not available then we return an error.
	cdiDeviceDriver := deviceDrivers["cdi"]
	if cdiDeviceDriver == nil {
		return fmt.Errorf("no CDI device driver registered: %w", incompatibleDeviceRequest{dev.req.Driver, dev.req.Capabilities})
	}

	// We construct a device instance using the CDI device IDs and forward this
	// to the cdiDeviceDriver.
	cdiRequest := dev.req
	cdiRequest.Count = 0
	cdiRequest.Options = nil
	cdiRequest.DeviceIDs = cdiDeviceIDs

	cdiDeviceInstance := deviceInstance{
		req:          cdiRequest,
		selectedCaps: nil,
	}

	return cdiDeviceDriver.updateSpec(s, &cdiDeviceInstance)
}

// normalizeDeviceID ensures that the specified deviceID is a fully-qualified
// CDI device name.
// If the deviceID is already a fully-qualified CDI device name it is returned
// as-is, otherwise, the defailt CDI device kind (vendor/class) is used to
// construct a fully qualified CDI device name.
func (i *cdiDeviceInjector) normalizeDeviceID(deviceID string) string {
	// TODO: We should ideally use the parser from the tags.cncf.io/cdi packages.
	parts := strings.SplitN(deviceID, "=", 2)
	if len(parts) == 2 {
		return deviceID
	}

	return i.defaultCDIDeviceKind + "=" + deviceID
}
