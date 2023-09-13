package portmapper

import (
	"net"
	"sync"

	"github.com/docker/docker/libnetwork/portallocator"
)

// PortMapper manages the network address translation
type PortMapper struct {
	bridgeName string

	// udp:ip:port
	currentMappings map[string]*mapping
	lock            sync.Mutex

	proxyPath string

	allocator *portallocator.PortAllocator
}

// AppendForwardingTableEntry adds a port mapping to the forwarding table
func (pm *PortMapper) AppendForwardingTableEntry(proto string, sourceIP net.IP, sourcePort int, containerIP string, containerPort int) error {
	return nil
}

// DeleteForwardingTableEntry removes a port mapping from the forwarding table
func (pm *PortMapper) DeleteForwardingTableEntry(proto string, sourceIP net.IP, sourcePort int, containerIP string, containerPort int) error {
	return nil
}

// checkIP checks if IP is valid and matching to chain version
func (pm *PortMapper) checkIP(ip net.IP) bool {
	// no IPv6 for port mapper on windows -> only IPv4 valid
	return ip.To4() != nil
}
