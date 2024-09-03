package bridge

import (
	"bytes"
	"net"
	"syscall"
	"testing"

	"github.com/docker/docker/internal/nlwrap"
	"github.com/docker/docker/internal/testutils/netnsutils"
	"github.com/docker/docker/libnetwork/netutils"
	"gotest.tools/v3/assert"
)

func TestSetupNewBridge(t *testing.T) {
	defer netnsutils.SetupTestOSContext(t)()

	nh, err := nlwrap.NewHandle()
	if err != nil {
		t.Fatal(err)
	}
	defer nh.Close()

	config := &networkConfiguration{BridgeName: DefaultBridgeName}
	br := &bridgeInterface{nlh: nh}

	if err := setupDevice(config, br); err != nil {
		t.Fatalf("Bridge creation failed: %v", err)
	}
	if br.Link == nil {
		t.Fatal("bridgeInterface link is nil (expected valid link)")
	}
	if _, err := nh.LinkByName(DefaultBridgeName); err != nil {
		t.Fatalf("Failed to retrieve bridge device: %v", err)
	}
	if br.Link.Attrs().Flags&net.FlagUp == net.FlagUp {
		t.Fatal("bridgeInterface should be created down")
	}
}

func TestSetupNewNonDefaultBridge(t *testing.T) {
	defer netnsutils.SetupTestOSContext(t)()

	nh, err := nlwrap.NewHandle()
	if err != nil {
		t.Fatal(err)
	}
	defer nh.Close()

	config := &networkConfiguration{BridgeName: "test0", DefaultBridge: true}
	br := &bridgeInterface{nlh: nh}

	err = setupDevice(config, br)
	if err == nil {
		t.Fatal(`Expected bridge creation failure with "non default name", succeeded`)
	}

	if _, ok := err.(NonDefaultBridgeExistError); !ok {
		t.Fatalf("Did not fail with expected error. Actual error: %v", err)
	}
}

func TestSetupDeviceUp(t *testing.T) {
	defer netnsutils.SetupTestOSContext(t)()

	nh, err := nlwrap.NewHandle()
	if err != nil {
		t.Fatal(err)
	}
	defer nh.Close()

	config := &networkConfiguration{BridgeName: DefaultBridgeName}
	br := &bridgeInterface{nlh: nh}

	if err := setupDevice(config, br); err != nil {
		t.Fatalf("Bridge creation failed: %v", err)
	}
	if err := setupDeviceUp(config, br); err != nil {
		t.Fatalf("Failed to up bridge device: %v", err)
	}

	lnk, _ := nh.LinkByName(DefaultBridgeName)
	if lnk.Attrs().Flags&net.FlagUp != net.FlagUp {
		t.Fatal("bridgeInterface should be up")
	}
}

func TestGenerateRandomMAC(t *testing.T) {
	defer netnsutils.SetupTestOSContext(t)()

	mac1 := netutils.GenerateRandomMAC()
	mac2 := netutils.GenerateRandomMAC()
	if bytes.Equal(mac1, mac2) {
		t.Fatalf("Generated twice the same MAC address %v", mac1)
	}
}

func TestMTUBiggerThan1500(t *testing.T) {
	defer netnsutils.SetupTestOSContext(t)()

	nh, err := nlwrap.NewHandle()
	if err != nil {
		t.Fatal(err)
	}
	defer nh.Close()

	config := &networkConfiguration{BridgeName: DefaultBridgeName, Mtu: 9000}
	br := &bridgeInterface{nlh: nh}

	assert.NilError(t, setupDevice(config, br))
	assert.NilError(t, setupMTU(config, br))
}

func TestMTUBiggerThan64K(t *testing.T) {
	defer netnsutils.SetupTestOSContext(t)()

	nh, err := nlwrap.NewHandle()
	if err != nil {
		t.Fatal(err)
	}
	defer nh.Close()

	config := &networkConfiguration{BridgeName: DefaultBridgeName, Mtu: 65536}
	br := &bridgeInterface{nlh: nh}

	assert.NilError(t, setupDevice(config, br))
	assert.ErrorIs(t, setupMTU(config, br), syscall.EINVAL)
}
