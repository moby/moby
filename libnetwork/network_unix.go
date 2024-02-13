//go:build !windows

package libnetwork

import "github.com/docker/docker/libnetwork/ipamapi"

// Stub implementations for DNS related functions

func (n *Network) startResolver() {
}

func defaultIpamForNetworkType(networkType string) string {
	return ipamapi.DefaultIPAM
}
