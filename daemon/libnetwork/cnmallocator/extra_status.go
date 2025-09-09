package cnmallocator

import (
	"context"
	"fmt"
	"net/netip"

	networktypes "github.com/moby/moby/api/types/network"
	"github.com/moby/moby/v2/daemon/cluster/convert/netextra"
	"github.com/moby/moby/v2/daemon/libnetwork/ipamapi"
	"github.com/moby/swarmkit/v2/api"
	"github.com/moby/swarmkit/v2/manager/allocator/networkallocator"
)

var _ networkallocator.OnGetNetworker = &cnmNetworkAllocator{}

// OnGetNetwork augments Swarm networks with operational status.
func (na *cnmNetworkAllocator) OnGetNetwork(ctx context.Context, swarmnet *api.Network, typeurl string, appdata []byte) error {
	opts, err := netextra.OptionsFrom(typeurl, appdata)
	if err != nil {
		return fmt.Errorf("cnmallocator: bad appdata provided to OnGetNetwork: %w", err)
	}
	if !opts.WithIPAMStatus {
		return nil
	}

	n := na.getNetwork(swarmnet.ID)
	if n == nil {
		return fmt.Errorf("cnmallocator: network %s not found", swarmnet.ID)
	}

	ipamdriver, _, _, err := na.resolveIPAM(swarmnet)
	if err != nil {
		return fmt.Errorf("cnmallocator: failed to resolve IPAM driver for network %s: %w", swarmnet.ID, err)
	}

	ipam, ok := ipamdriver.(ipamapi.PoolStatuser)
	if !ok {
		// IPAM driver does not support reporting operational status.
		return nil
	}

	status := networktypes.Status{
		IPAM: networktypes.IPAMStatus{
			Subnets: make(map[netip.Prefix]networktypes.SubnetStatus),
		},
	}
	for subnet, poolID := range n.pools {
		pstat, err := ipam.PoolStatus(poolID)
		if err != nil {
			return fmt.Errorf("cnmallocator: failed to get pool status for network %s: %w", swarmnet.ID, err)
		}
		status.IPAM.Subnets[subnet] = networktypes.SubnetStatus{
			IPsInUse:            pstat.IPsInUse,
			DynamicIPsAvailable: pstat.DynamicIPsAvailable,
		}
	}

	extra, err := netextra.MarshalStatus(&status)
	if err != nil {
		return fmt.Errorf("cnmallocator: failed to marshal network status for network %s: %w", swarmnet.ID, err)
	}
	swarmnet.Extra = extra

	return nil
}
