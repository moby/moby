//go:build !windows

package libnetwork

import (
	"context"

	"github.com/docker/docker/libnetwork/ipamapi"
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
	return ipamapi.DefaultIPAM
}
