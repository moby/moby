//go:build linux

package overlay

import (
	"context"
	"fmt"
	"net"
	"net/netip"
	"os"
	"syscall"
	"time"

	"github.com/containerd/log"
	"github.com/docker/docker/libnetwork/ns"
	"github.com/docker/docker/libnetwork/osl"
	"github.com/vishvananda/netlink"
)

const (
	vxlanMulticastMac = "01:00:5e:00:00:00"
	afBridge          = syscall.AF_BRIDGE
	
	// IGMP message types
	igmpMembershipQuery    = 0x11
	igmpV1MembershipReport = 0x12
	igmpV2MembershipReport = 0x16
	igmpV2LeaveGroup       = 0x17
	igmpV3MembershipReport = 0x22
	
	// MLD message types (IPv6)
	mldQuery        = 130
	mldReport       = 131
	mldDone         = 132
	mldV2Report     = 143
	
	// Default multicast groups
	allHostsMulticast = "224.0.0.1"
	allRoutersMulticast = "224.0.0.2"
	
	// Rate limiting constants
	defaultMulticastRate = 1000  // packets per second
	defaultBurstSize     = 100   // burst size for rate limiting
	maxMulticastGroups   = 256   // maximum multicast groups per network
)

func (n *network) setupMulticastRouting(s *subnet) error {
	if s == nil {
		return fmt.Errorf("subnet cannot be nil")
	}
	
	if s.vxlanName == "" {
		return fmt.Errorf("subnet vxlan name cannot be empty")
	}

	vxlan, err := netlink.LinkByName(s.vxlanName)
	if err != nil {
		return fmt.Errorf("failed to find vxlan interface %s: %v", s.vxlanName, err)
	}

	vxlanAttrs := vxlan.Attrs()
	if vxlanAttrs == nil {
		return fmt.Errorf("failed to get vxlan interface attributes for %s", s.vxlanName)
	}

	mmac, err := net.ParseMAC(vxlanMulticastMac)
	if err != nil {
		return fmt.Errorf("failed to parse multicast MAC %s: %v", vxlanMulticastMac, err)
	}

	var routeErrors []error
	
	err = n.driver.peerDbNetworkWalk(n.id, func(peerIP netip.Addr, peerMac net.HardwareAddr, pEntry *peerEntry) bool {
		if pEntry.isLocal {
			return false
		}

		if err := n.driver.peerAddMulticastRoute(n.id, pEntry.vtep, mmac, vxlanAttrs.Index); err != nil {
			routeErrors = append(routeErrors, fmt.Errorf("failed to add multicast route for %s: %v", pEntry.vtep.String(), err))
			log.G(context.TODO()).Warnf("Failed to add multicast route for %s: %v", pEntry.vtep.String(), err)
		}

		return false
	})

	if err != nil {
		return fmt.Errorf("failed to walk peer DB for multicast setup: %v", err)
	}

	// Log route errors but don't fail the entire setup
	if len(routeErrors) > 0 {
		log.G(context.TODO()).Warnf("Encountered %d errors during multicast route setup for subnet %s", len(routeErrors), s.subnetIP.String())
	}

	return nil
}

func (d *driver) peerAddMulticastRoute(nid string, vtep netip.Addr, mac net.HardwareAddr, linkIndex int) error {
	if !vtep.IsValid() {
		return fmt.Errorf("invalid VTEP address")
	}
	
	if len(mac) == 0 {
		return fmt.Errorf("MAC address cannot be empty")
	}
	
	if linkIndex <= 0 {
		return fmt.Errorf("invalid link index: %d", linkIndex)
	}

	neigh := &netlink.Neigh{
		LinkIndex:    linkIndex,
		Family:       afBridge,
		State:        netlink.NUD_PERMANENT,
		Flags:        netlink.NTF_SELF,
		IP:           vtep.AsSlice(),
		HardwareAddr: mac,
	}
	
	if err := ns.NlHandle().NeighSet(neigh); err != nil {
		return fmt.Errorf("failed to set neighbor entry for VTEP %s: %v", vtep, err)
	}
	
	return nil
}

