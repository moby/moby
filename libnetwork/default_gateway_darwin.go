package libnetwork

import "github.com/docker/docker/libnetwork/types"

const libnGWNetwork = ""

func getPlatformOption() EndpointOption {
	return nil
}

func (c *Controller) createGWNetwork() (*Network, error) {
	return nil, types.NotImplementedErrorf("default gateway functionality is not implemented on macOS")
}
