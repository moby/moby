//go:build linux

package firewaller

import (
	"context"
	"fmt"
	"net/netip"
	"slices"

	"github.com/moby/moby/v2/daemon/libnetwork/types"
)

// StubFirewaller implements a Firewaller for unit tests. It just tracks what it's been asked for.
type StubFirewaller struct {
	Config
	Networks map[string]*StubFirewallerNetwork
}

func NewStubFirewaller(config Config) *StubFirewaller {
	return &StubFirewaller{
		Config: config,
		// A real Firewaller shouldn't hold on to its own networks, the bridge driver is doing that.
		// But, for unit tests cross-checking the driver, this is useful.
		Networks: make(map[string]*StubFirewallerNetwork),
	}
}

func (fw *StubFirewaller) NewNetwork(_ context.Context, nc NetworkConfig) (Network, error) {
	if _, ok := fw.Networks[nc.IfName]; ok {
		return nil, fmt.Errorf("StubFirewaller: network with IfName %q already exists", nc.IfName)
	}
	nw := &StubFirewallerNetwork{
		NetworkConfig: nc,
		Endpoints:     map[stubEndpoint]struct{}{},
		parent:        fw,
	}
	fw.Networks[nc.IfName] = nw
	return nw, nil
}

type stubFirewallerLink struct {
	parentIP netip.Addr
	childIP  netip.Addr
	ports    []types.TransportPort
}

type stubEndpoint struct {
	addr4 netip.Addr
	addr6 netip.Addr
}

type StubFirewallerNetwork struct {
	NetworkConfig
	Deleted   bool
	Endpoints map[stubEndpoint]struct{}
	Ports     []types.PortBinding
	Links     []stubFirewallerLink

	parent *StubFirewaller
}

func (nw *StubFirewallerNetwork) ReapplyNetworkLevelRules(_ context.Context) error {
	return nil
}

func (nw *StubFirewallerNetwork) DelNetworkLevelRules(_ context.Context) error {
	if _, ok := nw.parent.Networks[nw.IfName]; !ok {
		return fmt.Errorf("StubFirewaller: DelNetworkLevelRules: network '%s' does not exist", nw.IfName)
	}
	// A real firewaller may not report an error if network rules are deleted without
	// per-endpoint/port/link rules being deleted first, the bridge driver is responsible
	// for tracking all that - and it may not be an error if, for example, the driver
	// knows the rules have already been deleted by a firewalld reload. So, this may be
	// wrong for some tests but, for now, cross-check the deletion.
	if len(nw.Endpoints) != 0 {
		return fmt.Errorf("StubFirewaller: DelNetworkLevelRules: network '%s' still has endpoints", nw.IfName)
	}
	if len(nw.Ports) != 0 {
		return fmt.Errorf("StubFirewaller: DelNetworkLevelRules: network '%s' still has ports", nw.IfName)
	}
	if len(nw.Links) != 0 {
		return fmt.Errorf("StubFirewaller: DelNetworkLevelRules: network '%s' still has links", nw.IfName)
	}
	delete(nw.parent.Networks, nw.IfName)
	return nil
}

func (nw *StubFirewallerNetwork) AddEndpoint(_ context.Context, epIPv4, epIPv6 netip.Addr) error {
	ep := stubEndpoint{addr4: epIPv4, addr6: epIPv6}
	if _, ok := nw.Endpoints[ep]; ok {
		return fmt.Errorf("StubFirewaller: AddEndpoint: %s/%s already exists", epIPv4, epIPv6)
	}
	nw.Endpoints[ep] = struct{}{}
	return nil
}

func (nw *StubFirewallerNetwork) DelEndpoint(_ context.Context, epIPv4, epIPv6 netip.Addr) error {
	ep := stubEndpoint{addr4: epIPv4, addr6: epIPv6}
	if _, ok := nw.Endpoints[ep]; !ok {
		return fmt.Errorf("StubFirewaller: DelEndpoint: %s/%s does not exist", epIPv4, epIPv6)
	}
	delete(nw.Endpoints, ep)
	return nil
}

func (nw *StubFirewallerNetwork) AddPorts(_ context.Context, pbs []types.PortBinding) error {
	for _, pb := range pbs {
		if nw.PortExists(pb) {
			return nil
		}
		nw.Ports = append(nw.Ports, pb.Copy())
	}
	return nil
}

func (nw *StubFirewallerNetwork) DelPorts(_ context.Context, pbs []types.PortBinding) error {
	for _, pb := range pbs {
		nw.Ports = slices.DeleteFunc(nw.Ports, pb.Equal)
	}
	return nil
}

func (nw *StubFirewallerNetwork) AddLink(_ context.Context, parentIP, childIP netip.Addr, ports []types.TransportPort) error {
	if nw.LinkExists(parentIP, childIP, ports) {
		return nil
	}
	nw.Links = append(nw.Links, stubFirewallerLink{
		parentIP: parentIP,
		childIP:  childIP,
		ports:    slices.Clone(ports),
	})
	return nil
}

func (nw *StubFirewallerNetwork) DelLink(_ context.Context, parentIP, childIP netip.Addr, ports []types.TransportPort) {
	nw.Links = slices.DeleteFunc(nw.Links, func(l stubFirewallerLink) bool {
		return matchLink(l, parentIP, childIP, ports)
	})
}

func (nw *StubFirewallerNetwork) PortExists(pb types.PortBinding) bool {
	return slices.ContainsFunc(nw.Ports, pb.Equal)
}

func (nw *StubFirewallerNetwork) LinkExists(parentIP, childIP netip.Addr, ports []types.TransportPort) bool {
	return slices.ContainsFunc(nw.Links, func(l stubFirewallerLink) bool {
		return matchLink(l, parentIP, childIP, ports)
	})
}

func matchLink(l stubFirewallerLink, parentIP, childIP netip.Addr, ports []types.TransportPort) bool {
	if len(l.ports) != len(ports) {
		return false
	}
	for i, p := range l.ports {
		if p != ports[i] {
			return false
		}
	}
	return (l.parentIP == parentIP) && (l.childIP == childIP)
}
