package ipallocator

import (
	"fmt"
	"net"
	"testing"
)

func reset() {
	allocatedIPs = networkSet{}
	availableIPS = networkSet{}
}

func TestRegisterNetwork(t *testing.T) {
	defer reset()
	network := &net.IPNet{
		IP:   []byte{192, 168, 0, 1},
		Mask: []byte{255, 255, 255, 0},
	}

	if err := RegisterNetwork(network); err != nil {
		t.Fatal(err)
	}

	n := newIPNet(network)
	if _, exists := allocatedIPs[n]; !exists {
		t.Fatal("IPNet should exist in allocated IPs")
	}

	if _, exists := availableIPS[n]; !exists {
		t.Fatal("IPNet should exist in available IPs")
	}
}

func TestRegisterTwoNetworks(t *testing.T) {
	defer reset()
	network := &net.IPNet{
		IP:   []byte{192, 168, 0, 1},
		Mask: []byte{255, 255, 255, 0},
	}

	if err := RegisterNetwork(network); err != nil {
		t.Fatal(err)
	}

	network2 := &net.IPNet{
		IP:   []byte{10, 1, 42, 1},
		Mask: []byte{255, 255, 255, 0},
	}

	if err := RegisterNetwork(network2); err != nil {
		t.Fatal(err)
	}
}

func TestRegisterNetworkThatExists(t *testing.T) {
	defer reset()
	network := &net.IPNet{
		IP:   []byte{192, 168, 0, 1},
		Mask: []byte{255, 255, 255, 0},
	}

	if err := RegisterNetwork(network); err != nil {
		t.Fatal(err)
	}

	if err := RegisterNetwork(network); err != ErrNetworkAlreadyRegisterd {
		t.Fatalf("Expected error of %s got %s", ErrNetworkAlreadyRegisterd, err)
	}
}

func TestRequestNewIps(t *testing.T) {
	defer reset()
	network := &net.IPNet{
		IP:   []byte{192, 168, 0, 1},
		Mask: []byte{255, 255, 255, 0},
	}

	if err := RegisterNetwork(network); err != nil {
		t.Fatal(err)
	}

	for i := 2; i < 10; i++ {
		ip, err := RequestIP(network, nil)
		if err != nil {
			t.Fatal(err)
		}

		if expected := fmt.Sprintf("192.168.0.%d", i); ip.String() != expected {
			t.Fatalf("Expected ip %s got %s", expected, ip.String())
		}
	}
}

func TestReleaseIp(t *testing.T) {
	defer reset()
	network := &net.IPNet{
		IP:   []byte{192, 168, 0, 1},
		Mask: []byte{255, 255, 255, 0},
	}

	if err := RegisterNetwork(network); err != nil {
		t.Fatal(err)
	}

	ip, err := RequestIP(network, nil)
	if err != nil {
		t.Fatal(err)
	}

	if err := ReleaseIP(network, ip); err != nil {
		t.Fatal(err)
	}
}

func TestGetReleasedIp(t *testing.T) {
	defer reset()
	network := &net.IPNet{
		IP:   []byte{192, 168, 0, 1},
		Mask: []byte{255, 255, 255, 0},
	}

	if err := RegisterNetwork(network); err != nil {
		t.Fatal(err)
	}

	ip, err := RequestIP(network, nil)
	if err != nil {
		t.Fatal(err)
	}

	value := ip.String()
	if err := ReleaseIP(network, ip); err != nil {
		t.Fatal(err)
	}

	ip, err = RequestIP(network, nil)
	if err != nil {
		t.Fatal(err)
	}

	if ip.String() != value {
		t.Fatalf("Expected to receive same ip %s got %s", value, ip.String())
	}
}

func TestRequesetSpecificIp(t *testing.T) {
	defer reset()
	network := &net.IPNet{
		IP:   []byte{192, 168, 0, 1},
		Mask: []byte{255, 255, 255, 0},
	}

	if err := RegisterNetwork(network); err != nil {
		t.Fatal(err)
	}

	ip := net.ParseIP("192.168.1.5")

	if _, err := RequestIP(network, &ip); err != nil {
		t.Fatal(err)
	}
}
