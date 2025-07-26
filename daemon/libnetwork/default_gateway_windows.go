package libnetwork

import (
	windriver "github.com/moby/moby/v2/daemon/libnetwork/drivers/windows"
	"github.com/moby/moby/v2/daemon/libnetwork/options"
	"github.com/moby/moby/v2/daemon/libnetwork/types"
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
