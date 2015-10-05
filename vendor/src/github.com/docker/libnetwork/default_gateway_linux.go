package libnetwork

import (
	"fmt"
	"strconv"

	"github.com/docker/libnetwork/drivers/bridge"
	"github.com/docker/libnetwork/netlabel"
	"github.com/docker/libnetwork/options"
)

func (c *controller) createGWNetwork() (Network, error) {
	netOption := map[string]string{
		bridge.BridgeName:         libnGWNetwork,
		bridge.EnableICC:          strconv.FormatBool(false),
		bridge.EnableIPMasquerade: strconv.FormatBool(true),
	}

	n, err := c.NewNetwork("bridge", libnGWNetwork,
		NetworkOptionGeneric(options.Generic{
			netlabel.GenericData: netOption,
			netlabel.EnableIPv6:  false,
		}))

	if err != nil {
		return nil, fmt.Errorf("error creating external connectivity network: %v", err)
	}
	return n, err
}
