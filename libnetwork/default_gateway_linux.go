package libnetwork

import (
	"fmt"
	"strconv"

	"github.com/docker/docker/libnetwork/drivers/bridge"
)

const libnGWNetwork = "docker_gwbridge"

func getPlatformOption() EndpointOption {
	return nil
}

func (c *Controller) createGWNetwork() (*Network, error) {
	n, err := c.NewNetwork("bridge", libnGWNetwork, "",
		NetworkOptionDriverOpts(map[string]string{
			bridge.BridgeName:         libnGWNetwork,
			bridge.EnableICC:          strconv.FormatBool(false),
			bridge.EnableIPMasquerade: strconv.FormatBool(true),
		}),
		NetworkOptionEnableIPv6(false),
	)
	if err != nil {
		return nil, fmt.Errorf("error creating external connectivity network: %v", err)
	}
	return n, err
}
