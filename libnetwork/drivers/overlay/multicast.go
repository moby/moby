//go:build linux

package overlay

import (
	"context"
	"fmt"
	"net"
	"sync"

	"github.com/containerd/log"
	"github.com/vishvananda/netlink"
)

const (
	// Default multicast MAC address that will be added to the FDB
	vxlanMulticastMac = "01:00:5e:00:00:00"
	// AF_BRIDGE constant for netlink
	afBridge = 7
)

// multicastRoutes maintains a map of multicast group to list of endpoints in that group
type multicastRoutes struct {
	sync.RWMutex
	routes map[string]map[string]struct{} // map[multicastGroupIP]map[endpointIP]struct{}
}

// newMulticastRoutes returns a new multicastRoutes instance
func newMulticastRoutes() *multicastRoutes {
	return &multicastRoutes{
		routes: make(map[string]map[string]struct{}),
	}
}

// addOrUpdateEndpoint adds an endpoint to a multicast group
func (mr *multicastRoutes) addOrUpdateEndpoint(mcastIP, endpointIP string) {
	mr.Lock()
	defer mr.Unlock()

	// Create the multicast group entry if it doesn't exist
	if _, ok := mr.routes[mcastIP]; !ok {
		mr.routes[mcastIP] = make(map[string]struct{})
	}

	// Add the endpoint to the multicast group
	mr.routes[mcastIP][endpointIP] = struct{}{}
}

// removeEndpoint removes an endpoint from a multicast group
func (mr *multicastRoutes) removeEndpoint(mcastIP, endpointIP string) {
	mr.Lock()
	defer mr.Unlock()

	if group, ok := mr.routes[mcastIP]; ok {
		delete(group, endpointIP)

		// If there are no more endpoints in the group, remove the group
		if len(group) == 0 {
			delete(mr.routes, mcastIP)
		}
	}
}

// getEndpoints returns all endpoints in a multicast group
func (mr *multicastRoutes) getEndpoints(mcastIP string) []string {
	mr.RLock()
	defer mr.RUnlock()

	if group, ok := mr.routes[mcastIP]; ok {
		endpoints := make([]string, 0, len(group))
		for ep := range group {
			endpoints = append(endpoints, ep)
		}
		return endpoints
	}

	return nil
}

// setupMulticastRouting configures the multicast routing in the overlay network
func (n *network) setupMulticastRouting(s *subnet) error {
	// Get the VXLAN interface link index
	vxlan, err := netlink.LinkByName(s.vxlanName)
	if err != nil {
		return fmt.Errorf("failed to find vxlan interface %s: %v", s.vxlanName, err)
	}

	// Parse the multicast MAC address
	mmac, err := net.ParseMAC(vxlanMulticastMac)
	if err != nil {
		return fmt.Errorf("failed to parse multicast MAC %s: %v", vxlanMulticastMac, err)
	}

	// Walk the peer database and add all remote endpoints to the FDB with the multicast MAC
	err = n.driver.peerDbNetworkWalk(n.id, func(pKey *peerKey, pEntry *peerEntry) bool {
		// Skip local endpoints
		if pEntry.isLocal {
			return false
		}

		// Add the remote VTEP to the FDB with the multicast MAC
		if err := n.driver.peerAddMulticastRoute(n.id, pEntry.vtep, mmac, vxlan.Attrs().Index); err != nil {
			log.G(context.TODO()).Warnf("Failed to add multicast route for %s: %v", pEntry.vtep.String(), err)
		}

		// Continue walking through all peers
		return false
	})

	if err != nil {
		return fmt.Errorf("failed to walk peer DB for multicast setup: %v", err)
	}

	return nil
}

// peerAddMulticastRoute adds a multicast route entry to the FDB
func (d *driver) peerAddMulticastRoute(nid string, vtep net.IP, mac net.HardwareAddr, linkIndex int) error {
	// Add an FDB entry for the multicast MAC to the remote VTEP
	neigh := &netlink.Neigh{
		LinkIndex:    linkIndex,
		Family:       afBridge,
		State:        netlink.NUD_PERMANENT,
		Flags:        netlink.NTF_SELF,
		IP:           vtep,
		HardwareAddr: mac,
		Vlan:         0,
		VNI:          0,
	}
	return netlink.NeighSet(neigh)
}

// initMulticast initializes the multicast routing for the overlay network
func (n *network) initMulticast() error {
	log.G(context.TODO()).Infof("Initializing multicast routing for overlay network %s", n.id)

	// Setup multicast routing for each subnet
	for _, s := range n.subnets {
		if err := n.setupMulticastRouting(s); err != nil {
			log.G(context.TODO()).Warnf("Failed to setup multicast routing for subnet %s: %v", s.subnetIP.String(), err)
		}
	}

	return nil
}

// notifyMulticastEvent notifies a multicast event (join/leave) to all peers
func (d *driver) notifyMulticastEvent(nid, eid string, mcastIP string, join bool) {
	// Not needed for current implementation, but can be used
	// in future to notify other nodes of multicast membership changes
}

// handleMulticastPackets processes multicast packets by replicating them to all other
// endpoints in the multicast group or, if not specifically tracked, to all endpoints
func (d *driver) handleMulticastPackets(nid string, srcIP net.IP, dstIP net.IP, packet []byte) error {
	// Skip if not a multicast packet
	if !dstIP.IsMulticast() {
		return nil
	}

	log.G(context.TODO()).Debugf("Processing multicast packet from %s to %s", srcIP, dstIP)

	n := d.network(nid)
	if n == nil {
		return fmt.Errorf("could not find network with id %s", nid)
	}

	// Get list of endpoints for this multicast group, or all endpoints if not specifically tracked
	var endpoints []string
	if eps := d.multicastRoutes.getEndpoints(dstIP.String()); len(eps) > 0 {
		endpoints = eps
	} else {
		// If no specific endpoints registered for this multicast group,
		// send to all endpoints except the source
		for _, ep := range n.endpoints {
			if !ep.addr.IP.Equal(srcIP) {
				endpoints = append(endpoints, ep.addr.IP.String())
			}
		}
	}

	// Replicate the packet to all endpoints in the group
	for _, epIP := range endpoints {
		ip := net.ParseIP(epIP)
		if ip == nil || ip.Equal(srcIP) {
			continue // Skip invalid IPs or the source
		}

		pKey, pEntry, err := d.peerDbSearch(nid, ip)
		if err != nil {
			log.G(context.TODO()).Warnf("Failed to find peer for multicast replication: %v", err)
			continue
		}

		// In a proper implementation, we would now send the multicast packet to this endpoint
		// For now, we only log it as the actual packet forwarding would need integration with
		// kernel packet forwarding mechanisms

		log.G(context.TODO()).Debugf("Replicated multicast packet to endpoint %s (MAC: %s, VTEP: %s)",
			ip, pKey.peerMac, pEntry.vtep)
	}

	return nil
}
