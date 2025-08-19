//go:build windows

package windows

import (
	"context"
	"net"
	"testing"

	"github.com/moby/moby/v2/daemon/libnetwork/driverapi"
	"github.com/moby/moby/v2/daemon/libnetwork/netlabel"
	"github.com/moby/moby/v2/daemon/libnetwork/types"
	"github.com/moby/moby/v2/internal/testutils/storeutils"
	"gotest.tools/v3/assert"
)

func testNetwork(networkType string, t *testing.T) {
	d, err := newDriver(networkType, storeutils.NewTempStore(t))
	assert.NilError(t, err)
	bnw, _ := types.ParseCIDR("172.16.0.0/24")
	br, _ := types.ParseCIDR("172.16.0.1/16")

	netOption := make(map[string]any)
	networkOptions := map[string]string{
		NetworkName: "TestNetwork",
	}

	netOption[netlabel.GenericData] = networkOptions
	ipdList := []driverapi.IPAMData{
		{
			Pool:    bnw,
			Gateway: br,
		},
	}

	err = d.CreateNetwork(context.Background(), "dummy", netOption, nil, ipdList, nil)
	if err != nil {
		t.Fatalf("Failed to create bridge: %v", err)
	}
	defer func() {
		err = d.DeleteNetwork("dummy")
		if err != nil {
			t.Fatalf("Failed to create bridge: %v", err)
		}
	}()

	epOptions := make(map[string]any)
	te := &testEndpoint{}
	err = d.CreateEndpoint(context.TODO(), "dummy", "ep1", te.Interface(), epOptions)
	if err != nil {
		t.Fatalf("Failed to create an endpoint : %s", err.Error())
	}

	err = d.DeleteEndpoint("dummy", "ep1")
	if err != nil {
		t.Fatalf("Failed to delete an endpoint : %s", err.Error())
	}
}

func TestNAT(t *testing.T) {
	t.Skip("Test does not work on CI and was never running to begin with")
	testNetwork("nat", t)
}

func TestTransparent(t *testing.T) {
	t.Skip("Test does not work on CI and was never running to begin with")
	testNetwork("transparent", t)
}

type testEndpoint struct {
	t                     *testing.T
	src                   string
	dst                   string
	address               string
	macAddress            string
	gateway               string
	disableGatewayService bool
}

func (test *testEndpoint) Interface() driverapi.InterfaceInfo {
	return test
}

func (test *testEndpoint) Address() *net.IPNet {
	if test.address == "" {
		return nil
	}
	nw, _ := types.ParseCIDR(test.address)
	return nw
}

func (test *testEndpoint) AddressIPv6() *net.IPNet {
	return nil
}

func (test *testEndpoint) MacAddress() net.HardwareAddr {
	if test.macAddress == "" {
		return nil
	}
	mac, _ := net.ParseMAC(test.macAddress)
	return mac
}

func (test *testEndpoint) SetMacAddress(mac net.HardwareAddr) error {
	if test.macAddress != "" {
		return types.ForbiddenErrorf("endpoint interface MAC address present (%s). Cannot be modified with %s.", test.macAddress, mac)
	}

	if mac == nil {
		return types.InvalidParameterErrorf("tried to set nil MAC address to endpoint interface")
	}
	test.macAddress = mac.String()
	return nil
}

func (test *testEndpoint) SetIPAddress(address *net.IPNet) error {
	if address.IP == nil {
		return types.InvalidParameterErrorf("tried to set nil IP address to endpoint interface")
	}

	test.address = address.String()
	return nil
}

func (test *testEndpoint) InterfaceName() driverapi.InterfaceNameInfo {
	return test
}

func (test *testEndpoint) SetGateway(ipv4 net.IP) error {
	return nil
}

func (test *testEndpoint) SetGatewayIPv6(ipv6 net.IP) error {
	return nil
}

func (test *testEndpoint) SetNames(_, _, _ string) error {
	return nil
}

func (test *testEndpoint) AddStaticRoute(destination *net.IPNet, routeType types.RouteType, nextHop net.IP) error {
	return nil
}

func (test *testEndpoint) DisableGatewayService() {
	test.disableGatewayService = true
}

func (test *testEndpoint) NetnsPath() string {
	return ""
}

func (test *testEndpoint) SetCreatedInContainer(bool) {
}