func (n *network) initMulticast() error {
	log.G(context.TODO()).Infof("Initializing multicast routing for overlay network %s", n.id)

	for _, s := range n.subnets {
		if err := n.setupMulticastRouting(s); err != nil {
			log.G(context.TODO()).Warnf("Failed to setup multicast routing for subnet %s: %v", s.subnetIP.String(), err)
		}

		if err := n.enableMulticastForwarding(s); err != nil {
			log.G(context.TODO()).Warnf("Failed to enable multicast forwarding for subnet %s: %v", s.subnetIP.String(), err)
		}

		// Start IGMP proxy for this subnet
		if err := n.startIGMPProxy(s); err != nil {
			log.G(context.TODO()).Warnf("Failed to start IGMP proxy for subnet %s: %v", s.subnetIP.String(), err)
		}
	}

	// Setup inter-subnet multicast routing
	if err := n.setupInterSubnetMulticastRouting(); err != nil {
		log.G(context.TODO()).Warnf("Failed to setup inter-subnet multicast routing: %v", err)
	}

	return nil
}

func (n *network) enableMulticastForwarding(s *subnet) error {
	vxlan, err := netlink.LinkByName(s.vxlanName)
	if err != nil {
		return fmt.Errorf("failed to find vxlan interface %s: %v", s.vxlanName, err)
	}

	vxlanAttrs := vxlan.Attrs()
	if vxlanAttrs == nil {
		return fmt.Errorf("failed to get vxlan attributes")
	}

	if err := ns.NlHandle().LinkSetAllmulticastOn(vxlan); err != nil {
		return fmt.Errorf("failed to enable allmulticast on %s: %v", s.vxlanName, err)
	}

	return nil
}

func (n *network) addMulticastFDBEntry(vtep netip.Addr, groupMac net.HardwareAddr, vni uint32) error {
	for _, s := range n.subnets {
		if s.vni != vni {
			continue
		}

		vxlan, err := netlink.LinkByName(s.vxlanName)
		if err != nil {
			return fmt.Errorf("failed to find vxlan interface %s: %v", s.vxlanName, err)
		}

		neigh := &netlink.Neigh{
			LinkIndex:    vxlan.Attrs().Index,
			Family:       afBridge,
			State:        netlink.NUD_PERMANENT,
			Flags:        netlink.NTF_SELF,
			IP:           vtep.AsSlice(),
			HardwareAddr: groupMac,
		}

		if err := ns.NlHandle().NeighSet(neigh); err != nil {
			return fmt.Errorf("failed to add FDB entry: %v", err)
		}
		break
	}

	return nil
}

func (d *driver) peerAddMulticast(nid, eid string, peerIP netip.Prefix, vtep netip.Addr) error {
	if !peerIP.Addr().IsMulticast() {
		return nil
	}

	n := d.network(nid)
	if n == nil {
		return fmt.Errorf("network %s not found", nid)
	}

	groupMac := multicastIPToMAC(peerIP.Addr())

	for _, s := range n.subnets {
		if err := n.addMulticastFDBEntry(vtep, groupMac, s.vni); err != nil {
			log.G(context.TODO()).Warnf("Failed to add multicast FDB entry for %s: %v", peerIP, err)
		}
	}

	return nil
}

func (d *driver) peerDeleteMulticast(nid, eid string, peerIP netip.Prefix, vtep netip.Addr) error {
	if !peerIP.Addr().IsMulticast() {
		return nil
	}

	n := d.network(nid)
	if n == nil {
		return fmt.Errorf("network %s not found", nid)
	}

	groupMac := multicastIPToMAC(peerIP.Addr())

	for _, s := range n.subnets {
		vxlan, err := netlink.LinkByName(s.vxlanName)
		if err != nil {
			continue
		}

		neigh := &netlink.Neigh{
			LinkIndex:    vxlan.Attrs().Index,
			Family:       afBridge,
			State:        netlink.NUD_PERMANENT,
			Flags:        netlink.NTF_SELF,
			IP:           vtep.AsSlice(),
			HardwareAddr: groupMac,
		}

		if err := ns.NlHandle().NeighDel(neigh); err != nil {
			log.G(context.TODO()).Warnf("Failed to delete multicast FDB entry: %v", err)
		}
	}

	return nil
}

