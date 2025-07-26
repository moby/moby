package daemon

import (
	"context"

	"github.com/docker/docker/daemon/config"
	"github.com/docker/docker/daemon/internal/capabilities"
	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/system"
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
	// If the requested driver is registered we update the spec using this
	// driver.
	if dd := deviceDrivers[req.Driver]; dd != nil {
		return dd.updateSpec(spec, &deviceInstance{req: req})
	}

	// If no matching friver can be found, we fallback to requesting based on
	// capabilities accross all drivers.
	for _, dd := range deviceDrivers {
		if selected := dd.capset.Match(req.Capabilities); selected != nil {
			return dd.updateSpec(spec, &deviceInstance{req: req, selectedCaps: selected})
		}
	}
	return incompatibleDeviceRequest{req.Driver, req.Capabilities}
}
