package libnetwork_test

import (
	"net"
	"testing"

	log "github.com/Sirupsen/logrus"
	"github.com/docker/libnetwork"
	_ "github.com/docker/libnetwork/drivers/bridge"
	"github.com/docker/libnetwork/netutils"
	"github.com/docker/libnetwork/pkg/options"
)

var bridgeName = "dockertest0"

func createTestNetwork(networkType, networkName string, option options.Generic) (libnetwork.Network, error) {
	controller := libnetwork.New()

	driver, err := controller.NewNetworkDriver(networkType, option)
	if err != nil {
		return nil, err
	}

	network, err := controller.NewNetwork(driver, networkName, "")
	if err != nil {
		return nil, err
	}

	return network, nil
}

func Testbridge(t *testing.T) {
	defer netutils.SetupTestNetNS(t)()
	ip, subnet, err := net.ParseCIDR("192.168.100.1/24")
	if err != nil {
		t.Fatal(err)
	}
	subnet.IP = ip

	ip, cidr, err := net.ParseCIDR("192.168.100.2/28")
	if err != nil {
		t.Fatal(err)
	}
	cidr.IP = ip

	ip, cidrv6, err := net.ParseCIDR("fe90::1/96")
	if err != nil {
		t.Fatal(err)
	}
	cidrv6.IP = ip

	log.Debug("Adding a bridge")
	option := options.Generic{
		"BridgeName":            bridgeName,
		"AddressIPv4":           subnet,
		"FixedCIDR":             cidr,
		"FixedCIDRv6":           cidrv6,
		"EnableIPv6":            true,
		"EnableIPTables":        true,
		"EnableIPMasquerade":    true,
		"EnableICC":             true,
		"EnableIPForwarding":    true,
		"AllowNonDefaultBridge": true}

	network, err := createTestNetwork("bridge", "testnetwork", option)
	if err != nil {
		t.Fatal(err)
	}

	ep, err := network.CreateEndpoint("testep", "", "")
	if err != nil {
		t.Fatal(err)
	}

	epList := network.Endpoints()
	if len(epList) != 1 {
		t.Fatal(err)
	}
	if ep != epList[0] {
		t.Fatal(err)
	}

	if err := ep.Delete(); err != nil {
		t.Fatal(err)
	}

	if err := network.Delete(); err != nil {
		t.Fatal(err)
	}
}

func TestUnknownDriver(t *testing.T) {
	defer netutils.SetupTestNetNS(t)()
	_, err := createTestNetwork("unknowndriver", "testnetwork", options.Generic{})
	if err == nil {
		t.Fatal("Expected to fail. But instead succeeded")
	}

	if _, ok := err.(libnetwork.NetworkTypeError); !ok {
		t.Fatalf("Did not fail with expected error. Actual error: %v", err)
	}
}

func TestNilDriver(t *testing.T) {
	controller := libnetwork.New()

	option := options.Generic{}
	_, err := controller.NewNetwork(nil, "dummy", option)
	if err == nil {
		t.Fatal("Expected to fail. But instead succeeded")
	}

	if err != libnetwork.ErrNilNetworkDriver {
		t.Fatalf("Did not fail with expected error. Actual error: %v", err)
	}
}

func TestNoInitDriver(t *testing.T) {
	controller := libnetwork.New()

	option := options.Generic{}
	_, err := controller.NewNetwork(&libnetwork.NetworkDriver{}, "dummy", option)
	if err == nil {
		t.Fatal("Expected to fail. But instead succeeded")
	}

	if err != libnetwork.ErrInvalidNetworkDriver {
		t.Fatalf("Did not fail with expected error. Actual error: %v", err)
	}
}

func TestDuplicateNetwork(t *testing.T) {
	defer netutils.SetupTestNetNS(t)()
	controller := libnetwork.New()

	option := options.Generic{}
	driver, err := controller.NewNetworkDriver("bridge", option)
	if err != nil {
		t.Fatal(err)
	}

	_, err = controller.NewNetwork(driver, "testnetwork", "")
	if err != nil {
		t.Fatal(err)
	}

	_, err = controller.NewNetwork(driver, "testnetwork", "")
	if err == nil {
		t.Fatal("Expected to fail. But instead succeeded")
	}

	if _, ok := err.(libnetwork.NetworkNameError); !ok {
		t.Fatalf("Did not fail with expected error. Actual error: %v", err)
	}
}

