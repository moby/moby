package daemon

import (
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/docker/docker/daemon/internal/capabilities"
	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/pkg/errors"
)

var errConflictCountDeviceIDs = errors.New("cannot set both Count and DeviceIDs on device request")

const (
	nvidiaContainerRuntimeHookExecutableName = "nvidia-container-runtime-hook"
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

	// No "gpu" capability
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

// countToDevices returns the list 0, 1, ... count-1 of deviceIDs.
func countToDevices(count int) string {
	devices := make([]string, count)
	for i := range devices {
		devices[i] = strconv.Itoa(i)
	}
	return strings.Join(devices, ",")
}
