//go:build !windows

package libnetwork

import (
	"fmt"
	"strconv"
	"time"

	"github.com/moby/moby/v2/daemon/libnetwork/netlabel"
	"github.com/moby/moby/v2/daemon/libnetwork/osl"
	"github.com/moby/moby/v2/daemon/network"
)

type platformNetwork struct{} //nolint:nolintlint,unused // only populated on windows

// Stub implementations for DNS related functions

func (n *Network) startResolver() {
}

func deleteEpFromResolver(epName string, epIface *EndpointInterface, resolvers []*Resolver) error {
	return nil
}

func (n *Network) validatedAdvertiseAddrNMsgs() (*int, error) {
	nMsgsStr, ok := n.DriverOptions()[netlabel.AdvertiseAddrNMsgs]
	if !ok {
		return nil, nil
	}
	nMsgs, err := strconv.Atoi(nMsgsStr)
	if err != nil {
		return nil, fmt.Errorf("value for option "+netlabel.AdvertiseAddrNMsgs+" %q must be an integer", nMsgsStr)
	}
	if nMsgs < osl.AdvertiseAddrNMsgsMin || nMsgs > osl.AdvertiseAddrNMsgsMax {
		return nil, fmt.Errorf(netlabel.AdvertiseAddrNMsgs+" must be in the range %d to %d",
			osl.AdvertiseAddrNMsgsMin, osl.AdvertiseAddrNMsgsMax)
	}
	return &nMsgs, nil
}

func (n *Network) validatedAdvertiseAddrInterval() (*time.Duration, error) {
	intervalStr, ok := n.DriverOptions()[netlabel.AdvertiseAddrIntervalMs]
	if !ok {
		return nil, nil
	}
	msecs, err := strconv.Atoi(intervalStr)
	if err != nil {
		return nil, fmt.Errorf("value for option "+netlabel.AdvertiseAddrIntervalMs+" %q must be integer milliseconds", intervalStr)
	}
	interval := time.Duration(msecs) * time.Millisecond
	if interval < osl.AdvertiseAddrIntervalMin || interval > osl.AdvertiseAddrIntervalMax {
		return nil, fmt.Errorf(netlabel.AdvertiseAddrIntervalMs+" must be in the range %d to %d",
			osl.AdvertiseAddrIntervalMin/time.Millisecond, osl.AdvertiseAddrIntervalMax/time.Millisecond)
	}
	return &interval, nil
}

// IsPruneable returns true if n can be considered for removal as part of a
// "docker network prune" (or system prune). The caller must still check that the
// network should be removed. For example, it may have active endpoints.
func (n *Network) IsPruneable() bool {
	return !network.IsPredefined(n.Name())
}
