package network

import (
	"net/netip"

	"github.com/moby/moby/api/types/network"
	"github.com/moby/moby/client"
)

// WithDriver sets the driver of the network
func WithDriver(driver string) func(*client.NetworkCreateOptions) {
	return func(n *client.NetworkCreateOptions) {
		n.Driver = driver
	}
}

// WithIPv4 enables/disables IPv4 on the network
func WithIPv4(enable bool) func(*client.NetworkCreateOptions) {
	return func(n *client.NetworkCreateOptions) {
		enableIPv4 := enable
		n.EnableIPv4 = &enableIPv4
	}
}

// WithIPv6 Enables IPv6 on the network
func WithIPv6() func(*client.NetworkCreateOptions) {
	return func(n *client.NetworkCreateOptions) {
		enableIPv6 := true
		n.EnableIPv6 = &enableIPv6
	}
}

// WithIPv4Disabled makes sure IPv4 is disabled on the network.
func WithIPv4Disabled() func(*client.NetworkCreateOptions) {
	return func(n *client.NetworkCreateOptions) {
		enable := false
		n.EnableIPv4 = &enable
	}
}

// WithIPv6Disabled makes sure IPv6 is disabled on the network.
func WithIPv6Disabled() func(*client.NetworkCreateOptions) {
	return func(n *client.NetworkCreateOptions) {
		enable := false
		n.EnableIPv6 = &enable
	}
}

// WithInternal enables Internal flag on the create network request
func WithInternal() func(*client.NetworkCreateOptions) {
	return func(n *client.NetworkCreateOptions) {
		n.Internal = true
	}
}

// WithConfigOnly sets the ConfigOnly flag in the create network request
func WithConfigOnly(co bool) func(*client.NetworkCreateOptions) {
	return func(n *client.NetworkCreateOptions) {
		n.ConfigOnly = co
	}
}

// WithConfigFrom sets the ConfigOnly flag in the create network request
func WithConfigFrom(name string) func(*client.NetworkCreateOptions) {
	return func(n *client.NetworkCreateOptions) {
		n.ConfigFrom = &network.ConfigReference{Network: name}
	}
}

// WithAttachable sets Attachable flag on the create network request
func WithAttachable() func(*client.NetworkCreateOptions) {
	return func(n *client.NetworkCreateOptions) {
		n.Attachable = true
	}
}

// WithScope sets the network scope.
func WithScope(s string) func(*client.NetworkCreateOptions) {
	return func(n *client.NetworkCreateOptions) {
		n.Scope = s
	}
}

// WithMacvlan sets the network as macvlan with the specified parent
func WithMacvlan(parent string) func(*client.NetworkCreateOptions) {
	return func(n *client.NetworkCreateOptions) {
		n.Driver = "macvlan"
		if parent != "" {
			n.Options = map[string]string{
				"parent": parent,
			}
		}
	}
}

// WithMacvlanPassthru sets the network as macvlan with the specified parent in passthru mode
func WithMacvlanPassthru(parent string) func(options *client.NetworkCreateOptions) {
	return func(n *client.NetworkCreateOptions) {
		n.Driver = "macvlan"
		n.Options = map[string]string{
			"macvlan_mode": "passthru",
		}
		if parent != "" {
			n.Options["parent"] = parent
		}
	}
}

// WithIPvlan sets the network as ipvlan with the specified parent and mode
func WithIPvlan(parent, mode string) func(*client.NetworkCreateOptions) {
	return func(n *client.NetworkCreateOptions) {
		n.Driver = "ipvlan"
		if n.Options == nil {
			n.Options = map[string]string{}
		}
		if parent != "" {
			n.Options["parent"] = parent
		}
		if mode != "" {
			n.Options["ipvlan_mode"] = mode
		}
	}
}

// WithOption adds the specified key/value pair to network's options
func WithOption(key, value string) func(*client.NetworkCreateOptions) {
	return func(n *client.NetworkCreateOptions) {
		if n.Options == nil {
			n.Options = map[string]string{}
		}
		n.Options[key] = value
	}
}

// WithIPAM adds an IPAM with the specified Subnet and Gateway to the network
func WithIPAM(subnet, gateway string) func(*client.NetworkCreateOptions) {
	return WithIPAMRange(subnet, "", gateway)
}

// WithIPAMRange adds an IPAM with the specified Subnet, IPRange and Gateway to the network
func WithIPAMRange(subnet, iprange, gateway string) func(*client.NetworkCreateOptions) {
	c := network.IPAMConfig{
		AuxAddress: map[string]netip.Addr{},
	}
	if subnet != "" {
		c.Subnet = netip.MustParsePrefix(subnet)
	}
	if iprange != "" {
		c.IPRange = netip.MustParsePrefix(iprange)
	}
	if gateway != "" {
		c.Gateway = netip.MustParseAddr(gateway)
	}
	return WithIPAMConfig(c)
}

// WithIPAMConfig adds the provided IPAM configurations to the network
func WithIPAMConfig(configs ...network.IPAMConfig) func(*client.NetworkCreateOptions) {
	return func(n *client.NetworkCreateOptions) {
		if n.IPAM == nil {
			n.IPAM = &network.IPAM{}
		}
		n.IPAM.Config = append(n.IPAM.Config, configs...)
	}
}
