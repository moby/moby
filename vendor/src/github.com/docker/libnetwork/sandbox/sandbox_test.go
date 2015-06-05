package sandbox

import (
	"net"
	"os"
	"testing"

	"github.com/docker/docker/pkg/reexec"
)

func TestMain(m *testing.M) {
	if reexec.Init() {
		return
	}
	os.Exit(m.Run())
}

func TestSandboxCreate(t *testing.T) {
	key, err := newKey(t)
	if err != nil {
		t.Fatalf("Failed to obtain a key: %v", err)
	}

	s, err := NewSandbox(key, true)
	if err != nil {
		t.Fatalf("Failed to create a new sandbox: %v", err)
	}

	if s.Key() != key {
		t.Fatalf("s.Key() returned %s. Expected %s", s.Key(), key)
	}

	info, err := newInfo(t)
	if err != nil {
		t.Fatalf("Failed to generate new sandbox info: %v", err)
	}

	for _, i := range info.Interfaces {
		err = s.AddInterface(i)
		if err != nil {
			t.Fatalf("Failed to add interfaces to sandbox: %v", err)
		}
	}

	err = s.SetGateway(info.Gateway)
	if err != nil {
		t.Fatalf("Failed to set gateway to sandbox: %v", err)
	}

	err = s.SetGatewayIPv6(info.GatewayIPv6)
	if err != nil {
		t.Fatalf("Failed to set ipv6 gateway to sandbox: %v", err)
	}

	verifySandbox(t, s)
	s.Destroy()
	verifyCleanup(t, s, true)
}

func TestSandboxCreateTwice(t *testing.T) {
	key, err := newKey(t)
	if err != nil {
		t.Fatalf("Failed to obtain a key: %v", err)
	}

	_, err = NewSandbox(key, true)
	if err != nil {
		t.Fatalf("Failed to create a new sandbox: %v", err)
	}

	// Create another sandbox with the same key to see if we handle it
	// gracefully.
	s, err := NewSandbox(key, true)
	if err != nil {
		t.Fatalf("Failed to create a new sandbox: %v", err)
	}
	s.Destroy()
}

func TestSandboxGC(t *testing.T) {
	key, err := newKey(t)
	if err != nil {
		t.Fatalf("Failed to obtain a key: %v", err)
	}

	s, err := NewSandbox(key, true)
	if err != nil {
		t.Fatalf("Failed to create a new sandbox: %v", err)
	}

	s.Destroy()

	GC()
	verifyCleanup(t, s, false)
}

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

func TestSandboxInfoEqual(t *testing.T) {
	si1 := &Info{Interfaces: getInterfaceList(), Gateway: net.ParseIP("192.168.1.254"), GatewayIPv6: net.ParseIP("2001:2345::abcd:8889")}
	si2 := &Info{Interfaces: getInterfaceList(), Gateway: net.ParseIP("172.18.255.254"), GatewayIPv6: net.ParseIP("2001:2345::abcd:8888")}

	if !si1.Equal(si1) {
		t.Fatalf("Info.Equal() returned false negative")
	}

	if si1.Equal(si2) {
		t.Fatalf("Info.Equal() returned false positive")
	}

	if si1.Equal(si2) != si2.Equal(si1) {
		t.Fatalf("Info.Equal() failed commutative check")
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
	si := Info{Interfaces: getInterfaceList(), Gateway: net.ParseIP("192.168.1.254"), GatewayIPv6: net.ParseIP("2001:2345::abcd:8889")}
	cp := si.GetCopy()

	if !si.Equal(cp) {
		t.Fatalf("Failed to return a copy of Info")
	}

	if &si == cp {
		t.Fatalf("Failed to return a true copy of Info")
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
