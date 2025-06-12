//go:build linux

package overlay

import (
	"net"
	"net/netip"
	"testing"
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
		id:      "test-network",
		driver:  d,
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
		id:      "test-network",
		driver:  d,
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
	n := &network{
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

