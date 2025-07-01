package libnetwork

import (
	"github.com/docker/docker/daemon/libnetwork/options"
	"github.com/docker/docker/daemon/libnetwork/types"
	windriver "github.com/docker/docker/libnetwork/drivers/windows"
)

const libnGWNetwork = "nat"

func getPlatformOption() EndpointOption {
	epOption := options.Generic{
		windriver.DisableICC: true,
		windriver.DisableDNS: true,
	}
	return EndpointOptionGeneric(epOption)
}

func (c *Controller) createGWNetwork() (*Network, error) {
	return nil, types.NotImplementedErrorf("default gateway functionality is not implemented in windows")
}
