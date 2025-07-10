package daemon

import (
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/daemon/internal/capabilities"
	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/pkg/errors"
)

var errConflictCountDeviceIDs = errors.New("cannot set both Count and DeviceIDs on device request")

const (
	nvidiaContainerRuntimeHookExecutableName = "nvidia-container-runtime-hook"
	nvidiaCDIHookExecutableName              = "nvidia-cdi-hook"
	amdContainerRuntimeExecutableName        = "amd-container-runtime"
)

func init() {
	// Register nvidia driver if the NVIDIA Container Runtime Hook binary is present.
	if _, err := exec.LookPath(nvidiaContainerRuntimeHookExecutableName); err == nil {
		registerDeviceDriver("nvidia", &deviceDriver{
			capset:     capabilities.Set{"gpu": struct{}{}, "nvidia": struct{}{}},
			updateSpec: setNvidiaGPUs,
		})
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

	// Register a driver that handles nvidia devices as CDI devices, converting
	// non-CDI device names to nvidia.com/gpu devices.
	if _, err := exec.LookPath(nvidiaCDIHookExecutableName); err == nil {
		registerDeviceDriver("nvidia", &deviceDriver{
			capset: capabilities.Set{"gpu": struct{}{}, "nvidia": struct{}{}},
			updateSpec: (&cdiDeviceInjector{
				defaultCDIDeviceKind: "nvidia.com/gpu",
			}).injectDevices,
		})
	}

	// No "gpu" capability
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
		return incompatibleDeviceRequest{dev.req.Driver, dev.req.Capabilities}
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
