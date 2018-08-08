package nvidia // import "github.com/docker/docker/daemon/gpu/nvidia"

import (
	"context"
	"fmt"
	"strings"

	"github.com/containerd/containerd/contrib/nvidia"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/daemon/gpu"
	"github.com/docker/docker/errdefs"
	"github.com/docker/docker/libcontainerd"
	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

func init() {
	gpu.Register("nvidia", handler)
}

func lookupEnv(env []string, key string) (string, bool) {
	for _, s := range env {
		p := strings.SplitN(s, "=", 2)
		if len(p) != 2 {
			logrus.Warnf("invalid environment variable: %s", s)
			return "", false
		}
		if p[0] == key {
			return p[1], true
		}
	}

	return "", false
}

func setDevices(opts []nvidia.Opts, spec *specs.Spec, set container.GPUSet) []nvidia.Opts {
	// We currently interpret this case as: add all GPUs.
	if len(set.GPUs) == 0 {
		opts = append(opts, nvidia.WithAllDevices)
	} else {
		for _, gpu := range set.GPUs {
			opts = append(opts, nvidia.WithDeviceUUIDs(gpu.ID))
		}
	}

	return opts
}

func setDriverCapabilities(opts []nvidia.Opts, spec *specs.Spec, set container.GPUSet) []nvidia.Opts {
	// CLI options have precedence over environment variables from the image.
	// If both are unset, default to all driver capabilities.
	caps, exists := set.Options["capabilities"]
	if !exists {
		caps, exists = lookupEnv(spec.Process.Env, "NVIDIA_DRIVER_CAPABILITIES")
		if !exists {
			caps = "all"
		}
	}

	if caps == "all" {
		opts = append(opts, nvidia.WithAllCapabilities)
	} else {
		for _, cap := range strings.Split(caps, ",") {
			opts = append(opts, nvidia.WithCapabilities(nvidia.Capability(cap)))
		}
	}

	return opts
}

func parseCudaVersion(cudaVersion string) (vmaj, vmin, vpatch int) {
	if _, err := fmt.Sscanf(cudaVersion, "%d.%d.%d\n", &vmaj, &vmin, &vpatch); err != nil {
		vpatch = 0
		if _, err := fmt.Sscanf(cudaVersion, "%d.%d\n", &vmaj, &vmin); err != nil {
			vmin = 0
			if _, err := fmt.Sscanf(cudaVersion, "%d\n", &vmaj); err != nil {
				// Invalid version, return 0.0.0
				vmaj = 0
			}
		}
	}

	return
}

func setCUDAVersion(opts []nvidia.Opts, spec *specs.Spec, _ container.GPUSet) []nvidia.Opts {
	// It's unlikely that this option needs to be exposed in the CLI.
	version, exists := lookupEnv(spec.Process.Env, "CUDA_VERSION")
	if !exists {
		return opts
	}

	vmaj, vmin, _ := parseCudaVersion(version)
	if vmaj > 0 {
		opts = append(opts, nvidia.WithRequiredCUDAVersion(vmaj, vmin))
	}

	return opts
}

func handler(spec *specs.Spec, set container.GPUSet) error {
	if set.Vendor != "nvidia" {
		return errdefs.NotFound(errors.Errorf("vendor mismatch: received %s, expected nvidia", set.Vendor))
	}

	var opts []nvidia.Opts
	// By default, the "nvidia" package will look for the the "containerd"
	// binary. With dockerd, the binary name is "docker-containerd" instead.
	opts = append(opts, nvidia.WithLookupOCIHookPath(libcontainerd.BinaryName))

	opts = setDevices(opts, spec, set)
	opts = setDriverCapabilities(opts, spec, set)
	opts = setCUDAVersion(opts, spec, set)

	return nvidia.WithGPUs(opts...)(context.TODO(), nil, nil, spec)
}
