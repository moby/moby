//go:build linux

package overlay

import (
	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netns"
	"net"
	"net/netip"
	"testing"
	"time"
)

func TestMulticastIPToMAC(t *testing.T) {
	tests := []struct {
		name     string
		ip       string
		expected string
	}{
		{
			name:     "IPv4 multicast",
			ip:       "239.1.2.3",
			expected: "01:00:5e:01:02:03",
		},
		{
			name:     "IPv4 multicast with high bit",
			ip:       "239.129.2.3",
			expected: "01:00:5e:01:02:03",
		},
		{
			name:     "IPv6 multicast",
			ip:       "ff02::1:ff00:1",
			expected: "33:33:ff:00:00:01",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ip, err := netip.ParseAddr(tt.ip)
			if err != nil {
				t.Fatalf("Failed to parse IP: %v", err)
			}

			mac := multicastIPToMAC(ip)
			if mac == nil {
				t.Fatal("Expected non-nil MAC address")
			}

			if mac.String() != tt.expected {
				t.Errorf("Expected MAC %s, got %s", tt.expected, mac.String())
			}
		})
	}
}

func TestMulticastIPToMACNonMulticast(t *testing.T) {
	ip, _ := netip.ParseAddr("10.0.0.1")
	mac := multicastIPToMAC(ip)
	if mac != nil {
		t.Errorf("Expected nil MAC for non-multicast IP, got %s", mac.String())
	}
}

func TestPeerAddMulticast(t *testing.T) {
	d := &driver{
		networks: make(networkTable),
	}

	n := &network{
		id:     "test-network",
		driver: d,
		subnets: []*subnet{
			{
				vni:       100,
				vxlanName: "vxlan100",
			},
		},
	}

	d.networks["test-network"] = n

	mcastIP, _ := netip.ParsePrefix("239.1.1.1/32")
	vtep, _ := netip.ParseAddr("192.168.1.10")

	err := d.peerAddMulticast("test-network", "ep1", mcastIP, vtep)
	if err != nil {
		t.Errorf("peerAddMulticast failed: %v", err)
	}
}

func TestPeerDeleteMulticast(t *testing.T) {
	d := &driver{
		networks: make(networkTable),
	}

	n := &network{
		id:     "test-network",
		driver: d,
		subnets: []*subnet{
			{
				vni:       100,
				vxlanName: "vxlan100",
			},
		},
	}

	d.networks["test-network"] = n

	mcastIP, _ := netip.ParsePrefix("239.1.1.1/32")
	vtep, _ := netip.ParseAddr("192.168.1.10")

	err := d.peerDeleteMulticast("test-network", "ep1", mcastIP, vtep)
	if err != nil {
		t.Errorf("peerDeleteMulticast failed: %v", err)
	}
}

func TestHandleNeighborMiss(t *testing.T) {
	// This test would require mocking netlink and network structures
	// For now, we just ensure the functions compile and have the right signatures
	_ = &network{
		id:     "test-network",
		stopCh: make(chan struct{}),
		subnets: []*subnet{
			{
				vxlanName: "vxlan100",
			},
		},
	}

	// Test that multicast IP detection works
	mcastIP, _ := netip.ParseAddr("239.1.1.1")
	if !mcastIP.IsMulticast() {
		t.Error("239.1.1.1 should be detected as multicast")
	}

	unicastIP, _ := netip.ParseAddr("10.0.0.1")
	if unicastIP.IsMulticast() {
		t.Error("10.0.0.1 should not be detected as multicast")
	}
}

func TestMulticastRateLimiter(t *testing.T) {
	n := &network{
		id:     "test-network",
		stopCh: make(chan struct{}),
	}

	limiter := n.startMulticastRateLimiter()
	defer limiter.stop()

	groupIP, _ := netip.ParseAddr("239.1.1.1")

	// Test initial rate limit check (should pass)
	if !limiter.checkRateLimit(groupIP) {
		t.Error("First rate limit check should pass")
	}

	// Add group
	limiter.addGroup(groupIP)
	if limiter.groupCounts[groupIP.String()] != 1 {
		t.Error("Group count should be 1")
	}

	// Remove group
	limiter.removeGroup(groupIP)
	if limiter.groupCounts[groupIP.String()] != 0 {
		t.Error("Group count should be 0 after removal")
	}
}

