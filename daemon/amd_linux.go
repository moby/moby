package daemon

import (
	"os/exec"
	"strconv"
	"strings"

	"github.com/docker/docker/daemon/internal/capabilities"
	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/pkg/errors"
)

var errConflictCountDeviceIDsAMD = errors.New("cannot set both Count and DeviceIDs on device request")

const amdContainerRuntime = "amd-container-runtime"

func init() {
	if _, err := exec.LookPath(nvidiaHook); err == nil {
		// Do not register AMD driver if Nvidia helper binary is present.
		return
	}

	if _, err := exec.LookPath(amdContainerRuntime); err != nil {
		// Do not register AMD driver if AMD container runtime is not present.
		return
	}

	capset := capabilities.Set{"gpu": struct{}{}, "amd": struct{}{}}
	amdDriver := &deviceDriver{
		capset:     capset,
		updateSpec: setAMDGPUs,
	}
	registerDeviceDriver("amd", amdDriver)
}

func setAMDGPUs(s *specs.Spec, dev *deviceInstance) error {
	req := dev.req
	if req.Count != 0 && len(req.DeviceIDs) > 0 {
		return errConflictCountDeviceIDsAMD
	}

	switch {
	case len(req.DeviceIDs) > 0:
		s.Process.Env = append(s.Process.Env, "AMD_VISIBLE_DEVICES="+strings.Join(req.DeviceIDs, ","))
	case req.Count > 0:
		s.Process.Env = append(s.Process.Env, "AMD_VISIBLE_DEVICES="+countToDevicesAMD(req.Count))
	case req.Count < 0:
		s.Process.Env = append(s.Process.Env, "AMD_VISIBLE_DEVICES=all")
	case req.Count == 0:
		s.Process.Env = append(s.Process.Env, "AMD_VISIBLE_DEVICES=void")
	}

	return nil
}

// countToDevicesAMD returns the list 0, 1, ... count-1 of deviceIDs.
func countToDevicesAMD(count int) string {
	devices := make([]string, count)
	for i := range devices {
		devices[i] = strconv.Itoa(i)
	}
	return strings.Join(devices, ",")
}
