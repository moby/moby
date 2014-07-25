package netlink

import (
	"net"
	"testing"
)

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
		t.Fatal("expected error getting interface because bridge was deleted")
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