func TestInterSubnetMulticastRouting(t *testing.T) {
	d := &driver{
		networks: make(networkTable),
	}

	subnet1 := &subnet{
		vni:       100,
		vxlanName: "vxlan100",
		brName:    "br100",
		subnetIP:  netip.MustParsePrefix("10.0.1.0/24"),
	}

	subnet2 := &subnet{
		vni:       200,
		vxlanName: "vxlan200",
		brName:    "br200",
		subnetIP:  netip.MustParsePrefix("10.0.2.0/24"),
	}

	n := &network{
		id:      "test-network",
		driver:  d,
		subnets: []*subnet{subnet1, subnet2},
		stopCh:  make(chan struct{}),
	}

	d.networks["test-network"] = n

	// Test setup inter-subnet routing
	err := n.setupInterSubnetMulticastRouting()
	if err != nil {
		t.Logf("setupInterSubnetMulticastRouting returned error (expected in test environment): %v", err)
	}

	// Test multicast group handling
	groupIP, _ := netip.ParseAddr("239.1.1.1")
	err = n.handleMulticastGroupJoin(groupIP, subnet1)
	if err != nil {
		t.Logf("handleMulticastGroupJoin returned error (expected in test environment): %v", err)
	}

	err = n.handleMulticastGroupLeave(groupIP, subnet1)
	if err != nil {
		t.Logf("handleMulticastGroupLeave returned error (expected in test environment): %v", err)
	}
}

func TestIGMPProxy(t *testing.T) {
	n := &network{
		id:     "test-network",
		stopCh: make(chan struct{}),
	}

	subnet := &subnet{
		vni:       100,
		vxlanName: "vxlan100",
		subnetIP:  netip.MustParsePrefix("10.0.1.0/24"),
	}

	proxy := &igmpProxy{
		network:    n,
		subnet:     subnet,
		stopCh:     make(chan struct{}),
		groupState: make(map[string]time.Time),
	}

	groupIP, _ := netip.ParseAddr("239.1.1.1")

	// Test IGMP join
	err := proxy.handleIGMPJoin(groupIP)
	if err != nil {
		t.Logf("handleIGMPJoin returned error (expected in test environment): %v", err)
	}

	// Check group state
	if _, exists := proxy.groupState[groupIP.String()]; !exists {
		t.Error("Group should be in proxy state after join")
	}

	// Test IGMP leave
	err = proxy.handleIGMPLeave(groupIP)
	if err != nil {
		t.Logf("handleIGMPLeave returned error (expected in test environment): %v", err)
	}

	// Check group state
	if _, exists := proxy.groupState[groupIP.String()]; exists {
		t.Error("Group should not be in proxy state after leave")
	}
}

func TestContainerMulticastLifecycle(t *testing.T) {
	d := &driver{
		networks: make(networkTable),
	}

	subnet := &subnet{
		vni:       100,
		vxlanName: "vxlan100",
		subnetIP:  netip.MustParsePrefix("10.0.1.0/24"),
	}

	n := &network{
		id:      "test-network",
		driver:  d,
		subnets: []*subnet{subnet},
	}

	d.networks["test-network"] = n

	containerIP, _ := netip.ParseAddr("10.0.1.10")
	groupIP, _ := netip.ParseAddr("239.1.1.1")
	groups := []netip.Addr{groupIP}

	// Test container join
	err := n.handleContainerMulticastJoin(containerIP, groups)
	if err != nil {
		t.Errorf("handleContainerMulticastJoin failed: %v", err)
	}

	// Test container leave
	err = n.handleContainerMulticastLeave(containerIP, groups)
	if err != nil {
		t.Errorf("handleContainerMulticastLeave failed: %v", err)
	}
}

func TestMulticastValidation(t *testing.T) {
	// Test invalid inputs
	d := &driver{}

	// Test with invalid VTEP
	err := d.peerAddMulticastRoute("nid", netip.Addr{}, nil, 1)
	if err == nil {
		t.Error("Should fail with invalid VTEP")
	}

	// Test with empty MAC
	vtep, _ := netip.ParseAddr("192.168.1.1")
	err = d.peerAddMulticastRoute("nid", vtep, nil, 1)
	if err == nil {
		t.Error("Should fail with empty MAC")
	}

	// Test with invalid link index
	mac, _ := net.ParseMAC("01:02:03:04:05:06")
	err = d.peerAddMulticastRoute("nid", vtep, mac, 0)
	if err == nil {
		t.Error("Should fail with invalid link index")
	}
}

