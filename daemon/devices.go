package daemon // import "github.com/docker/docker/daemon"

import (
	"context"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/system"
	"github.com/docker/docker/daemon/config"
	"github.com/docker/docker/daemon/internal/capabilities"
	"github.com/opencontainers/runtime-spec/specs-go"
)

var deviceDrivers = map[string]*deviceDriver{}

type deviceListing struct {
	Devices  []system.DeviceInfo
	Warnings []string
}

type deviceDriver struct {
	capset     capabilities.Set
	updateSpec func(*specs.Spec, *deviceInstance) error

	// ListDevices returns a list of discoverable devices provided by this
	// driver, any warnings encountered during the discovery, and an error if
	// the overall listing operation failed.
	// Can be nil if the driver does not provide a device listing.
	ListDevices func(ctx context.Context, cfg *config.Config) (deviceListing, error)
}

type deviceInstance struct {
	req          container.DeviceRequest
	selectedCaps []string
}

func registerDeviceDriver(name string, d *deviceDriver) {
	deviceDrivers[name] = d
}

func (daemon *Daemon) handleDevice(req container.DeviceRequest, spec *specs.Spec) error {
	if req.Driver == "" {
		for _, dd := range deviceDrivers {
			if selected := dd.capset.Match(req.Capabilities); selected != nil {
				return dd.updateSpec(spec, &deviceInstance{req: req, selectedCaps: selected})
			}
		}
	} else if dd := deviceDrivers[req.Driver]; dd != nil {
		// We add a special case for the CDI driver here as the cdi driver does
		// not distinguish between capabilities.
		// Furthermore, the "OR" and "AND" matching logic for the capability
		// sets requires that a dummy capability be specified when constructing a
		// DeviceRequest.
		// This workaround can be removed once these device driver are
		// refactored to be plugins, with each driver implementing its own
		// matching logic, for example.
		if req.Driver == "cdi" {
			return dd.updateSpec(spec, &deviceInstance{req: req})
		}
		if selected := dd.capset.Match(req.Capabilities); selected != nil {
			return dd.updateSpec(spec, &deviceInstance{req: req, selectedCaps: selected})
		}
	}
	return incompatibleDeviceRequest{req.Driver, req.Capabilities}
}
