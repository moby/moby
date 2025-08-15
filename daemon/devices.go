package daemon

import (
	"context"

	"github.com/containerd/log"
	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/system"
	"github.com/moby/moby/v2/daemon/config"
	"github.com/moby/moby/v2/daemon/internal/capabilities"
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
		// If no driver is explicitly requested, we iterate over the registered
		// drivers and attempt to match on capabilities.
		for driver, dd := range deviceDrivers {
			if selected := dd.capset.Match(req.Capabilities); selected != nil {
				log.G(context.TODO()).WithFields(log.Fields{
					"driver": driver,
					"capabilities": map[string]any{
						"requested": req.Capabilities,
						"selected":  selected,
					},
				}).Debug("Selecting device driver by capabilities")
				return dd.updateSpec(spec, &deviceInstance{req: req, selectedCaps: selected})
			}
		}
	} else if dd := deviceDrivers[req.Driver]; dd != nil {
		selected := dd.capset.Match(req.Capabilities)
		// If a driver is explicitly requested and registered, then we use the
		// specified driver, ignoring the capabilities.
		log.G(context.TODO()).WithFields(log.Fields{
			"driver": req.Driver,
			"capabilities": map[string]any{
				"requested": req.Capabilities,
				"selected":  selected,
			},
		}).Debug("Selecting device driver by driver name; possibly ignoring capabilities")
		return dd.updateSpec(spec, &deviceInstance{req: req, selectedCaps: selected})
	}

	return incompatibleDeviceRequest{req.Driver, req.Capabilities}
}
