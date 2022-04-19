package portmapper

import (
	"net"
	"sync"

	"github.com/docker/docker/libnetwork/firewallapi"
	"github.com/docker/docker/libnetwork/portallocator"
)

// PortMapper manages the network address translation
type PortMapper struct {
	bridgeName string

	// udp:ip:port
	currentMappings map[string]*mapping
	lock            sync.Mutex

	proxyPath string

	Allocator *portallocator.PortAllocator
	chain     firewallapi.FirewallChain
	table     firewallapi.FirewallTable
}

// SetIptablesChain sets the specified chain into portmapper
func (pm *PortMapper) SetFirewallTablesChain(c firewallapi.FirewallChain, bridgeName string, table firewallapi.FirewallTable) {
	pm.chain = c
	pm.bridgeName = bridgeName
	pm.table = table
}

// AppendForwardingTableEntry adds a port mapping to the forwarding table
func (pm *PortMapper) AppendForwardingTableEntry(proto string, sourceIP net.IP, sourcePort int, containerIP string, containerPort int) error {
	if pm.table == nil {
		return nil
	}
	return pm.forward(firewallapi.Action(pm.table.GetAppendAction()), proto, sourceIP, sourcePort, containerIP, containerPort)
}

// DeleteForwardingTableEntry removes a port mapping from the forwarding table
func (pm *PortMapper) DeleteForwardingTableEntry(proto string, sourceIP net.IP, sourcePort int, containerIP string, containerPort int) error {
	if pm.table == nil {
		return nil
	}
	return pm.forward(firewallapi.Action(pm.table.GetDeleteAction()), proto, sourceIP, sourcePort, containerIP, containerPort)
}

func (pm *PortMapper) forward(action firewallapi.Action, proto string, sourceIP net.IP, sourcePort int, containerIP string, containerPort int) error {
	if pm.chain == nil {
		return nil
	}
	return pm.chain.Forward(action, sourceIP, sourcePort, proto, containerIP, containerPort, pm.bridgeName)
}