func TestNetworkName(t *testing.T) {
	networkName := "testnetwork"

	n, err := createTestNetwork("bridge", networkName, options.Generic{})
	if err != nil {
		t.Fatal(err)
	}

	if n.Name() != networkName {
		t.Fatalf("Expected network name %s, got %s", networkName, n.Name())
	}
}

func TestNetworkType(t *testing.T) {
	networkType := "bridge"

	n, err := createTestNetwork(networkType, "testnetwork", options.Generic{})
	if err != nil {
		t.Fatal(err)
	}

	if n.Type() != networkType {
		t.Fatalf("Expected network type %s, got %s", networkType, n.Type())
	}
}

func TestNetworkID(t *testing.T) {
	networkType := "bridge"

	n, err := createTestNetwork(networkType, "testnetwork", options.Generic{})
	if err != nil {
		t.Fatal(err)
	}

	if n.ID() == "" {
		t.Fatal("Expected non-empty network id")
	}
}

func TestDeleteNetworkWithActiveEndpoints(t *testing.T) {
	defer netutils.SetupTestNetNS(t)()
	option := options.Generic{
		"BridgeName":            bridgeName,
		"AllowNonDefaultBridge": true}

	network, err := createTestNetwork("bridge", "testnetwork", option)
	if err != nil {
		t.Fatal(err)
	}

	ep, err := network.CreateEndpoint("testep", "", "")
	if err != nil {
		t.Fatal(err)
	}

	err = network.Delete()
	if err == nil {
		t.Fatal("Expected to fail. But instead succeeded")
	}

	if _, ok := err.(*libnetwork.ActiveEndpointsError); !ok {
		t.Fatalf("Did not fail with expected error. Actual error: %v", err)
	}

	// Done testing. Now cleanup.
	if err := ep.Delete(); err != nil {
		t.Fatal(err)
	}

	if err := network.Delete(); err != nil {
		t.Fatal(err)
	}
}

func TestUnknownNetwork(t *testing.T) {
	defer netutils.SetupTestNetNS(t)()
	option := options.Generic{
		"BridgeName":            bridgeName,
		"AllowNonDefaultBridge": true}

	network, err := createTestNetwork("bridge", "testnetwork", option)
	if err != nil {
		t.Fatal(err)
	}

	err = network.Delete()
	if err != nil {
		t.Fatal(err)
	}

	err = network.Delete()
	if err == nil {
		t.Fatal("Expected to fail. But instead succeeded")
	}

	if _, ok := err.(*libnetwork.UnknownNetworkError); !ok {
		t.Fatalf("Did not fail with expected error. Actual error: %v", err)
	}
}

func TestUnknownEndpoint(t *testing.T) {
	defer netutils.SetupTestNetNS(t)()
	ip, subnet, err := net.ParseCIDR("192.168.100.1/24")
	if err != nil {
		t.Fatal(err)
	}
	subnet.IP = ip

	option := options.Generic{
		"BridgeName":            bridgeName,
		"AddressIPv4":           subnet,
		"AllowNonDefaultBridge": true}

	network, err := createTestNetwork("bridge", "testnetwork", option)
	if err != nil {
		t.Fatal(err)
	}

	ep, err := network.CreateEndpoint("testep", "", "")
	if err != nil {
		t.Fatal(err)
	}

	err = ep.Delete()
	if err != nil {
		t.Fatal(err)
	}

	err = ep.Delete()
	if err == nil {
		t.Fatal("Expected to fail. But instead succeeded")
	}

	if _, ok := err.(*libnetwork.UnknownEndpointError); !ok {
		t.Fatalf("Did not fail with expected error. Actual error: %v", err)
	}

	// Done testing. Now cleanup
	if err := network.Delete(); err != nil {
		t.Fatal(err)
	}
}