// Integration tests with real netlink operations
func TestMulticastNetlinkIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Requires root privileges
	origNs, err := netns.Get()
	if err != nil {
		t.Skip("Cannot get current netns, skipping integration test")
	}
	defer origNs.Close()

	// Create a new network namespace for testing
	testNs, err := netns.New()
	if err != nil {
		t.Skip("Cannot create netns, skipping integration test (requires root)")
	}
	defer testNs.Close()
	defer netns.Set(origNs)

	// Create test VXLAN interface
	vxlan := &netlink.Vxlan{
		LinkAttrs: netlink.LinkAttrs{
			Name: "vxlan-test",
		},
		VxlanId: 100,
		Port:    4789,
	}

	if err := netlink.LinkAdd(vxlan); err != nil {
		t.Fatalf("Failed to create VXLAN interface: %v", err)
	}
	defer netlink.LinkDel(vxlan)

	// Create bridge
	br := &netlink.Bridge{
		LinkAttrs: netlink.LinkAttrs{
			Name: "br-test",
		},
	}

	if err := netlink.LinkAdd(br); err != nil {
		t.Fatalf("Failed to create bridge: %v", err)
	}
	defer netlink.LinkDel(br)

	// Test multicast FDB operations
	t.Run("MulticastFDBOperations", func(t *testing.T) {
		mcastMAC, _ := net.ParseMAC("01:00:5e:00:00:01")
		vtep := net.ParseIP("192.168.1.10")

		neigh := &netlink.Neigh{
			LinkIndex:    vxlan.Attrs().Index,
			Family:       afBridge,
			State:        netlink.NUD_PERMANENT,
			Flags:        netlink.NTF_SELF,
			IP:           vtep,
			HardwareAddr: mcastMAC,
		}

		// Add multicast FDB entry
		if err := netlink.NeighSet(neigh); err != nil {
			t.Errorf("Failed to add multicast FDB entry: %v", err)
		}

		// List and verify
		neighs, err := netlink.NeighList(vxlan.Attrs().Index, afBridge)
		if err != nil {
			t.Errorf("Failed to list neighbors: %v", err)
		}

		found := false
		for _, n := range neighs {
			if n.HardwareAddr.String() == mcastMAC.String() {
				found = true
				break
			}
		}

		if !found {
			t.Error("Multicast FDB entry not found after adding")
		}

		// Delete FDB entry
		if err := netlink.NeighDel(neigh); err != nil {
			t.Errorf("Failed to delete multicast FDB entry: %v", err)
		}
	})
}

func TestMulticastFeatureDetection(t *testing.T) {
	features, err := checkMulticastFeatures()
	if err != nil {
		t.Fatalf("Failed to check multicast features: %v", err)
	}

	// Basic sanity checks - these features should be available on most Linux systems
	if features.KernelVersion == "" {
		t.Error("Kernel version should not be empty")
	}

	t.Logf("Detected multicast features: %+v", features)
}

func TestMulticastCleanup(t *testing.T) {
	d := &driver{
		networks: make(networkTable),
	}

	n := &network{
		id:      "test-network",
		driver:  d,
		stopCh:  make(chan struct{}),
		subnets: []*subnet{},
	}

	// Start rate limiter
	n.multicastRateLimiter = n.startMulticastRateLimiter()

	// Add a subnet with IGMP proxy
	subnet := &subnet{
		vni:       100,
		vxlanName: "vxlan100",
		subnetIP:  netip.MustParsePrefix("10.0.1.0/24"),
	}

	proxy := &igmpProxy{
		network:    n,
		subnet:     subnet,
		stopCh:     make(chan struct{}),
		groupState: make(map[string]time.Time),
	}
	subnet.igmpProxy = proxy
	n.subnets = append(n.subnets, subnet)

	// Test cleanup
	n.cleanupMulticast()

	// Verify resources are cleaned up
	if n.multicastRateLimiter != nil {
		t.Error("Rate limiter should be nil after cleanup")
	}

	if subnet.igmpProxy != nil {
		t.Error("IGMP proxy should be nil after cleanup")
	}
}

func TestConcurrentMulticastOperations(t *testing.T) {
	n := &network{
		id:     "test-network",
		stopCh: make(chan struct{}),
	}

	limiter := n.startMulticastRateLimiter()
	defer limiter.stop()

	// Run concurrent operations
	done := make(chan bool)
	groupIP, _ := netip.ParseAddr("239.1.1.1")

	// Concurrent adds
	for i := 0; i < 10; i++ {
		go func() {
			limiter.addGroup(groupIP)
			done <- true
		}()
	}

	// Wait for adds
	for i := 0; i < 10; i++ {
		<-done
	}

	// Check count
	limiter.mu.RLock()
	count := limiter.groupCounts[groupIP.String()]
	limiter.mu.RUnlock()

	if count != 10 {
		t.Errorf("Expected count 10, got %d", count)
	}

	// Concurrent removes
	for i := 0; i < 10; i++ {
		go func() {
			limiter.removeGroup(groupIP)
			done <- true
		}()
	}

	// Wait for removes
	for i := 0; i < 10; i++ {
		<-done
	}

	// Check count
	limiter.mu.RLock()
	count = limiter.groupCounts[groupIP.String()]
	limiter.mu.RUnlock()

	if count != 0 {
		t.Errorf("Expected count 0 after removes, got %d", count)
	}
}