func multicastIPToMAC(ip netip.Addr) net.HardwareAddr {
	if ip.Is4() {
		mac := make(net.HardwareAddr, 6)
		mac[0] = 0x01
		mac[1] = 0x00
		mac[2] = 0x5e
		ipBytes := ip.As4()
		mac[3] = ipBytes[1] & 0x7f
		mac[4] = ipBytes[2]
		mac[5] = ipBytes[3]
		return mac
	} else if ip.Is6() {
		mac := make(net.HardwareAddr, 6)
		mac[0] = 0x33
		mac[1] = 0x33
		ipBytes := ip.As16()
		mac[2] = ipBytes[12]
		mac[3] = ipBytes[13]
		mac[4] = ipBytes[14]
		mac[5] = ipBytes[15]
		return mac
	}
	return nil
}

// IGMP proxy functionality for handling container join/leave
type igmpProxy struct {
	network    *network
	subnet     *subnet
	stopCh     chan struct{}
	groupState map[string]time.Time // group IP -> last seen time
}

func (n *network) startIGMPProxy(s *subnet) error {
	proxy := &igmpProxy{
		network:    n,
		subnet:     s,
		stopCh:     make(chan struct{}),
		groupState: make(map[string]time.Time),
	}
	
	// Start IGMP proxy goroutine
	go proxy.run()
	
	return nil
}

func (proxy *igmpProxy) run() {
	ticker := time.NewTicker(30 * time.Second) // Cleanup every 30 seconds
	defer ticker.Stop()
	
	for {
		select {
		case <-ticker.C:
			proxy.cleanupExpiredGroups()
		case <-proxy.stopCh:
			return
		case <-proxy.network.stopCh:
			return
		}
	}
}

func (proxy *igmpProxy) cleanupExpiredGroups() {
	now := time.Now()
	for groupIP, lastSeen := range proxy.groupState {
		// Remove groups not seen for 5 minutes
		if now.Sub(lastSeen) > 5*time.Minute {
			delete(proxy.groupState, groupIP)
			
			// Remove multicast routes for expired group
			if addr, err := netip.ParseAddr(groupIP); err == nil {
				proxy.removeMulticastRoute(addr)
			}
		}
	}
}

func (proxy *igmpProxy) handleIGMPJoin(groupIP netip.Addr) error {
	if !groupIP.IsMulticast() {
		return nil
	}
	
	// Skip reserved multicast addresses
	if groupIP.String() == allHostsMulticast || groupIP.String() == allRoutersMulticast {
		return nil
	}
	
	// Update group state
	proxy.groupState[groupIP.String()] = time.Now()
	
	// Add multicast route
	return proxy.addMulticastRoute(groupIP)
}

func (proxy *igmpProxy) handleIGMPLeave(groupIP netip.Addr) error {
	if !groupIP.IsMulticast() {
		return nil
	}
	
	// Remove from group state
	delete(proxy.groupState, groupIP.String())
	
	// Remove multicast route
	return proxy.removeMulticastRoute(groupIP)
}

func (proxy *igmpProxy) addMulticastRoute(groupIP netip.Addr) error {
	groupMAC := multicastIPToMAC(groupIP)
	if groupMAC == nil {
		return fmt.Errorf("failed to convert multicast IP to MAC")
	}
	
	// Add FDB entries for all remote peers
	return proxy.network.driver.peerDbNetworkWalk(proxy.network.id, func(peerIP netip.Addr, peerMAC net.HardwareAddr, pEntry *peerEntry) bool {
		if pEntry.isLocal {
			return false
		}
		
		vxlan, err := netlink.LinkByName(proxy.subnet.vxlanName)
		if err != nil {
			return false
		}
		
		neigh := &netlink.Neigh{
			LinkIndex:    vxlan.Attrs().Index,
			Family:       afBridge,
			State:        netlink.NUD_PERMANENT,
			Flags:        netlink.NTF_SELF,
			IP:           pEntry.vtep.AsSlice(),
			HardwareAddr: groupMAC,
		}
		
		if err := ns.NlHandle().NeighSet(neigh); err != nil {
			log.G(context.TODO()).Debugf("Failed to add multicast FDB entry for group %s to VTEP %s: %v", groupIP, pEntry.vtep, err)
		} else {
			log.G(context.TODO()).Debugf("Added multicast FDB entry: Group=%s MAC=%s VTEP=%s", groupIP, groupMAC, pEntry.vtep)
		}
		
		return false
	})
}

