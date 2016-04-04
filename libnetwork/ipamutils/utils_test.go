package ipamutils

import (
	"net"
	"testing"

	"github.com/docker/libnetwork/testutils"
	"github.com/docker/libnetwork/types"
	"github.com/vishvananda/netlink"
)

func init() {
	InitNetworks()
}

func TestGranularPredefined(t *testing.T) {
	for _, nw := range PredefinedGranularNetworks {
		if ones, bits := nw.Mask.Size(); bits != 32 || ones != 24 {
			t.Fatalf("Unexpected size for network in granular list: %v", nw)
		}
	}

	for _, nw := range PredefinedBroadNetworks {
		if ones, bits := nw.Mask.Size(); bits != 32 || (ones != 20 && ones != 16) {
			t.Fatalf("Unexpected size for network in broad list: %v", nw)
		}
	}

}

func TestNetworkRequest(t *testing.T) {
	defer testutils.SetupTestOSContext(t)()
	_, exp, err := net.ParseCIDR("172.17.0.0/16")
	if err != nil {
		t.Fatal(err)
	}

	nw, err := FindAvailableNetwork(PredefinedBroadNetworks)
	if err != nil {
		t.Fatal(err)
	}
	if !types.CompareIPNet(exp, nw) {
		t.Fatalf("exected %s. got %s", exp, nw)
	}

	_, exp, err = net.ParseCIDR("10.0.0.0/24")
	if err != nil {
		t.Fatal(err)
	}
	nw, err = FindAvailableNetwork(PredefinedGranularNetworks)
	if err != nil {
		t.Fatal(err)
	}
	if !types.CompareIPNet(exp, nw) {
		t.Fatalf("exected %s. got %s", exp, nw)
	}

	// Add iface and ssert returned address on request
	createInterface(t, "test", "172.17.42.1/16")

	_, exp, err = net.ParseCIDR("172.18.0.0/16")
	if err != nil {
		t.Fatal(err)
	}
	nw, err = FindAvailableNetwork(PredefinedBroadNetworks)
	if err != nil {
		t.Fatal(err)
	}
	if !types.CompareIPNet(exp, nw) {
		t.Fatalf("exected %s. got %s", exp, nw)
	}
}

func TestElectInterfaceAddress(t *testing.T) {
	defer testutils.SetupTestOSContext(t)()
	nws := "172.101.202.254/16"
	createInterface(t, "test", nws)

	ipv4Nw, ipv6Nw, err := ElectInterfaceAddresses("test")
	if err != nil {
		t.Fatal(err)
	}

	if ipv4Nw == nil {
		t.Fatalf("unexpected empty ipv4 network addresses")
	}

	if len(ipv6Nw) == 0 {
		t.Fatalf("unexpected empty ipv4 network addresses")
	}

	if nws != ipv4Nw.String() {
		t.Fatalf("expected %s. got %s", nws, ipv4Nw)
	}
}

func createInterface(t *testing.T, name, nw string) {
	// Add interface
	link := &netlink.Bridge{
		LinkAttrs: netlink.LinkAttrs{
			Name: "test",
		},
	}
	bip, err := types.ParseCIDR(nw)
	if err != nil {
		t.Fatal(err)
	}
	if err = netlink.LinkAdd(link); err != nil {
		t.Fatalf("Failed to create interface via netlink: %v", err)
	}
	if err := netlink.AddrAdd(link, &netlink.Addr{IPNet: bip}); err != nil {
		t.Fatal(err)
	}
	if err = netlink.LinkSetUp(link); err != nil {
		t.Fatal(err)
	}
}
