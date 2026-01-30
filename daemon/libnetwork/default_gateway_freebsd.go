package libnetwork

import (
	"context"

	"github.com/moby/moby/v2/daemon/libnetwork/types"
)

const libnGWNetwork = "docker_gwbridge"

func getPlatformOption() EndpointOption {
	return nil
}

func (c *Controller) createGWNetwork(context.Context) (*Network, error) {
	return nil, types.NotImplementedErrorf("default gateway functionality is not implemented in freebsd")
}
