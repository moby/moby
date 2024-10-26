package daemon

import (
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/containerd/containerd/contrib/nvidia"
	"github.com/docker/docker/daemon/internal/capabilities"
	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/pkg/errors"
)

// TODO: nvidia should not be hard-coded, and should be a device plugin instead on the daemon object.
// TODO: add list of device capabilities in daemon/node info

var errConflictCountDeviceIDs = errors.New("cannot set both Count and DeviceIDs on device request")

const nvidiaHook = "nvidia-container-runtime-hook"

// These are NVIDIA-specific capabilities stolen from github.com/containerd/containerd/contrib/nvidia.allCaps
var allNvidiaCaps = map[nvidia.Capability]struct{}{
	nvidia.Compute:  {},
	nvidia.Compat32: {},
	nvidia.Graphics: {},
	nvidia.Utility:  {},
	nvidia.Video:    {},
	nvidia.Display:  {},
}

func init() {
	if _, err := exec.LookPath(nvidiaHook); err != nil {
		// do not register Nvidia driver if helper binary is not present.
		return
	}
	capset := capabilities.Set{"gpu": struct{}{}, "nvidia": struct{}{}}
	nvidiaDriver := &deviceDriver{
		capset:     capset,
		updateSpec: setNvidiaGPUs,
	}
	for c := range allNvidiaCaps {
		nvidiaDriver.capset[string(c)] = struct{}{}
	}
	registerDeviceDriver("nvidia", nvidiaDriver)
}

func setNvidiaGPUs(s *specs.Spec, dev *deviceInstance) error {
	req := dev.req
	if req.Count != 0 && len(req.DeviceIDs) > 0 {
		return errConflictCountDeviceIDs
	}

	switch {
	case len(req.DeviceIDs) > 0:
		s.Process.Env = append(s.Process.Env, "NVIDIA_VISIBLE_DEVICES="+strings.Join(req.DeviceIDs, ","))
	case req.Count > 0:
		s.Process.Env = append(s.Process.Env, "NVIDIA_VISIBLE_DEVICES="+countToDevices(req.Count))
	case req.Count < 0:
		s.Process.Env = append(s.Process.Env, "NVIDIA_VISIBLE_DEVICES=all")
	case req.Count == 0:
		s.Process.Env = append(s.Process.Env, "NVIDIA_VISIBLE_DEVICES=void")
	}

	var nvidiaCaps []string
	// req.Capabilities contains device capabilities, some but not all are NVIDIA driver capabilities.
	for _, c := range dev.selectedCaps {
		nvcap := nvidia.Capability(c)
		if _, isNvidiaCap := allNvidiaCaps[nvcap]; isNvidiaCap {
			nvidiaCaps = append(nvidiaCaps, c)
			continue
		}
		// TODO: nvidia.WithRequiredCUDAVersion
		// for now we let the prestart hook verify cuda versions but errors are not pretty.
	}

	if nvidiaCaps != nil {
		s.Process.Env = append(s.Process.Env, "NVIDIA_DRIVER_CAPABILITIES="+strings.Join(nvidiaCaps, ","))
	}

	path, err := exec.LookPath(nvidiaHook)
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
			nvidiaHook,
			"prestart",
		},
		Env: os.Environ(),
	})

	return nil
}

// countToDevices returns the list 0, 1, ... count-1 of deviceIDs.
func countToDevices(count int) string {
	devices := make([]string, count)
	for i := range devices {
		devices[i] = strconv.Itoa(i)
	}
	return strings.Join(devices, ",")
}
