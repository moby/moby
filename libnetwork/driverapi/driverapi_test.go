package driverapi

import (
	"net"
	"testing"
)

func TestInterfaceEqual(t *testing.T) {
	list := getInterfaceList()

	if !list[0].Equal(list[0]) {
		t.Fatalf("Interface.Equal() returned false negative")
	}

	if list[0].Equal(list[1]) {
		t.Fatalf("Interface.Equal() returned false positive")
	}

	if list[0].Equal(list[1]) != list[1].Equal(list[0]) {
		t.Fatalf("Interface.Equal() failed commutative check")
	}
}

func TestInterfaceCopy(t *testing.T) {
	for _, iface := range getInterfaceList() {
		cp := iface.GetCopy()

		if !iface.Equal(cp) {
			t.Fatalf("Failed to return a copy of Interface")
		}

		if iface == cp {
			t.Fatalf("Failed to return a true copy of Interface")
		}
	}
}

func TestSandboxInfoCopy(t *testing.T) {
	si := SandboxInfo{Interfaces: getInterfaceList(), Gateway: net.ParseIP("192.168.1.254"), GatewayIPv6: net.ParseIP("2001:2345::abcd:8889")}
	cp := si.GetCopy()

	if !si.Equal(cp) {
		t.Fatalf("Failed to return a copy of SandboxInfo")
	}

	if &si == cp {
		t.Fatalf("Failed to return a true copy of SanboxInfo")
	}
}

func getInterfaceList() []*Interface {
	_, netv4a, _ := net.ParseCIDR("192.168.30.1/24")
	_, netv4b, _ := net.ParseCIDR("172.18.255.2/23")
	_, netv6a, _ := net.ParseCIDR("2001:2345::abcd:8888/80")
	_, netv6b, _ := net.ParseCIDR("2001:2345::abcd:8889/80")

	return []*Interface{
		&Interface{
			SrcName:     "veth1234567",
			DstName:     "eth0",
			Address:     netv4a,
			AddressIPv6: netv6a,
		},
		&Interface{
			SrcName:     "veth7654321",
			DstName:     "eth1",
			Address:     netv4b,
			AddressIPv6: netv6b,
		},
	}
}
