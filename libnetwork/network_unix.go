//go:build !windows

package libnetwork

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/docker/docker/libnetwork/netlabel"
	"github.com/docker/docker/libnetwork/osl"

	"github.com/docker/docker/libnetwork/ipams/defaultipam"
)

type platformNetwork struct{} //nolint:nolintlint,unused // only populated on windows

// Stub implementations for DNS related functions

func (n *Network) startResolver() {
}

func addEpToResolver(
	ctx context.Context,
	netName, epName string,
	config *containerConfig,
	epIface *EndpointInterface,
	resolvers []*Resolver,
) error {
	return nil
}

func deleteEpFromResolver(epName string, epIface *EndpointInterface, resolvers []*Resolver) error {
	return nil
}

func defaultIpamForNetworkType(networkType string) string {
	return defaultipam.DriverName
}

func (n *Network) validateAdvertiseAddrNMsgs() (*int, error) {
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

func (n *Network) validateAdvertiseAddrInterval() (*time.Duration, error) {
	intervalStr, ok := n.DriverOptions()[netlabel.AdvertiseAddrInterval]
	if !ok {
		return nil, nil
	}
	interval, err := time.ParseDuration(intervalStr)
	if err != nil {
		return nil, fmt.Errorf("value for option "+netlabel.AdvertiseAddrInterval+" %q must be parseable by time.ParseDuration()", intervalStr)
	}
	if interval < osl.AdvertiseAddrIntervalMin || interval > osl.AdvertiseAddrIntervalMax {
		return nil, fmt.Errorf(netlabel.AdvertiseAddrInterval+" must be in the range %v to %v",
			osl.AdvertiseAddrIntervalMin, osl.AdvertiseAddrIntervalMax)
	}
	return &interval, nil
}
