package libnetwork

import (
	"context"
	"fmt"
	"strconv"

	"github.com/moby/moby/v2/daemon/internal/otelutil"
	"github.com/moby/moby/v2/daemon/libnetwork/drivers/bridge"
	"go.opentelemetry.io/otel/baggage"
)

const libnGWNetwork = "docker_gwbridge"

func getPlatformOption() EndpointOption {
	return nil
}

func (c *Controller) createGWNetwork() (*Network, error) {
	ctx := baggage.ContextWithBaggage(context.TODO(), otelutil.MustNewBaggage(
		otelutil.MustNewMemberRaw(otelutil.TriggerKey, "libnetwork.Controller.createGWNetwork"),
	))

	drvOpts := map[string]string{
		bridge.BridgeName:         libnGWNetwork,
		bridge.EnableICC:          strconv.FormatBool(false),
		bridge.EnableIPMasquerade: strconv.FormatBool(true),
	}
	for k, v := range c.cfg.DefaultNetworkOpts["bridge"] {
		if _, ok := drvOpts[k]; !ok {
			drvOpts[k] = v
		}
	}

	n, err := c.NewNetwork(ctx, "bridge", libnGWNetwork, "",
		NetworkOptionDriverOpts(drvOpts),
		NetworkOptionEnableIPv4(true),
		NetworkOptionEnableIPv6(false),
	)
	if err != nil {
		return nil, fmt.Errorf("error creating external connectivity network: %v", err)
	}
	return n, err
}
