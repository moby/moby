package gpu // import "github.com/docker/docker/daemon/gpu"

import (
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/errdefs"
	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/pkg/errors"
)

var (
	vendors = make(map[string]SpecHandler)
)

// SpecHandler modifies an OCI runtime spec to enable access to GPU devices.
type SpecHandler func(spec *specs.Spec, set container.GPUSet) error

// Register registers a SpecHandler for a hardware vendor.
func Register(vendor string, handler SpecHandler) error {
	if _, exists := vendors[vendor]; exists {
		return errdefs.AlreadyExists(errors.Errorf("GPU vendor already registered %s", vendor))
	}
	vendors[vendor] = handler

	return nil
}

// HandleSpec dispatches to the right pre-registered SpecHandler based on the
// vendor name.
func HandleSpec(spec *specs.Spec, set container.GPUSet) error {
	handler, exists := vendors[set.Vendor]
	if !exists {
		return errdefs.System(errors.Errorf("unsupported GPU vendor: %s", set.Vendor))
	}

	return handler(spec, set)
}