func (proxy *igmpProxy) removeMulticastRoute(groupIP netip.Addr) error {
	groupMAC := multicastIPToMAC(groupIP)
	if groupMAC == nil {
		return fmt.Errorf("failed to convert multicast IP to MAC")
	}
	
	// Remove FDB entries for all remote peers
	return proxy.network.driver.peerDbNetworkWalk(proxy.network.id, func(peerIP netip.Addr, peerMAC net.HardwareAddr, pEntry *peerEntry) bool {
		if pEntry.isLocal {
			return false
		}
		
		vxlan, err := netlink.LinkByName(proxy.subnet.vxlanName)
		if err != nil {
			return false
		}
		
		neigh := &netlink.Neigh{
			LinkIndex:    vxlan.Attrs().Index,
			Family:       afBridge,
			State:        netlink.NUD_PERMANENT,
			Flags:        netlink.NTF_SELF,
			IP:           pEntry.vtep.AsSlice(),
			HardwareAddr: groupMAC,
		}
		
		if err := ns.NlHandle().NeighDel(neigh); err != nil {
			log.G(context.TODO()).Debugf("Failed to remove multicast FDB entry for group %s from VTEP %s: %v", groupIP, pEntry.vtep, err)
		} else {
			log.G(context.TODO()).Debugf("Removed multicast FDB entry: Group=%s MAC=%s VTEP=%s", groupIP, groupMAC, pEntry.vtep)
		}
		
		return false
	})
}

// Container lifecycle integration 
func (n *network) handleContainerMulticastJoin(containerIP netip.Addr, groups []netip.Addr) error {
	for _, s := range n.subnets {
		if s.subnetIP.Contains(containerIP) {
			for _, group := range groups {
				if proxy := n.getIGMPProxy(s); proxy != nil {
					if err := proxy.handleIGMPJoin(group); err != nil {
						log.G(context.TODO()).Warnf("Failed to handle IGMP join for %s: %v", group, err)
					}
				}
			}
			break
		}
	}
	return nil
}

func (n *network) handleContainerMulticastLeave(containerIP netip.Addr, groups []netip.Addr) error {
	for _, s := range n.subnets {
		if s.subnetIP.Contains(containerIP) {
			for _, group := range groups {
				if proxy := n.getIGMPProxy(s); proxy != nil {
					if err := proxy.handleIGMPLeave(group); err != nil {
						log.G(context.TODO()).Warnf("Failed to handle IGMP leave for %s: %v", group, err)
					}
				}
			}
			break
		}
	}
	return nil
}

func (n *network) getIGMPProxy(s *subnet) *igmpProxy {
	// This would need to be stored in the subnet struct in a real implementation
	// For now, return nil as placeholder
	return nil
}

// Inter-subnet multicast routing
func (n *network) setupInterSubnetMulticastRouting() error {
	if len(n.subnets) <= 1 {
		return nil // No inter-subnet routing needed
	}

	log.G(context.TODO()).Infof("Setting up inter-subnet multicast routing for network %s", n.id)

	// For each subnet, add routes to other subnets for multicast traffic
	for i, sourceSubnet := range n.subnets {
		for j, targetSubnet := range n.subnets {
			if i == j {
				continue // Skip same subnet
			}

			if err := n.addInterSubnetMulticastRoute(sourceSubnet, targetSubnet); err != nil {
				log.G(context.TODO()).Warnf("Failed to add inter-subnet multicast route from %s to %s: %v", 
					sourceSubnet.subnetIP, targetSubnet.subnetIP, err)
			}
		}
	}

	return nil
}

