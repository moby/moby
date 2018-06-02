package network

import (
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/network"
)

// WithDriver sets the driver of the network
func WithDriver(driver string) func(*types.NetworkCreate) {
	return func(n *types.NetworkCreate) {
		n.Driver = driver
	}
}

// WithIPv6 Enables IPv6 on the network
func WithIPv6() func(*types.NetworkCreate) {
	return func(n *types.NetworkCreate) {
		n.EnableIPv6 = true
	}
}

// WithMacvlan sets the network as macvlan with the specified parent
func WithMacvlan(parent string) func(*types.NetworkCreate) {
	return func(n *types.NetworkCreate) {
		n.Driver = "macvlan"
		if parent != "" {
			n.Options = map[string]string{
				"parent": parent,
			}
		}
	}
}

// WithOption adds the specified key/value pair to network's options
func WithOption(key, value string) func(*types.NetworkCreate) {
	return func(n *types.NetworkCreate) {
		if n.Options == nil {
			n.Options = map[string]string{}
		}
		n.Options[key] = value
	}
}

// WithIPAM adds an IPAM with the specified Subnet and Gateway to the network
func WithIPAM(subnet, gateway string) func(*types.NetworkCreate) {
	return func(n *types.NetworkCreate) {
		if n.IPAM == nil {
			n.IPAM = &network.IPAM{}
		}

		n.IPAM.Config = append(n.IPAM.Config, network.IPAMConfig{
			Subnet:     subnet,
			Gateway:    gateway,
			AuxAddress: map[string]string{},
		})
	}
}
