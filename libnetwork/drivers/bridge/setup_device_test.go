package bridge

import (
	"bytes"
	"net"
	"strings"
	"testing"

	"github.com/docker/libnetwork"
	"github.com/vishvananda/netlink"
)

func TestSetupNewBridge(t *testing.T) {
	defer libnetwork.SetupTestNetNS(t)()

	br := &bridgeInterface{
		Config: &Configuration{
			BridgeName: DefaultBridgeName,
		},
	}
	if err := setupDevice(br); err != nil {
		t.Fatalf("Bridge creation failed: %v", err)
	}
	if br.Link == nil {
		t.Fatal("bridgeInterface link is nil (expected valid link)")
	}
	if _, err := netlink.LinkByName(DefaultBridgeName); err != nil {
		t.Fatalf("Failed to retrieve bridge device: %v", err)
	}
	if br.Link.Attrs().Flags&net.FlagUp == net.FlagUp {
		t.Fatalf("bridgeInterface should be created down")
	}
}

func TestSetupNewNonDefaultBridge(t *testing.T) {
	defer libnetwork.SetupTestNetNS(t)()

	br := &bridgeInterface{
		Config: &Configuration{
			BridgeName: "test0",
		},
	}
	if err := setupDevice(br); err == nil || !strings.Contains(err.Error(), "non default name") {
		t.Fatalf("Expected bridge creation failure with \"non default name\", got: %v", err)
	}
}

func TestSetupDeviceUp(t *testing.T) {
	defer libnetwork.SetupTestNetNS(t)()

	br := &bridgeInterface{
		Config: &Configuration{
			BridgeName: DefaultBridgeName,
		},
	}
	if err := setupDevice(br); err != nil {
		t.Fatalf("Bridge creation failed: %v", err)
	}
	if err := setupDeviceUp(br); err != nil {
		t.Fatalf("Failed to up bridge device: %v", err)
	}

	lnk, _ := netlink.LinkByName(DefaultBridgeName)
	if lnk.Attrs().Flags&net.FlagUp != net.FlagUp {
		t.Fatalf("bridgeInterface should be up")
	}
}

func TestGenerateRandomMAC(t *testing.T) {
	defer libnetwork.SetupTestNetNS(t)()

	mac1 := libnetwork.GenerateRandomMAC()
	mac2 := libnetwork.GenerateRandomMAC()
	if bytes.Compare(mac1, mac2) == 0 {
		t.Fatalf("Generated twice the same MAC address %v", mac1)
	}
}