func (n *network) addInterSubnetMulticastRoute(source, target *subnet) error {
	if source == nil || target == nil {
		return fmt.Errorf("source and target subnets cannot be nil")
	}

	sourceBridge, err := netlink.LinkByName(source.brName)
	if err != nil {
		return fmt.Errorf("failed to find source bridge %s: %v", source.brName, err)
	}

	targetBridge, err := netlink.LinkByName(target.brName)
	if err != nil {
		return fmt.Errorf("failed to find target bridge %s: %v", target.brName, err)
	}

	// Add multicast forwarding rule between bridges
	return n.sbox.InvokeFunc(func() error {
		// Enable multicast forwarding between the two bridge interfaces
		srcIndex := sourceBridge.Attrs().Index
		tgtIndex := targetBridge.Attrs().Index

		// Create a bridge FDB entry for multicast traffic forwarding
		// This is a simplified implementation - in production, this would need
		// more sophisticated routing based on IGMP join/leave messages
		route := &netlink.Route{
			Dst:       &net.IPNet{IP: net.IPv4(224, 0, 0, 0), Mask: net.CIDRMask(4, 32)}, // All multicast
			LinkIndex: tgtIndex,
			Scope:     netlink.SCOPE_LINK,
			Type:      syscall.RTN_MULTICAST,
		}

		if err := netlink.RouteAdd(route); err != nil && !os.IsExist(err) {
			return fmt.Errorf("failed to add multicast route: %v", err)
		}

		log.G(context.TODO()).Debugf("Added inter-subnet multicast route from bridge %s (idx=%d) to bridge %s (idx=%d)", 
			source.brName, srcIndex, target.brName, tgtIndex)

		return nil
	})
}

func (n *network) removeInterSubnetMulticastRoute(source, target *subnet) error {
	if source == nil || target == nil {
		return fmt.Errorf("source and target subnets cannot be nil")
	}

	targetBridge, err := netlink.LinkByName(target.brName)
	if err != nil {
		return fmt.Errorf("failed to find target bridge %s: %v", target.brName, err)
	}

	return n.sbox.InvokeFunc(func() error {
		tgtIndex := targetBridge.Attrs().Index

		route := &netlink.Route{
			Dst:       &net.IPNet{IP: net.IPv4(224, 0, 0, 0), Mask: net.CIDRMask(4, 32)}, // All multicast
			LinkIndex: tgtIndex,
			Scope:     netlink.SCOPE_LINK,
			Type:      syscall.RTN_MULTICAST,
		}

		if err := netlink.RouteDel(route); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("failed to remove multicast route: %v", err)
		}

		log.G(context.TODO()).Debugf("Removed inter-subnet multicast route from bridge %s to bridge %s", 
			source.brName, target.brName)

		return nil
	})
}

// Enhanced multicast group management with subnet awareness
func (n *network) handleMulticastGroupJoin(groupIP netip.Addr, sourceSubnet *subnet) error {
	if !groupIP.IsMulticast() {
		return nil
	}

	log.G(context.TODO()).Debugf("Handling multicast group join for %s in subnet %s", groupIP, sourceSubnet.subnetIP)

	// Add group to local subnet
	groupMAC := multicastIPToMAC(groupIP)
	if groupMAC == nil {
		return fmt.Errorf("failed to convert multicast IP to MAC")
	}

	// Add FDB entries in source subnet
	if err := n.addMulticastFDBEntry(netip.Addr{}, groupMAC, sourceSubnet.vni); err != nil {
		log.G(context.TODO()).Warnf("Failed to add local multicast FDB entry: %v", err)
	}

	// Propagate to other subnets if inter-subnet multicast is enabled
	for _, targetSubnet := range n.subnets {
		if targetSubnet.vni == sourceSubnet.vni {
			continue
		}

		if err := n.addMulticastFDBEntry(netip.Addr{}, groupMAC, targetSubnet.vni); err != nil {
			log.G(context.TODO()).Warnf("Failed to propagate multicast group to subnet %s: %v", 
				targetSubnet.subnetIP, err)
		}
	}

	return nil
}

