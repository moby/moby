package daemon

import (
	"os/exec"
	"strconv"

	"github.com/containerd/containerd/contrib/nvidia"
	"github.com/docker/docker/pkg/capabilities"
	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/pkg/errors"
)

// TODO: nvidia should not be hard-coded, and should be a device plugin instead on the daemon object.
// TODO: add list of device capabilities in daemon/node info

var errConflictCountDeviceIDs = errors.New("cannot set both Count and DeviceIDs on device request")

// stolen from github.com/containerd/containerd/contrib/nvidia
const nvidiaCLI = "nvidia-container-cli"

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
	if _, err := exec.LookPath(nvidiaCLI); err != nil {
		// do not register Nvidia driver if helper binary is not present.
		return
	}
	capset := capabilities.Set{"gpu": struct{}{}, "nvidia": struct{}{}}
	nvidiaDriver := &deviceDriver{
		capset:     capset,
		updateSpec: setNvidiaGPUs,
	}
	for c := range capset {
		nvidiaDriver.capset[c] = struct{}{}
	}
	registerDeviceDriver("nvidia", nvidiaDriver)
}

func setNvidiaGPUs(s *specs.Spec, dev *deviceInstance) error {
	var opts []nvidia.Opts

	req := dev.req
	if req.Count != 0 && len(req.DeviceIDs) > 0 {
		return errConflictCountDeviceIDs
	}

	if len(req.DeviceIDs) > 0 {
		var ids []int
		var uuids []string
		for _, devID := range req.DeviceIDs {
			id, err := strconv.Atoi(devID)
			if err == nil {
				ids = append(ids, id)
				continue
			}
			// if not an integer, then assume UUID.
			uuids = append(uuids, devID)
		}
		if len(ids) > 0 {
			opts = append(opts, nvidia.WithDevices(ids...))
		}
		if len(uuids) > 0 {
			opts = append(opts, nvidia.WithDeviceUUIDs(uuids...))
		}
	}

	if req.Count < 0 {
		opts = append(opts, nvidia.WithAllDevices)
	} else if req.Count > 0 {
		opts = append(opts, nvidia.WithDevices(countToDevices(req.Count)...))
	}

	var nvidiaCaps []nvidia.Capability
	// req.Capabilities contains device capabilities, some but not all are NVIDIA driver capabilities.
	for _, c := range dev.selectedCaps {
		nvcap := nvidia.Capability(c)
		if _, isNvidiaCap := allNvidiaCaps[nvcap]; isNvidiaCap {
			nvidiaCaps = append(nvidiaCaps, nvcap)
			continue
		}
		// TODO: nvidia.WithRequiredCUDAVersion
		// for now we let the prestart hook verify cuda versions but errors are not pretty.
	}

	if nvidiaCaps != nil {
		opts = append(opts, nvidia.WithCapabilities(nvidiaCaps...))
	}

	return nvidia.WithGPUs(opts...)(nil, nil, nil, s)
}

// countToDevices returns the list 0, 1, ... count-1 of deviceIDs.
func countToDevices(count int) []int {
	devices := make([]int, count)
	for i := range devices {
		devices[i] = i
	}
	return devices
}
