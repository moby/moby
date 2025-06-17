package cnmallocator

import (
	"strings"

	"github.com/docker/docker/libnetwork/driverapi"
	"github.com/docker/docker/libnetwork/drivers/overlay/overlayutils"
	"github.com/docker/docker/libnetwork/ipamapi"
	"github.com/docker/docker/pkg/plugingetter"
	"github.com/moby/swarmkit/v2/api"
	"github.com/moby/swarmkit/v2/manager/allocator/networkallocator"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type Provider struct {
	pg plugingetter.PluginGetter
}

var _ networkallocator.Provider = &Provider{}

// NewProvider returns a new cnmallocator provider.
func NewProvider(pg plugingetter.PluginGetter) *Provider {
	return &Provider{pg: pg}
}

// ValidateIPAMDriver implements networkallocator.NetworkProvider.
func (p *Provider) ValidateIPAMDriver(driver *api.Driver) error {
	if driver == nil {
		// It is ok to not specify the driver. We will choose
		// a default driver.
		return nil
	}

	if driver.Name == "" {
		return status.Errorf(codes.InvalidArgument, "driver name: if driver is specified name is required")
	}
	if strings.ToLower(driver.Name) == ipamapi.DefaultIPAM {
		return nil
	}
	return p.validatePluginDriver(driver, ipamapi.PluginEndpointType)
}

// ValidateIngressNetworkDriver implements networkallocator.NetworkProvider.
func (p *Provider) ValidateIngressNetworkDriver(driver *api.Driver) error {
	if driver != nil && driver.Name != "overlay" {
		return status.Errorf(codes.Unimplemented, "only overlay driver is currently supported for ingress network")
	}
	return p.ValidateNetworkDriver(driver)
}

// ValidateNetworkDriver implements networkallocator.NetworkProvider.
func (p *Provider) ValidateNetworkDriver(driver *api.Driver) error {
	if driver == nil {
		// It is ok to not specify the driver. We will choose
		// a default driver.
		return nil
	}

	if driver.Name == "" {
		return status.Errorf(codes.InvalidArgument, "driver name: if driver is specified name is required")
	}

	// First check against the known drivers
	if IsBuiltInDriver(driver.Name) {
		return nil
	}

	return p.validatePluginDriver(driver, driverapi.NetworkPluginEndpointType)
}

func (p *Provider) validatePluginDriver(driver *api.Driver, pluginType string) error {
	if p.pg == nil {
		return status.Errorf(codes.InvalidArgument, "plugin %s not supported", driver.Name)
	}

	plug, err := p.pg.Get(driver.Name, pluginType, plugingetter.Lookup)
	if err != nil {
		return status.Errorf(codes.InvalidArgument, "error during lookup of plugin %s", driver.Name)
	}

	if plug.IsV1() {
		return status.Errorf(codes.InvalidArgument, "legacy plugin %s of type %s is not supported in swarm mode", driver.Name, pluginType)
	}

	return nil
}

func (p *Provider) SetDefaultVXLANUDPPort(port uint32) error {
	return overlayutils.ConfigVXLANUDPPort(port)
}
