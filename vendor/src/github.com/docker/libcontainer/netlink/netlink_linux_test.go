package netlink

import (
	"net"
	"strings"
	"testing"
)

func ipAssigned(iface *net.Interface, ip net.IP) bool {
	addrs, _ := iface.Addrs()

	for _, addr := range addrs {
		args := strings.SplitN(addr.String(), "/", 2)
		if args[0] == ip.String() {
			return true
		}
	}

	return false
}

func TestAddDelNetworkIp(t *testing.T) {
	if testing.Short() {
		return
	}

	ifaceName := "lo"
	ip := net.ParseIP("127.0.1.1")
	mask := net.IPv4Mask(255, 255, 255, 255)
	ipNet := &net.IPNet{IP: ip, Mask: mask}

	iface, err := net.InterfaceByName(ifaceName)
	if err != nil {
		t.Skip("No 'lo' interface; skipping tests")
	}

	if err := NetworkLinkAddIp(iface, ip, ipNet); err != nil {
		t.Fatal(err)
	}

	if !ipAssigned(iface, ip) {
		t.Fatalf("Could not locate address '%s' in lo address list.", ip.String())
	}

	if err := NetworkLinkDelIp(iface, ip, ipNet); err != nil {
		t.Fatal(err)
	}

	if ipAssigned(iface, ip) {
		t.Fatalf("Located address '%s' in lo address list after removal.", ip.String())
	}
}

func TestCreateBridgeWithMac(t *testing.T) {
	if testing.Short() {
		return
	}

	name := "testbridge"

	if err := CreateBridge(name, true); err != nil {
		t.Fatal(err)
	}

	if _, err := net.InterfaceByName(name); err != nil {
		t.Fatal(err)
	}

	// cleanup and tests

	if err := DeleteBridge(name); err != nil {
		t.Fatal(err)
	}

	if _, err := net.InterfaceByName(name); err == nil {
		t.Fatalf("expected error getting interface because %s bridge was deleted", name)
	}
}

func TestCreateBridgeLink(t *testing.T) {
	if testing.Short() {
		return
	}

	name := "mybrlink"

	if err := NetworkLinkAdd(name, "bridge"); err != nil {
		t.Fatal(err)
	}

	if _, err := net.InterfaceByName(name); err != nil {
		t.Fatal(err)
	}

	if err := NetworkLinkDel(name); err != nil {
		t.Fatal(err)
	}

	if _, err := net.InterfaceByName(name); err == nil {
		t.Fatalf("expected error getting interface because %s bridge was deleted", name)
	}

}

func TestCreateVethPair(t *testing.T) {
	if testing.Short() {
		return
	}

	var (
		name1 = "veth1"
		name2 = "veth2"
	)

	if err := NetworkCreateVethPair(name1, name2); err != nil {
		t.Fatal(err)
	}

	if _, err := net.InterfaceByName(name1); err != nil {
		t.Fatal(err)
	}

	if _, err := net.InterfaceByName(name2); err != nil {
		t.Fatal(err)
	}
}