func (n *network) handleMulticastGroupLeave(groupIP netip.Addr, sourceSubnet *subnet) error {
	if !groupIP.IsMulticast() {
		return nil
	}

	log.G(context.TODO()).Debugf("Handling multicast group leave for %s in subnet %s", groupIP, sourceSubnet.subnetIP)

	groupMAC := multicastIPToMAC(groupIP)
	if groupMAC == nil {
		return fmt.Errorf("failed to convert multicast IP to MAC")
	}

	// Remove from all subnets - in a production implementation, this would check
	// if other containers in other subnets are still subscribed
	for _, subnet := range n.subnets {
		if err := n.removeMulticastFDBEntry(groupMAC, subnet.vni); err != nil {
			log.G(context.TODO()).Warnf("Failed to remove multicast FDB entry from subnet %s: %v", 
				subnet.subnetIP, err)
		}
	}

	return nil
}

func (n *network) removeMulticastFDBEntry(groupMac net.HardwareAddr, vni uint32) error {
	for _, s := range n.subnets {
		if s.vni != vni {
			continue
		}

		vxlan, err := netlink.LinkByName(s.vxlanName)
		if err != nil {
			continue
		}

		// Remove all FDB entries for this multicast group
		err = n.driver.peerDbNetworkWalk(n.id, func(peerIP netip.Addr, peerMAC net.HardwareAddr, pEntry *peerEntry) bool {
			if pEntry.isLocal {
				return false
			}

			neigh := &netlink.Neigh{
				LinkIndex:    vxlan.Attrs().Index,
				Family:       afBridge,
				State:        netlink.NUD_PERMANENT,
				Flags:        netlink.NTF_SELF,
				IP:           pEntry.vtep.AsSlice(),
				HardwareAddr: groupMac,
			}

			if err := ns.NlHandle().NeighDel(neigh); err != nil {
				log.G(context.TODO()).Debugf("Failed to delete multicast FDB entry: %v", err)
			}

			return false
		})

		if err != nil {
			return fmt.Errorf("failed to walk peer DB for multicast cleanup: %v", err)
		}
		break
	}

	return nil
}

// Rate limiting and storm control
type multicastRateLimiter struct {
	network     *network
	groupCounts map[string]int           // group IP -> active member count
	rateLimits  map[string]time.Time     // group IP -> last rate limit reset time
	packetCount map[string]int           // group IP -> packet count in current interval
	stopCh      chan struct{}
}

func (n *network) startMulticastRateLimiter() *multicastRateLimiter {
	limiter := &multicastRateLimiter{
		network:     n,
		groupCounts: make(map[string]int),
		rateLimits:  make(map[string]time.Time),
		packetCount: make(map[string]int),
		stopCh:      make(chan struct{}),
	}
	
	go limiter.run()
	return limiter
}

func (limiter *multicastRateLimiter) run() {
	ticker := time.NewTicker(time.Second) // Reset counters every second
	defer ticker.Stop()
	
	for {
		select {
		case <-ticker.C:
			limiter.resetCounters()
		case <-limiter.stopCh:
			return
		case <-limiter.network.stopCh:
			return
		}
	}
}

func (limiter *multicastRateLimiter) resetCounters() {
	now := time.Now()
	for groupIP := range limiter.packetCount {
		limiter.rateLimits[groupIP] = now
		limiter.packetCount[groupIP] = 0
	}
}

