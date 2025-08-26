package cnmallocator

import (
	"context"
	"fmt"
	"net/netip"
	"strconv"
	"strings"

	"github.com/containerd/log"
	"github.com/moby/moby/v2/daemon/libnetwork/ipamapi"
	"github.com/moby/moby/v2/daemon/libnetwork/ipams"
	"github.com/moby/moby/v2/daemon/libnetwork/ipamutils"
	"github.com/moby/swarmkit/v2/manager/allocator/networkallocator"
)

func initIPAMDrivers(r ipamapi.Registerer, netConfig *networkallocator.Config) error {
	var addressPool []*ipamutils.NetworkToSplit
	var str strings.Builder
	str.WriteString("Subnetlist - ")
	// Extract defaultAddrPool param info and construct ipamutils.NetworkToSplit
	// from the info. We will be using it to call Libnetwork API
	// We also need to log new address pool info whenever swarm init
	// happens with default address pool option
	if netConfig != nil {
		for _, p := range netConfig.DefaultAddrPool {
			base, err := netip.ParsePrefix(p)
			if err != nil {
				return fmt.Errorf("invalid prefix %q: %w", p, err)
			}
			addressPool = append(addressPool, &ipamutils.NetworkToSplit{
				Base: base,
				Size: int(netConfig.SubnetSize),
			})
			str.WriteString(p + ",")
		}
		str.WriteString(": Size ")
		str.WriteString(strconv.Itoa(int(netConfig.SubnetSize)))

	}

	if len(addressPool) > 0 {
		log.G(context.TODO()).Info("Swarm initialized global default address pool to: " + str.String())
	}

	return ipams.Register(r, nil, nil, addressPool)
}
