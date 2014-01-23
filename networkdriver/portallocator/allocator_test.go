package ipallocator

import (
	"net"
	"testing"
)

func TestRegisterNetwork(t *testing.T) {
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