func (limiter *multicastRateLimiter) checkRateLimit(groupIP netip.Addr) bool {
	groupStr := groupIP.String()
	
	// Check maximum groups limit
	if len(limiter.groupCounts) >= maxMulticastGroups {
		if _, exists := limiter.groupCounts[groupStr]; !exists {
			log.G(context.TODO()).Warnf("Maximum multicast groups (%d) reached, dropping group %s", 
				maxMulticastGroups, groupStr)
			return false
		}
	}
	
	// Check rate limit
	now := time.Now()
	lastReset, exists := limiter.rateLimits[groupStr]
	if !exists {
		limiter.rateLimits[groupStr] = now
		limiter.packetCount[groupStr] = 1
		return true
	}
	
	// If more than a second has passed, reset counter
	if now.Sub(lastReset) >= time.Second {
		limiter.rateLimits[groupStr] = now
		limiter.packetCount[groupStr] = 1
		return true
	}
	
	// Check if within rate limit
	currentCount := limiter.packetCount[groupStr]
	if currentCount >= defaultMulticastRate {
		log.G(context.TODO()).Debugf("Rate limit exceeded for multicast group %s (count: %d)", 
			groupStr, currentCount)
		return false
	}
	
	limiter.packetCount[groupStr]++
	return true
}

func (limiter *multicastRateLimiter) addGroup(groupIP netip.Addr) {
	groupStr := groupIP.String()
	limiter.groupCounts[groupStr]++
	
	if limiter.groupCounts[groupStr] == 1 {
		// First member of this group
		limiter.rateLimits[groupStr] = time.Now()
		limiter.packetCount[groupStr] = 0
	}
}

func (limiter *multicastRateLimiter) removeGroup(groupIP netip.Addr) {
	groupStr := groupIP.String()
	
	if count, exists := limiter.groupCounts[groupStr]; exists {
		count--
		if count <= 0 {
			// No more members, remove group tracking
			delete(limiter.groupCounts, groupStr)
			delete(limiter.rateLimits, groupStr)
			delete(limiter.packetCount, groupStr)
		} else {
			limiter.groupCounts[groupStr] = count
		}
	}
}

func (limiter *multicastRateLimiter) stop() {
	close(limiter.stopCh)
}

// Bridge storm control configuration
func (n *network) enableBridgeStormControl(sbox *osl.Namespace, brName string) error {
	return sbox.InvokeFunc(func() error {
		// Set multicast hash max (limits multicast forwarding table size)
		hashMaxPath := fmt.Sprintf("/sys/class/net/%s/bridge/multicast_hash_max", brName)
		if err := os.WriteFile(hashMaxPath, []byte("512"), 0644); err != nil {
			log.G(context.TODO()).Debugf("Failed to set multicast hash max: %v", err)
		}
		
		// Set multicast hash elasticity (controls hash collision handling)
		hashElasticityPath := fmt.Sprintf("/sys/class/net/%s/bridge/multicast_hash_elasticity", brName)
		if err := os.WriteFile(hashElasticityPath, []byte("16"), 0644); err != nil {
			log.G(context.TODO()).Debugf("Failed to set multicast hash elasticity: %v", err)
		}
		
		// Enable multicast fast leave (removes ports immediately on IGMP leave)
		fastLeavePath := fmt.Sprintf("/sys/class/net/%s/bridge/multicast_fast_leave", brName)
		if err := os.WriteFile(fastLeavePath, []byte("1"), 0644); err != nil {
			log.G(context.TODO()).Debugf("Failed to enable multicast fast leave: %v", err)
		}
		
		// Set multicast startup query count (number of queries sent on startup)
		startupQueryCountPath := fmt.Sprintf("/sys/class/net/%s/bridge/multicast_startup_query_count", brName)
		if err := os.WriteFile(startupQueryCountPath, []byte("2"), 0644); err != nil {
			log.G(context.TODO()).Debugf("Failed to set startup query count: %v", err)
		}
		
		// Set multicast startup query interval (time between startup queries)
		startupQueryIntervalPath := fmt.Sprintf("/sys/class/net/%s/bridge/multicast_startup_query_interval", brName)
		if err := os.WriteFile(startupQueryIntervalPath, []byte("3125"), 0644); err != nil { // 31.25 seconds in centiseconds
			log.G(context.TODO()).Debugf("Failed to set startup query interval: %v", err)
		}
		
		log.G(context.TODO()).Debugf("Enabled multicast storm control on bridge %s", brName)
		return nil
	})
}
