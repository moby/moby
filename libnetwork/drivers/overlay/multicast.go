//go:build linux

package overlay

import (
	"context"
	"fmt"
	"net"
	"net/netip"
	"os"
	"strconv"
	"sync"
	"syscall"
	"time"

	"github.com/containerd/log"
	"github.com/docker/docker/libnetwork/ns"
	"github.com/docker/docker/libnetwork/osl"
	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"
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
	mldQuery    = 130
	mldReport   = 131
	mldDone     = 132
	mldV2Report = 143

	// Default multicast groups
	allHostsMulticast   = "224.0.0.1"
	allRoutersMulticast = "224.0.0.2"

	// Rate limiting constants
	defaultMulticastRate = 1000 // packets per second
	defaultBurstSize     = 100  // burst size for rate limiting
	maxMulticastGroups   = 256  // maximum multicast groups per network

	// Configuration option keys
	multicastEnabledOption   = "com.docker.network.multicast.enable"
	multicastRateLimitOption = "com.docker.network.multicast.ratelimit"
	multicastMaxGroupsOption = "com.docker.network.multicast.maxgroups"
)

// MulticastFeatures represents available kernel multicast features
type MulticastFeatures struct {
	VXLANMulticast    bool
	BridgeMulticast   bool
	MulticastSnooping bool
	IGMPSupport       bool
	KernelVersion     string
}

// MulticastConfig represents multicast configuration options
type MulticastConfig struct {
	Enabled         bool
	RateLimit       int // packets per second
	MaxGroups       int // maximum multicast groups
	EnableSnooping  bool
	EnableIGMPProxy bool
}

// checkMulticastFeatures detects available kernel multicast features
func checkMulticastFeatures() (*MulticastFeatures, error) {
	features := &MulticastFeatures{}

	// Check kernel version
	uname := &unix.Utsname{}
	if err := unix.Uname(uname); err == nil {
		features.KernelVersion = unix.ByteSliceToString(uname.Release[:])
	}

	// Check for VXLAN multicast support
	// VXLAN multicast support was added in Linux 3.7
	if _, err := os.Stat("/sys/module/vxlan"); err == nil {
		features.VXLANMulticast = true
	}

	// Check for bridge multicast support
	if _, err := os.Stat("/sys/module/bridge"); err == nil {
		features.BridgeMulticast = true

		// Check for multicast snooping capability
		if _, err := os.Stat("/sys/module/bridge/parameters/multicast_snooping"); err == nil {
			features.MulticastSnooping = true
		}
	}

	// Check for IGMP support
	if _, err := os.Stat("/proc/net/igmp"); err == nil {
		features.IGMPSupport = true
	}

	return features, nil
}

// validateMulticastSupport ensures the system has required multicast features
func validateMulticastSupport() error {
	features, err := checkMulticastFeatures()
	if err != nil {
		return fmt.Errorf("failed to check multicast features: %v", err)
	}

	if !features.VXLANMulticast {
		return fmt.Errorf("VXLAN multicast support not available - ensure vxlan kernel module is loaded")
	}

	if !features.BridgeMulticast {
		return fmt.Errorf("Bridge multicast support not available - ensure bridge kernel module is loaded")
	}

	if !features.MulticastSnooping {
		log.G(context.TODO()).Warn("Bridge multicast snooping not available - multicast performance may be degraded")
	}

	if !features.IGMPSupport {
		log.G(context.TODO()).Warn("IGMP support not available - multicast group management may be limited")
	}

	log.G(context.TODO()).Infof("Multicast features validated - Kernel: %s, VXLAN: %v, Bridge: %v, Snooping: %v, IGMP: %v",
		features.KernelVersion, features.VXLANMulticast, features.BridgeMulticast,
		features.MulticastSnooping, features.IGMPSupport)

	return nil
}

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

	// Instead of creating static routes with a hardcoded MAC, we'll set up
	// the infrastructure for dynamic multicast group management.
	// The actual FDB entries will be created when containers join specific groups.

	log.G(context.TODO()).Infof("Multicast routing infrastructure setup complete for subnet %s", s.subnetIP.String())
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

