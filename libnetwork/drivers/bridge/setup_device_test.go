package bridge

import (
	"bytes"
	"net"
	"testing"

	"github.com/docker/libnetwork/netutils"
	"github.com/vishvananda/netlink"
)

func TestSetupNewBridge(t *testing.T) {
	defer netutils.SetupTestNetNS(t)()

	config := &NetworkConfiguration{BridgeName: DefaultBridgeName}
	br := &bridgeInterface{}

	if err := setupDevice(config, br); err != nil {
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
	defer netutils.SetupTestNetNS(t)()

	config := &NetworkConfiguration{BridgeName: "test0"}
	br := &bridgeInterface{}

	err := setupDevice(config, br)
	if err == nil {
		t.Fatal("Expected bridge creation failure with \"non default name\", succeeded")
	}

	if _, ok := err.(NonDefaultBridgeExistError); !ok {
		t.Fatalf("Did not fail with expected error. Actual error: %v", err)
	}
}

func TestSetupDeviceUp(t *testing.T) {
	defer netutils.SetupTestNetNS(t)()

	config := &NetworkConfiguration{BridgeName: DefaultBridgeName}
	br := &bridgeInterface{}

	if err := setupDevice(config, br); err != nil {
		t.Fatalf("Bridge creation failed: %v", err)
	}
	if err := setupDeviceUp(config, br); err != nil {
		t.Fatalf("Failed to up bridge device: %v", err)
	}

	lnk, _ := netlink.LinkByName(DefaultBridgeName)
	if lnk.Attrs().Flags&net.FlagUp != net.FlagUp {
		t.Fatalf("bridgeInterface should be up")
	}
}

func TestGenerateRandomMAC(t *testing.T) {
	defer netutils.SetupTestNetNS(t)()

	mac1 := netutils.GenerateRandomMAC()
	mac2 := netutils.GenerateRandomMAC()
	if bytes.Compare(mac1, mac2) == 0 {
		t.Fatalf("Generated twice the same MAC address %v", mac1)
	}
}
