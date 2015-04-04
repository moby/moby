package bridge

import (
	"fmt"
	"net"
	"strconv"
	"testing"

	"github.com/docker/docker/daemon/network"
	"github.com/docker/docker/daemon/networkdriver/portmapper"
	"github.com/docker/docker/nat"
	"github.com/docker/docker/pkg/iptables"
)

func init() {
	// reset the new proxy command for mocking out the userland proxy in tests
	portmapper.NewProxy = portmapper.NewMockProxyCommand
}

func findFreePort(t *testing.T) string {
	l, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Fatal("Failed to find a free port")
	}
	defer l.Close()

	result, err := net.ResolveTCPAddr("tcp", l.Addr().String())
	if err != nil {
		t.Fatal("Failed to resolve address to identify free port")
	}
	return strconv.Itoa(result.Port)
}

func TestAllocatePortDetection(t *testing.T) {
	freePort := findFreePort(t)

	if err := InitDriver(new(Config)); err != nil {
		t.Fatal("Failed to initialize network driver")
	}

	// Allocate interface
	if _, err := Allocate("container_id", "", "", ""); err != nil {
		t.Fatal("Failed to allocate network interface")
	}

	port := nat.Port(freePort + "/tcp")
	binding := nat.PortBinding{HostIp: "127.0.0.1", HostPort: freePort}

	// Allocate same port twice, expect failure on second call
	if _, err := AllocatePort("container_id", port, binding); err != nil {
		t.Fatal("Failed to find a free port to allocate")
	}
	if _, err := AllocatePort("container_id", port, binding); err == nil {
		t.Fatal("Duplicate port allocation granted by AllocatePort")
	}
}

func TestHostnameFormatChecking(t *testing.T) {
	freePort := findFreePort(t)

	if err := InitDriver(new(Config)); err != nil {
		t.Fatal("Failed to initialize network driver")
	}

	// Allocate interface
	if _, err := Allocate("container_id", "", "", ""); err != nil {
		t.Fatal("Failed to allocate network interface")
	}

	port := nat.Port(freePort + "/tcp")
	binding := nat.PortBinding{HostIp: "localhost", HostPort: freePort}

	if _, err := AllocatePort("container_id", port, binding); err == nil {
		t.Fatal("Failed to check invalid HostIP")
	}
}

func newInterfaceAllocation(t *testing.T, globalIPv6 *net.IPNet, requestedMac, requestedIP, requestedIPv6 string, expectFail bool) *network.Settings {
	// set IPv6 global if given
	if globalIPv6 != nil {
		globalIPv6Network = globalIPv6
	}

	networkSettings, err := Allocate("container_id", requestedMac, requestedIP, requestedIPv6)
	if err == nil && expectFail {
		t.Fatal("Doesn't fail to allocate network interface")
	} else if err != nil && !expectFail {
		t.Fatal("Failed to allocate network interface")

	}

	if globalIPv6 != nil {
		// check for bug #11427
		if globalIPv6Network.IP.String() != globalIPv6.IP.String() {
			t.Fatal("globalIPv6Network was modified during allocation")
		}
		// clean up IPv6 global
		globalIPv6Network = nil
	}

	return networkSettings
}

func TestIPv6InterfaceAllocationAutoNetmaskGt80(t *testing.T) {
	_, subnet, _ := net.ParseCIDR("2001:db8:1234:1234:1234::/81")
	networkSettings := newInterfaceAllocation(t, subnet, "", "", "", false)

	// ensure low manually assigend global ip
	ip := net.ParseIP(networkSettings.GlobalIPv6Address)
	_, subnet, _ = net.ParseCIDR(fmt.Sprintf("%s/%d", subnet.IP.String(), 120))
	if !subnet.Contains(ip) {
		t.Fatalf("Error ip %s not in subnet %s", ip.String(), subnet.String())
	}
}

func TestIPv6InterfaceAllocationAutoNetmaskLe80(t *testing.T) {
	_, subnet, _ := net.ParseCIDR("2001:db8:1234:1234:1234::/80")
	networkSettings := newInterfaceAllocation(t, subnet, "ab:cd:ab:cd:ab:cd", "", "", false)

	// ensure global ip with mac
	ip := net.ParseIP(networkSettings.GlobalIPv6Address)
	expectedIP := net.ParseIP("2001:db8:1234:1234:1234:abcd:abcd:abcd")
	if ip.String() != expectedIP.String() {
		t.Fatalf("Error ip %s should be %s", ip.String(), expectedIP.String())
	}

	// ensure link local format
	ip = net.ParseIP(networkSettings.LinkLocalIPv6Address)
	expectedIP = net.ParseIP("fe80::a9cd:abff:fecd:abcd")
	if ip.String() != expectedIP.String() {
		t.Fatalf("Error ip %s should be %s", ip.String(), expectedIP.String())
	}

}

func TestIPv6InterfaceAllocationRequest(t *testing.T) {
	_, subnet, _ := net.ParseCIDR("2001:db8:1234:1234:1234::/80")
	expectedIP := "2001:db8:1234:1234:1234::1328"

	networkSettings := newInterfaceAllocation(t, subnet, "", "", expectedIP, false)

	// ensure global ip with mac
	ip := net.ParseIP(networkSettings.GlobalIPv6Address)
	if ip.String() != expectedIP {
		t.Fatalf("Error ip %s should be %s", ip.String(), expectedIP)
	}

	// retry -> fails for duplicated address
	_ = newInterfaceAllocation(t, subnet, "", "", expectedIP, true)
}

func TestMacAddrGeneration(t *testing.T) {
	ip := net.ParseIP("192.168.0.1")
	mac := generateMacAddr(ip).String()

	// Should be consistent.
	if generateMacAddr(ip).String() != mac {
		t.Fatal("Inconsistent MAC address")
	}

	// Should be unique.
	ip2 := net.ParseIP("192.168.0.2")
	if generateMacAddr(ip2).String() == mac {
		t.Fatal("Non-unique MAC address")
	}
}

func TestLinkContainers(t *testing.T) {
	// Init driver
	if err := InitDriver(new(Config)); err != nil {
		t.Fatal("Failed to initialize network driver")
	}

	// Allocate interface
	if _, err := Allocate("container_id", "", "", ""); err != nil {
		t.Fatal("Failed to allocate network interface")
	}

	bridgeIface = "lo"
	if _, err := iptables.NewChain("DOCKER", bridgeIface, iptables.Filter); err != nil {
		t.Fatal(err)
	}

	if err := LinkContainers("-I", "172.17.0.1", "172.17.0.2", []nat.Port{nat.Port("1234")}, false); err != nil {
		t.Fatal("LinkContainers failed")
	}

	// flush rules
	if _, err := iptables.Raw([]string{"-F", "DOCKER"}...); err != nil {
		t.Fatal(err)
	}

}