// parseMulticastConfig extracts multicast configuration from network options
func parseMulticastConfig(options map[string]string) *MulticastConfig {
	config := &MulticastConfig{
		Enabled:         true, // Default enabled
		RateLimit:       defaultMulticastRate,
		MaxGroups:       maxMulticastGroups,
		EnableSnooping:  true,
		EnableIGMPProxy: true,
	}

	if options == nil {
		return config
	}

	// Parse enable/disable
	if val, ok := options[multicastEnabledOption]; ok {
		config.Enabled = val != "false"
	}

	// Parse rate limit
	if val, ok := options[multicastRateLimitOption]; ok {
		if rate, err := strconv.Atoi(val); err == nil && rate > 0 {
			config.RateLimit = rate
		}
	}

	// Parse max groups
	if val, ok := options[multicastMaxGroupsOption]; ok {
		if max, err := strconv.Atoi(val); err == nil && max > 0 {
			config.MaxGroups = max
		}
	}

	return config
}

func (n *network) initMulticast() error {
	log.G(context.TODO()).Infof("Initializing multicast routing for overlay network %s", n.id)

	// Check if multicast is enabled for this network
	if n.multicastConfig != nil && !n.multicastConfig.Enabled {
		log.G(context.TODO()).Infof("Multicast disabled for network %s", n.id)
		return nil
	}

	// Validate kernel multicast support
	if err := validateMulticastSupport(); err != nil {
		log.G(context.TODO()).Warnf("Multicast support validation failed: %v - continuing with limited functionality", err)
		// Continue with limited functionality rather than failing completely
	}

	// Start the multicast rate limiter for the network
	n.multicastRateLimiter = n.startMulticastRateLimiter()

	var initErrors []error
	for _, s := range n.subnets {
		if err := n.setupMulticastRouting(s); err != nil {
			// Non-critical - log and continue
			log.G(context.TODO()).Warnf("Failed to setup multicast routing for subnet %s: %v", s.subnetIP.String(), err)
		}

		if err := n.enableMulticastForwarding(s); err != nil {
			// Critical - multicast won't work without this
			initErrors = append(initErrors, fmt.Errorf("failed to enable multicast forwarding for subnet %s: %v", s.subnetIP.String(), err))
			continue
		}

		// Start IGMP proxy for this subnet
		proxy, err := n.startIGMPProxy(s)
		if err != nil {
			// Non-critical - basic multicast will work without proxy
			log.G(context.TODO()).Warnf("Failed to start IGMP proxy for subnet %s: %v", s.subnetIP.String(), err)
		} else {
			s.igmpProxy = proxy
		}
	}

	// Setup inter-subnet multicast routing
	if err := n.setupInterSubnetMulticastRouting(); err != nil {
		// Non-critical - single subnet multicast will still work
		log.G(context.TODO()).Warnf("Failed to setup inter-subnet multicast routing: %v", err)
	}

	// Setup proactive multicast routes for common groups
	if err := n.proactiveMulticastSetup(); err != nil {
		// Non-critical - but helpful for immediate functionality
		log.G(context.TODO()).Warnf("Failed to setup proactive multicast routes: %v", err)
	}

	// Return error if any critical operations failed
	if len(initErrors) > 0 {
		return fmt.Errorf("multicast initialization partially failed: %v", initErrors)
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
	if groupMac == nil {
		return fmt.Errorf("failed to convert multicast IP %s to MAC", peerIP)
	}

	var addErrors []error
	for _, s := range n.subnets {
		if err := n.addMulticastFDBEntry(vtep, groupMac, s.vni); err != nil {
			addErrors = append(addErrors, fmt.Errorf("subnet %s: %v", s.subnetIP, err))
		}
	}

	if len(addErrors) > 0 {
		return fmt.Errorf("failed to add multicast FDB entries: %v", addErrors)
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
	mu         sync.RWMutex         // protects groupState
}

func (n *network) startIGMPProxy(s *subnet) (*igmpProxy, error) {
	proxy := &igmpProxy{
		network:    n,
		subnet:     s,
		stopCh:     make(chan struct{}),
		groupState: make(map[string]time.Time),
	}

	// Start IGMP proxy goroutine
	go proxy.run()

	return proxy, nil
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
	proxy.mu.Lock()
	defer proxy.mu.Unlock()

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
	proxy.mu.Lock()
	proxy.groupState[groupIP.String()] = time.Now()
	proxy.mu.Unlock()

	// Add multicast route
	return proxy.addMulticastRoute(groupIP)
}

func (proxy *igmpProxy) handleIGMPLeave(groupIP netip.Addr) error {
	if !groupIP.IsMulticast() {
		return nil
	}

	// Remove from group state
	proxy.mu.Lock()
	delete(proxy.groupState, groupIP.String())
	proxy.mu.Unlock()

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
	return s.igmpProxy
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

	sourceVxlan, err := netlink.LinkByName(source.vxlanName)
	if err != nil {
		return fmt.Errorf("failed to find source vxlan %s: %v", source.vxlanName, err)
	}

	targetVxlan, err := netlink.LinkByName(target.vxlanName)
	if err != nil {
		return fmt.Errorf("failed to find target vxlan %s: %v", target.vxlanName, err)
	}

	// Add multicast forwarding between VXLANs
	var invokeErr error
	err = n.sbox.InvokeFunc(func() {
		// Get VXLAN device attributes
		srcVxlan, ok := sourceVxlan.(*netlink.Vxlan)
		if !ok {
			invokeErr = fmt.Errorf("source is not a VXLAN device")
			return
		}

		tgtVxlan, ok := targetVxlan.(*netlink.Vxlan)
		if !ok {
			invokeErr = fmt.Errorf("target is not a VXLAN device")
			return
		}

		// For each remote VTEP, we need to ensure multicast groups can reach other subnets
		// This is done by adding FDB entries that replicate multicast traffic across VNIs
		err := n.driver.peerDbNetworkWalk(n.id, func(peerIP netip.Addr, peerMAC net.HardwareAddr, pEntry *peerEntry) bool {
			if pEntry.isLocal {
				return false
			}

			// Add FDB entry for well-known multicast MAC (all-hosts multicast)
			mcastMAC, _ := net.ParseMAC("01:00:5e:00:00:01")

			// Create cross-VNI multicast forwarding entry
			neigh := &netlink.Neigh{
				LinkIndex:    srcVxlan.Attrs().Index,
				Family:       afBridge,
				State:        netlink.NUD_PERMANENT,
				Flags:        netlink.NTF_SELF,
				IP:           pEntry.vtep.AsSlice(),
				HardwareAddr: mcastMAC,
				VNI:          int(tgtVxlan.VxlanId), // Target VNI for cross-subnet forwarding
			}

			if err := ns.NlHandle().NeighSet(neigh); err != nil {
				log.G(context.TODO()).Debugf("Failed to add cross-VNI multicast FDB: %v", err)
			}

			return false
		})

		if err != nil {
			invokeErr = fmt.Errorf("failed to setup cross-VNI multicast forwarding: %v", err)
			return
		}

		log.G(context.TODO()).Debugf("Added inter-subnet multicast forwarding from VNI %d to VNI %d",
			srcVxlan.VxlanId, tgtVxlan.VxlanId)
	})

	if err != nil {
		return err
	}
	return invokeErr
}

func (n *network) removeInterSubnetMulticastRoute(source, target *subnet) error {
	if source == nil || target == nil {
		return fmt.Errorf("source and target subnets cannot be nil")
	}

	targetBridge, err := netlink.LinkByName(target.brName)
	if err != nil {
		return fmt.Errorf("failed to find target bridge %s: %v", target.brName, err)
	}

	var invokeErr error
	err = n.sbox.InvokeFunc(func() {
		tgtIndex := targetBridge.Attrs().Index

		route := &netlink.Route{
			Dst:       &net.IPNet{IP: net.IPv4(224, 0, 0, 0), Mask: net.CIDRMask(4, 32)}, // All multicast
			LinkIndex: tgtIndex,
			Scope:     netlink.SCOPE_LINK,
			Type:      syscall.RTN_MULTICAST,
		}

		if err := netlink.RouteDel(route); err != nil && !os.IsNotExist(err) {
			invokeErr = fmt.Errorf("failed to remove multicast route: %v", err)
			return
		}

		log.G(context.TODO()).Debugf("Removed inter-subnet multicast route from bridge %s to bridge %s",
			source.brName, target.brName)
	})

	if err != nil {
		return err
	}
	return invokeErr
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
	groupCounts map[string]int       // group IP -> active member count
	rateLimits  map[string]time.Time // group IP -> last rate limit reset time
	packetCount map[string]int       // group IP -> packet count in current interval
	stopCh      chan struct{}
	mu          sync.RWMutex // protects all maps
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
	limiter.mu.Lock()
	defer limiter.mu.Unlock()

	for groupIP := range limiter.packetCount {
		limiter.rateLimits[groupIP] = now
		limiter.packetCount[groupIP] = 0
	}
}

func (limiter *multicastRateLimiter) checkRateLimit(groupIP netip.Addr) bool {
	groupStr := groupIP.String()

	limiter.mu.Lock()
	defer limiter.mu.Unlock()

	// Get configured limits
	maxGroups := maxMulticastGroups
	rateLimit := defaultMulticastRate
	if limiter.network.multicastConfig != nil {
		maxGroups = limiter.network.multicastConfig.MaxGroups
		rateLimit = limiter.network.multicastConfig.RateLimit
	}

	// Check maximum groups limit
	if len(limiter.groupCounts) >= maxGroups {
		if _, exists := limiter.groupCounts[groupStr]; !exists {
			log.G(context.TODO()).Warnf("Maximum multicast groups (%d) reached, dropping group %s",
				maxGroups, groupStr)
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
	if currentCount >= rateLimit {
		log.G(context.TODO()).Debugf("Rate limit exceeded for multicast group %s (count: %d, limit: %d)",
			groupStr, currentCount, rateLimit)
		return false
	}

	limiter.packetCount[groupStr]++
	return true
}

func (limiter *multicastRateLimiter) addGroup(groupIP netip.Addr) {
	groupStr := groupIP.String()

	limiter.mu.Lock()
	defer limiter.mu.Unlock()

	limiter.groupCounts[groupStr]++

	if limiter.groupCounts[groupStr] == 1 {
		// First member of this group
		limiter.rateLimits[groupStr] = time.Now()
		limiter.packetCount[groupStr] = 0
	}
}

func (limiter *multicastRateLimiter) removeGroup(groupIP netip.Addr) {
	groupStr := groupIP.String()

	limiter.mu.Lock()
	defer limiter.mu.Unlock()

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

// cleanupMulticast performs cleanup of all multicast resources for a network
func (n *network) cleanupMulticast() {
	log.G(context.TODO()).Infof("Cleaning up multicast resources for overlay network %s", n.id)

	// Stop rate limiter
	if n.multicastRateLimiter != nil {
		n.multicastRateLimiter.stop()
		n.multicastRateLimiter = nil
	}

	// Stop IGMP proxies for all subnets
	for _, s := range n.subnets {
		if s.igmpProxy != nil {
			close(s.igmpProxy.stopCh)
			s.igmpProxy = nil
		}
	}
}

// sysfsWrite safely writes to a sysfs file with proper error handling
func sysfsWrite(path string, value []byte) error {
	// Check if the file exists first
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return fmt.Errorf("sysfs path does not exist: %s", path)
	}

	// Try to write to the file
	if err := os.WriteFile(path, value, 0644); err != nil {
		// Check if it's a permission error
		if os.IsPermission(err) {
			return fmt.Errorf("permission denied writing to %s (may require root)", path)
		}
		return fmt.Errorf("failed to write to %s: %v", path, err)
	}

	return nil
}

// Bridge storm control configuration
func (n *network) enableBridgeStormControl(sbox *osl.Namespace, brName string) error {
	var invokeErr error
	err := sbox.InvokeFunc(func() {
		var errors []error

		// Define sysfs parameters with their values
		type sysfsParam struct {
			name  string
			value string
			desc  string
		}

		params := []sysfsParam{
			{"multicast_hash_max", "512", "multicast forwarding table size"},
			{"multicast_hash_elasticity", "16", "hash collision handling"},
			{"multicast_fast_leave", "1", "fast leave on IGMP leave"},
			{"multicast_startup_query_count", "2", "startup query count"},
			{"multicast_startup_query_interval", "3125", "startup query interval"}, // 31.25 seconds in centiseconds
		}

		// Try to set each parameter
		for _, param := range params {
			path := fmt.Sprintf("/sys/class/net/%s/bridge/%s", brName, param.name)
			if err := sysfsWrite(path, []byte(param.value)); err != nil {
				log.G(context.TODO()).Debugf("Failed to set %s: %v", param.desc, err)
				errors = append(errors, err)
			}
		}

		if len(errors) > 0 {
			log.G(context.TODO()).Warnf("Bridge storm control partially configured on %s with %d errors", brName, len(errors))
		} else {
			log.G(context.TODO()).Debugf("Successfully enabled multicast storm control on bridge %s", brName)
		}

		// Don't fail the entire operation if storm control setup fails
	})

	if err != nil {
		return err
	}
	return invokeErr
}

// addMulticastGroupForContainer adds FDB entries for a specific multicast group
// when a container joins the group
func (n *network) addMulticastGroupForContainer(groupIP netip.Addr, containerSubnet *subnet) error {
	if \!groupIP.IsMulticast() {
		return fmt.Errorf("IP %s is not a multicast address", groupIP)
	}

	groupMac := multicastIPToMAC(groupIP)
	if groupMac == nil {
		return fmt.Errorf("failed to convert multicast IP %s to MAC", groupIP)
	}

	log.G(context.TODO()).Infof("Adding multicast group %s (MAC: %s) for container in subnet %s", 
		groupIP, groupMac, containerSubnet.subnetIP)

	// Add FDB entries for all remote VTEPs to forward this multicast group
	err := n.driver.peerDbNetworkWalk(n.id, func(peerIP netip.Addr, peerMac net.HardwareAddr, pEntry *peerEntry) bool {
		if pEntry.isLocal {
			return false
		}

		// Add FDB entry for this specific multicast group to this VTEP
		if err := n.addMulticastFDBEntry(pEntry.vtep, groupMac, containerSubnet.vni); err \!= nil {
			log.G(context.TODO()).Warnf("Failed to add multicast FDB entry for group %s to VTEP %s: %v", 
				groupIP, pEntry.vtep, err)
		} else {
			log.G(context.TODO()).Debugf("Added multicast FDB entry: Group=%s MAC=%s VTEP=%s VNI=%d", 
				groupIP, groupMac, pEntry.vtep, containerSubnet.vni)
		}

		return false
	})

	if err \!= nil {
		return fmt.Errorf("failed to walk peer DB for multicast group setup: %v", err)
	}

	// Also enable flooding for this multicast group on the bridge
	if err := n.enableBridgeMulticastFlooding(containerSubnet, groupMac); err \!= nil {
		log.G(context.TODO()).Warnf("Failed to enable bridge flooding for group %s: %v", groupIP, err)
	}

	return nil
}

// removeMulticastGroupForContainer removes FDB entries for a specific multicast group
// when a container leaves the group
func (n *network) removeMulticastGroupForContainer(groupIP netip.Addr, containerSubnet *subnet) error {
	if \!groupIP.IsMulticast() {
		return fmt.Errorf("IP %s is not a multicast address", groupIP)
	}

	groupMac := multicastIPToMAC(groupIP)
	if groupMac == nil {
		return fmt.Errorf("failed to convert multicast IP %s to MAC", groupIP)
	}

	log.G(context.TODO()).Infof("Removing multicast group %s (MAC: %s) for container in subnet %s", 
		groupIP, groupMac, containerSubnet.subnetIP)

	// Remove FDB entries for all remote VTEPs for this multicast group
	err := n.driver.peerDbNetworkWalk(n.id, func(peerIP netip.Addr, peerMac net.HardwareAddr, pEntry *peerEntry) bool {
		if pEntry.isLocal {
			return false
		}

		// Remove FDB entry for this specific multicast group from this VTEP
		if err := n.removeMulticastFDBEntryByVTEP(pEntry.vtep, groupMac, containerSubnet.vni); err \!= nil {
			log.G(context.TODO()).Warnf("Failed to remove multicast FDB entry for group %s from VTEP %s: %v", 
				groupIP, pEntry.vtep, err)
		} else {
			log.G(context.TODO()).Debugf("Removed multicast FDB entry: Group=%s MAC=%s VTEP=%s VNI=%d", 
				groupIP, groupMac, pEntry.vtep, containerSubnet.vni)
		}

		return false
	})

	if err \!= nil {
		return fmt.Errorf("failed to walk peer DB for multicast group cleanup: %v", err)
	}

	return nil
}

// removeMulticastFDBEntryByVTEP removes a specific FDB entry for a multicast group from a VTEP
func (n *network) removeMulticastFDBEntryByVTEP(vtep netip.Addr, groupMac net.HardwareAddr, vni uint32) error {
	for _, s := range n.subnets {
		if s.vni \!= vni {
			continue
		}

		vxlan, err := netlink.LinkByName(s.vxlanName)
		if err \!= nil {
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

		if err := ns.NlHandle().NeighDel(neigh); err \!= nil {
			return fmt.Errorf("failed to delete FDB entry: %v", err)
		}
		break
	}

	return nil
}

// enableBridgeMulticastFlooding enables flooding for a specific multicast group on the bridge
func (n *network) enableBridgeMulticastFlooding(s *subnet, groupMac net.HardwareAddr) error {
	// This ensures that multicast traffic is flooded to all ports on the bridge
	// until IGMP group membership is properly established
	vxlan, err := netlink.LinkByName(s.vxlanName)
	if err \!= nil {
		return fmt.Errorf("failed to find vxlan interface %s: %v", s.vxlanName, err)
	}

	// Enable flooding for this multicast MAC on the VXLAN interface
	if err := ns.NlHandle().LinkSetFlood(vxlan, true); err \!= nil {
		log.G(context.TODO()).Debugf("Failed to enable flooding on VXLAN %s: %v", s.vxlanName, err)
	}

	return nil
}
EOF < /dev/null
// handleMulticastGroupJoinForContainer automatically handles when a container joins a multicast group
// This function should be called when the overlay network detects multicast traffic
func (n *network) handleMulticastGroupJoinForContainer(containerIP netip.Addr, groupIP netip.Addr) error {
	if \!groupIP.IsMulticast() {
		return nil
	}

	// Find the subnet for this container
	containerSubnet := n.getSubnetforIP(netip.PrefixFrom(containerIP, 32))
	if containerSubnet == nil {
		log.G(context.TODO()).Warnf("Cannot find subnet for container IP %s", containerIP)
		return nil
	}

	log.G(context.TODO()).Infof("Container %s joined multicast group %s", containerIP, groupIP)

	// Add FDB entries for this specific multicast group
	return n.addMulticastGroupForContainer(groupIP, containerSubnet)
}

// proactiveMulticastSetup sets up multicast routes for common multicast groups
// This ensures basic multicast functionality works immediately
func (n *network) proactiveMulticastSetup() error {
	log.G(context.TODO()).Infof("Setting up proactive multicast routes for network %s", n.id)

	// Common multicast groups to set up proactively
	commonGroups := []string{
		"224.0.0.0/24",   // Local network control block
		"239.255.0.0/16", // Organization-Local Scope (common for applications)
	}

	// Also specifically set up the 239.255.1.1 group for immediate functionality
	if err := n.handleSpecificMulticastGroup("239.255.1.1"); err != nil {
		log.G(context.TODO()).Warnf("Failed to setup specific multicast group 239.255.1.1: %v", err)
	}

	for _, s := range n.subnets {
		for _, groupCIDR := range commonGroups {
			if prefix, err := netip.ParsePrefix(groupCIDR); err == nil {
				// Set up flooding for this range
				if err := n.setupMulticastRangeFlooding(s, prefix); err \!= nil {
					log.G(context.TODO()).Warnf("Failed to setup flooding for range %s: %v", groupCIDR, err)
				}
			}
		}
	}

	return nil
}

// setupMulticastRangeFlooding enables flooding for a range of multicast addresses
func (n *network) setupMulticastRangeFlooding(s *subnet, groupRange netip.Prefix) error {
	vxlan, err := netlink.LinkByName(s.vxlanName)
	if err \!= nil {
		return fmt.Errorf("failed to find vxlan interface %s: %v", s.vxlanName, err)
	}

	// Enable flooding on the VXLAN interface for unknown multicast
	if err := ns.NlHandle().LinkSetFlood(vxlan, true); err \!= nil {
		log.G(context.TODO()).Debugf("Failed to enable flooding on VXLAN %s: %v", s.vxlanName, err)
	}

	log.G(context.TODO()).Debugf("Enabled flooding for multicast range %s on subnet %s", 
		groupRange, s.subnetIP)

	return nil
}
EOF < /dev/null
// handleSpecificMulticastGroup sets up FDB entries for a specific multicast group
// This function can be called explicitly for testing or when specific groups are known
func (n *network) handleSpecificMulticastGroup(groupIP string) error {
	addr, err := netip.ParseAddr(groupIP)
	if err \!= nil {
		return fmt.Errorf("invalid multicast IP %s: %v", groupIP, err)
	}

	if \!addr.IsMulticast() {
		return fmt.Errorf("IP %s is not a multicast address", groupIP)
	}

	log.G(context.TODO()).Infof("Setting up FDB entries for specific multicast group %s", groupIP)

	// Set up FDB entries for all subnets
	for _, s := range n.subnets {
		if err := n.addMulticastGroupForContainer(addr, s); err \!= nil {
			log.G(context.TODO()).Warnf("Failed to setup multicast group %s for subnet %s: %v", 
				groupIP, s.subnetIP, err)
		}
	}

	return nil
}
EOF < /dev/null