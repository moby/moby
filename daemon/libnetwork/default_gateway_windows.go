package libnetwork

import (
	windriver "github.com/docker/docker/daemon/libnetwork/drivers/windows"
	"github.com/docker/docker/daemon/libnetwork/options"
	"github.com/docker/docker/daemon/libnetwork/types"
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
