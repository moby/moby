package libnetwork

import (
	"context"
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

	opts := map[string]string{
		bridge.BridgeName:         libnGWNetwork,
		bridge.EnableICC:          strconv.FormatBool(false),
		bridge.EnableIPMasquerade: strconv.FormatBool(true),
	}

	// Merge daemon's default network opts, without overriding explicitly set keys
	ApplyDefaultDriverOpts(ctx, opts, "bridge", libnGWNetwork, c.cfg.DefaultNetworkOpts)

	n, err := c.NewNetwork(ctx, "bridge", libnGWNetwork, "",
		NetworkOptionDriverOpts(opts),
		NetworkOptionEnableIPv4(true),
		NetworkOptionEnableIPv6(false),
	)
	if err != nil {
		return nil, err
	}
	return n, nil
}
