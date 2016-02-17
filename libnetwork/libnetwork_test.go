package libnetwork_test

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"testing"

	log "github.com/Sirupsen/logrus"
	"github.com/docker/docker/pkg/plugins"
	"github.com/docker/docker/pkg/reexec"
	"github.com/docker/libnetwork"
	"github.com/docker/libnetwork/config"
	"github.com/docker/libnetwork/datastore"
	"github.com/docker/libnetwork/driverapi"
	"github.com/docker/libnetwork/ipamapi"
	"github.com/docker/libnetwork/netlabel"
	"github.com/docker/libnetwork/options"
	"github.com/docker/libnetwork/osl"
	"github.com/docker/libnetwork/testutils"
	"github.com/docker/libnetwork/types"
	"github.com/opencontainers/runc/libcontainer"
	"github.com/opencontainers/runc/libcontainer/configs"
	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netns"
)

const (
	bridgeNetType = "bridge"
)

var controller libnetwork.NetworkController

func TestMain(m *testing.M) {
	if reexec.Init() {
		return
	}

	if err := createController(); err != nil {
		log.Errorf("Error creating controller: %v", err)
		os.Exit(1)
	}

	//libnetwork.SetTestDataStore(controller, datastore.NewCustomDataStore(datastore.NewMockStore()))

	x := m.Run()
	controller.Stop()
	os.Exit(x)
}

func createController() error {
	var err error

	// Cleanup local datastore file
	os.Remove(datastore.DefaultScopes("")[datastore.LocalScope].Client.Address)

	option := options.Generic{
		"EnableIPForwarding": true,
	}

	genericOption := make(map[string]interface{})
	genericOption[netlabel.GenericData] = option

	cfgOptions, err := libnetwork.OptionBoltdbWithRandomDBFile()
	if err != nil {
		return err
	}
	controller, err = libnetwork.New(append(cfgOptions, config.OptionDriverConfig(bridgeNetType, genericOption))...)
	if err != nil {
		return err
	}

	return nil
}

func createTestNetwork(networkType, networkName string, netOption options.Generic, ipamV4Configs, ipamV6Configs []*libnetwork.IpamConf) (libnetwork.Network, error) {
	return controller.NewNetwork(networkType, networkName,
		libnetwork.NetworkOptionGeneric(netOption),
		libnetwork.NetworkOptionIpam(ipamapi.DefaultIPAM, "", ipamV4Configs, ipamV6Configs, nil))
}

func getEmptyGenericOption() map[string]interface{} {
	genericOption := make(map[string]interface{})
	genericOption[netlabel.GenericData] = options.Generic{}
	return genericOption
}

func getPortMapping() []types.PortBinding {
	return []types.PortBinding{
		{Proto: types.TCP, Port: uint16(230), HostPort: uint16(23000)},
		{Proto: types.UDP, Port: uint16(200), HostPort: uint16(22000)},
		{Proto: types.TCP, Port: uint16(120), HostPort: uint16(12000)},
		{Proto: types.TCP, Port: uint16(320), HostPort: uint16(32000), HostPortEnd: uint16(32999)},
		{Proto: types.UDP, Port: uint16(420), HostPort: uint16(42000), HostPortEnd: uint16(42001)},
	}
}

func TestNull(t *testing.T) {
	cnt, err := controller.NewSandbox("null_container",
		libnetwork.OptionHostname("test"),
		libnetwork.OptionDomainname("docker.io"),
		libnetwork.OptionExtraHost("web", "192.168.0.1"))
	if err != nil {
		t.Fatal(err)
	}

	network, err := createTestNetwork("null", "testnull", options.Generic{}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	ep, err := network.CreateEndpoint("testep")
	if err != nil {
		t.Fatal(err)
	}

	err = ep.Join(cnt)
	if err != nil {
		t.Fatal(err)
	}

	err = ep.Leave(cnt)
	if err != nil {
		t.Fatal(err)
	}

	if err := ep.Delete(false); err != nil {
		t.Fatal(err)
	}

	if err := cnt.Delete(); err != nil {
		t.Fatal(err)
	}

	// host type is special network. Cannot be removed.
	err = network.Delete()
	if err == nil {
		t.Fatal(err)
	}
	if _, ok := err.(types.ForbiddenError); !ok {
		t.Fatalf("Unexpected error type")
	}
}

func TestHost(t *testing.T) {
	sbx1, err := controller.NewSandbox("host_c1",
		libnetwork.OptionHostname("test1"),
		libnetwork.OptionDomainname("docker.io"),
		libnetwork.OptionExtraHost("web", "192.168.0.1"),
		libnetwork.OptionUseDefaultSandbox())
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := sbx1.Delete(); err != nil {
			t.Fatal(err)
		}
	}()

	sbx2, err := controller.NewSandbox("host_c2",
		libnetwork.OptionHostname("test2"),
		libnetwork.OptionDomainname("docker.io"),
		libnetwork.OptionExtraHost("web", "192.168.0.1"),
		libnetwork.OptionUseDefaultSandbox())
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := sbx2.Delete(); err != nil {
			t.Fatal(err)
		}
	}()

	network, err := createTestNetwork("host", "testhost", options.Generic{}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	ep1, err := network.CreateEndpoint("testep1")
	if err != nil {
		t.Fatal(err)
	}

	if err := ep1.Join(sbx1); err != nil {
		t.Fatal(err)
	}

	ep2, err := network.CreateEndpoint("testep2")
	if err != nil {
		t.Fatal(err)
	}

	if err := ep2.Join(sbx2); err != nil {
		t.Fatal(err)
	}

	if err := ep1.Leave(sbx1); err != nil {
		t.Fatal(err)
	}

	if err := ep2.Leave(sbx2); err != nil {
		t.Fatal(err)
	}

	if err := ep1.Delete(false); err != nil {
		t.Fatal(err)
	}

	if err := ep2.Delete(false); err != nil {
		t.Fatal(err)
	}

	// Try to create another host endpoint and join/leave that.
	cnt3, err := controller.NewSandbox("host_c3",
		libnetwork.OptionHostname("test3"),
		libnetwork.OptionDomainname("docker.io"),
		libnetwork.OptionExtraHost("web", "192.168.0.1"),
		libnetwork.OptionUseDefaultSandbox())
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := cnt3.Delete(); err != nil {
			t.Fatal(err)
		}
	}()

	ep3, err := network.CreateEndpoint("testep3")
	if err != nil {
		t.Fatal(err)
	}

	if err := ep3.Join(sbx2); err != nil {
		t.Fatal(err)
	}

	if err := ep3.Leave(sbx2); err != nil {
		t.Fatal(err)
	}

	if err := ep3.Delete(false); err != nil {
		t.Fatal(err)
	}

	// host type is special network. Cannot be removed.
	err = network.Delete()
	if err == nil {
		t.Fatal(err)
	}
	if _, ok := err.(types.ForbiddenError); !ok {
		t.Fatalf("Unexpected error type")
	}
}

func TestBridge(t *testing.T) {
	if !testutils.IsRunningInContainer() {
		defer testutils.SetupTestOSContext(t)()
	}

	netOption := options.Generic{
		netlabel.EnableIPv6: true,
		netlabel.GenericData: options.Generic{
			"BridgeName":         "testnetwork",
			"EnableICC":          true,
			"EnableIPMasquerade": true,
		},
	}
	ipamV4ConfList := []*libnetwork.IpamConf{{PreferredPool: "192.168.100.0/24", Gateway: "192.168.100.1"}}
	ipamV6ConfList := []*libnetwork.IpamConf{{PreferredPool: "fe90::/64", Gateway: "fe90::22"}}

	network, err := createTestNetwork(bridgeNetType, "testnetwork", netOption, ipamV4ConfList, ipamV6ConfList)
	if err != nil {
		t.Fatal(err)
	}

	ep, err := network.CreateEndpoint("testep", libnetwork.CreateOptionPortMapping(getPortMapping()))
	if err != nil {
		t.Fatal(err)
	}

	epInfo, err := ep.DriverInfo()
	if err != nil {
		t.Fatal(err)
	}
	pmd, ok := epInfo[netlabel.PortMap]
	if !ok {
		t.Fatalf("Could not find expected info in endpoint data")
	}
	pm, ok := pmd.([]types.PortBinding)
	if !ok {
		t.Fatalf("Unexpected format for port mapping in endpoint operational data")
	}
	if len(pm) != 5 {
		t.Fatalf("Incomplete data for port mapping in endpoint operational data: %d", len(pm))
	}

	if err := ep.Delete(false); err != nil {
		t.Fatal(err)
	}

	if err := network.Delete(); err != nil {
		t.Fatal(err)
	}
}

// Testing IPV6 from MAC address
func TestBridgeIpv6FromMac(t *testing.T) {
	if !testutils.IsRunningInContainer() {
		defer testutils.SetupTestOSContext(t)()
	}

	netOption := options.Generic{
		netlabel.GenericData: options.Generic{
			"BridgeName":         "testipv6mac",
			"EnableICC":          true,
			"EnableIPMasquerade": true,
		},
	}
	ipamV4ConfList := []*libnetwork.IpamConf{{PreferredPool: "192.168.100.0/24", Gateway: "192.168.100.1"}}
	ipamV6ConfList := []*libnetwork.IpamConf{{PreferredPool: "fe90::/64", Gateway: "fe90::22"}}

	network, err := controller.NewNetwork(bridgeNetType, "testipv6mac",
		libnetwork.NetworkOptionGeneric(netOption),
		libnetwork.NetworkOptionEnableIPv6(true),
		libnetwork.NetworkOptionIpam(ipamapi.DefaultIPAM, "", ipamV4ConfList, ipamV6ConfList, nil),
		libnetwork.NetworkOptionDeferIPv6Alloc(true))
	if err != nil {
		t.Fatal(err)
	}

	mac := net.HardwareAddr{0xaa, 0xbb, 0xcc, 0xdd, 0xee, 0xff}
	epOption := options.Generic{netlabel.MacAddress: mac}

	ep, err := network.CreateEndpoint("testep", libnetwork.EndpointOptionGeneric(epOption))
	if err != nil {
		t.Fatal(err)
	}

	iface := ep.Info().Iface()
	if !bytes.Equal(iface.MacAddress(), mac) {
		t.Fatalf("Unexpected mac address: %v", iface.MacAddress())
	}

	ip, expIP, _ := net.ParseCIDR("fe90::aabb:ccdd:eeff/64")
	expIP.IP = ip
	if !types.CompareIPNet(expIP, iface.AddressIPv6()) {
		t.Fatalf("Expected %v. Got: %v", expIP, iface.AddressIPv6())
	}

	if err := ep.Delete(false); err != nil {
		t.Fatal(err)
	}

	if err := network.Delete(); err != nil {
		t.Fatal(err)
	}
}

func TestUnknownDriver(t *testing.T) {
	if !testutils.IsRunningInContainer() {
		defer testutils.SetupTestOSContext(t)()
	}

	_, err := createTestNetwork("unknowndriver", "testnetwork", options.Generic{}, nil, nil)
	if err == nil {
		t.Fatal("Expected to fail. But instead succeeded")
	}

	if _, ok := err.(types.NotFoundError); !ok {
		t.Fatalf("Did not fail with expected error. Actual error: %v", err)
	}
}

func TestNilRemoteDriver(t *testing.T) {
	_, err := controller.NewNetwork("framerelay", "dummy",
		libnetwork.NetworkOptionGeneric(getEmptyGenericOption()))
	if err == nil {
		t.Fatal("Expected to fail. But instead succeeded")
	}

	if _, ok := err.(types.NotFoundError); !ok {
		t.Fatalf("Did not fail with expected error. Actual error: %v", err)
	}
}

func TestNetworkName(t *testing.T) {
	if !testutils.IsRunningInContainer() {
		defer testutils.SetupTestOSContext(t)()
	}

	netOption := options.Generic{
		netlabel.GenericData: options.Generic{
			"BridgeName": "testnetwork",
		},
	}

	_, err := createTestNetwork(bridgeNetType, "", netOption, nil, nil)
	if err == nil {
		t.Fatal("Expected to fail. But instead succeeded")
	}

	if _, ok := err.(libnetwork.ErrInvalidName); !ok {
		t.Fatalf("Expected to fail with ErrInvalidName error. Got %v", err)
	}

	networkName := "testnetwork"
	n, err := createTestNetwork(bridgeNetType, networkName, netOption, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := n.Delete(); err != nil {
			t.Fatal(err)
		}
	}()

	if n.Name() != networkName {
		t.Fatalf("Expected network name %s, got %s", networkName, n.Name())
	}
}

func TestNetworkType(t *testing.T) {
	if !testutils.IsRunningInContainer() {
		defer testutils.SetupTestOSContext(t)()
	}

	netOption := options.Generic{
		netlabel.GenericData: options.Generic{
			"BridgeName": "testnetwork",
		},
	}

	n, err := createTestNetwork(bridgeNetType, "testnetwork", netOption, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := n.Delete(); err != nil {
			t.Fatal(err)
		}
	}()

	if n.Type() != bridgeNetType {
		t.Fatalf("Expected network type %s, got %s", bridgeNetType, n.Type())
	}
}

func TestNetworkID(t *testing.T) {
	if !testutils.IsRunningInContainer() {
		defer testutils.SetupTestOSContext(t)()
	}

	netOption := options.Generic{
		netlabel.GenericData: options.Generic{
			"BridgeName": "testnetwork",
		},
	}

	n, err := createTestNetwork(bridgeNetType, "testnetwork", netOption, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := n.Delete(); err != nil {
			t.Fatal(err)
		}
	}()

	if n.ID() == "" {
		t.Fatal("Expected non-empty network id")
	}
}

func TestDeleteNetworkWithActiveEndpoints(t *testing.T) {
	if !testutils.IsRunningInContainer() {
		defer testutils.SetupTestOSContext(t)()
	}

	netOption := options.Generic{
		"BridgeName": "testnetwork",
	}
	option := options.Generic{
		netlabel.GenericData: netOption,
	}

	network, err := createTestNetwork(bridgeNetType, "testnetwork", option, nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	ep, err := network.CreateEndpoint("testep")
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
	if err := ep.Delete(false); err != nil {
		t.Fatal(err)
	}

	if err := network.Delete(); err != nil {
		t.Fatal(err)
	}
}

func TestUnknownNetwork(t *testing.T) {
	if !testutils.IsRunningInContainer() {
		defer testutils.SetupTestOSContext(t)()
	}

	netOption := options.Generic{
		"BridgeName": "testnetwork",
	}
	option := options.Generic{
		netlabel.GenericData: netOption,
	}

	network, err := createTestNetwork(bridgeNetType, "testnetwork", option, nil, nil)
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
	if !testutils.IsRunningInContainer() {
		defer testutils.SetupTestOSContext(t)()
	}

	netOption := options.Generic{
		"BridgeName": "testnetwork",
	}
	option := options.Generic{
		netlabel.GenericData: netOption,
	}
	ipamV4ConfList := []*libnetwork.IpamConf{{PreferredPool: "192.168.100.0/24"}}

	network, err := createTestNetwork(bridgeNetType, "testnetwork", option, ipamV4ConfList, nil)
	if err != nil {
		t.Fatal(err)
	}

	_, err = network.CreateEndpoint("")
	if err == nil {
		t.Fatal("Expected to fail. But instead succeeded")
	}
	if _, ok := err.(libnetwork.ErrInvalidName); !ok {
		t.Fatalf("Expected to fail with ErrInvalidName error. Actual error: %v", err)
	}

	ep, err := network.CreateEndpoint("testep")
	if err != nil {
		t.Fatal(err)
	}

	err = ep.Delete(false)
	if err != nil {
		t.Fatal(err)
	}

	// Done testing. Now cleanup
	if err := network.Delete(); err != nil {
		t.Fatal(err)
	}
}

func TestNetworkEndpointsWalkers(t *testing.T) {
	if !testutils.IsRunningInContainer() {
		defer testutils.SetupTestOSContext(t)()
	}

	// Create network 1 and add 2 endpoint: ep11, ep12
	netOption := options.Generic{
		netlabel.GenericData: options.Generic{
			"BridgeName": "network1",
		},
	}

	net1, err := createTestNetwork(bridgeNetType, "network1", netOption, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := net1.Delete(); err != nil {
			t.Fatal(err)
		}
	}()

	ep11, err := net1.CreateEndpoint("ep11")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := ep11.Delete(false); err != nil {
			t.Fatal(err)
		}
	}()

	ep12, err := net1.CreateEndpoint("ep12")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := ep12.Delete(false); err != nil {
			t.Fatal(err)
		}
	}()

	// Test list methods on net1
	epList1 := net1.Endpoints()
	if len(epList1) != 2 {
		t.Fatalf("Endpoints() returned wrong number of elements: %d instead of 2", len(epList1))
	}
	// endpoint order is not guaranteed
	for _, e := range epList1 {
		if e != ep11 && e != ep12 {
			t.Fatal("Endpoints() did not return all the expected elements")
		}
	}

	// Test Endpoint Walk method
	var epName string
	var epWanted libnetwork.Endpoint
	wlk := func(ep libnetwork.Endpoint) bool {
		if ep.Name() == epName {
			epWanted = ep
			return true
		}
		return false
	}

	// Look for ep1 on network1
	epName = "ep11"
	net1.WalkEndpoints(wlk)
	if epWanted == nil {
		t.Fatal(err)
	}
	if ep11 != epWanted {
		t.Fatal(err)
	}

	current := len(controller.Networks())

	// Create network 2
	netOption = options.Generic{
		netlabel.GenericData: options.Generic{
			"BridgeName": "network2",
		},
	}

	net2, err := createTestNetwork(bridgeNetType, "network2", netOption, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := net2.Delete(); err != nil {
			t.Fatal(err)
		}
	}()

	// Test Networks method
	if len(controller.Networks()) != current+1 {
		t.Fatalf("Did not find the expected number of networks")
	}

	// Test Network Walk method
	var netName string
	var netWanted libnetwork.Network
	nwWlk := func(nw libnetwork.Network) bool {
		if nw.Name() == netName {
			netWanted = nw
			return true
		}
		return false
	}

	// Look for network named "network1" and "network2"
	netName = "network1"
	controller.WalkNetworks(nwWlk)
	if netWanted == nil {
		t.Fatal(err)
	}
	if net1.ID() != netWanted.ID() {
		t.Fatal(err)
	}

	netName = "network2"
	controller.WalkNetworks(nwWlk)
	if netWanted == nil {
		t.Fatal(err)
	}
	if net2.ID() != netWanted.ID() {
		t.Fatal(err)
	}
}

func TestDuplicateEndpoint(t *testing.T) {
	if !testutils.IsRunningInContainer() {
		defer testutils.SetupTestOSContext(t)()
	}

	netOption := options.Generic{
		netlabel.GenericData: options.Generic{
			"BridgeName": "testnetwork",
		},
	}
	n, err := createTestNetwork(bridgeNetType, "testnetwork", netOption, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := n.Delete(); err != nil {
			t.Fatal(err)
		}
	}()

	ep, err := n.CreateEndpoint("ep1")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := ep.Delete(false); err != nil {
			t.Fatal(err)
		}
	}()

	ep2, err := n.CreateEndpoint("ep1")
	defer func() {
		// Cleanup ep2 as well, else network cleanup might fail for failure cases
		if ep2 != nil {
			if err := ep2.Delete(false); err != nil {
				t.Fatal(err)
			}
		}
	}()

	if err == nil {
		t.Fatal("Expected to fail. But instead succeeded")
	}

	if _, ok := err.(types.ForbiddenError); !ok {
		t.Fatalf("Did not fail with expected error. Actual error: %v", err)
	}
}

func TestControllerQuery(t *testing.T) {
	if !testutils.IsRunningInContainer() {
		defer testutils.SetupTestOSContext(t)()
	}

	// Create network 1
	netOption := options.Generic{
		netlabel.GenericData: options.Generic{
			"BridgeName": "network1",
		},
	}
	net1, err := createTestNetwork(bridgeNetType, "network1", netOption, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := net1.Delete(); err != nil {
			t.Fatal(err)
		}
	}()

	// Create network 2
	netOption = options.Generic{
		netlabel.GenericData: options.Generic{
			"BridgeName": "network2",
		},
	}
	net2, err := createTestNetwork(bridgeNetType, "network2", netOption, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := net2.Delete(); err != nil {
			t.Fatal(err)
		}
	}()

	_, err = controller.NetworkByName("")
	if err == nil {
		t.Fatalf("NetworkByName() succeeded with invalid target name")
	}
	if _, ok := err.(libnetwork.ErrInvalidName); !ok {
		t.Fatalf("Expected NetworkByName() to fail with ErrInvalidName error. Got: %v", err)
	}

	_, err = controller.NetworkByID("")
	if err == nil {
		t.Fatalf("NetworkByID() succeeded with invalid target id")
	}
	if _, ok := err.(libnetwork.ErrInvalidID); !ok {
		t.Fatalf("NetworkByID() failed with unexpected error: %v", err)
	}

	g, err := controller.NetworkByID("network1")
	if err == nil {
		t.Fatalf("Unexpected success for NetworkByID(): %v", g)
	}
	if _, ok := err.(libnetwork.ErrNoSuchNetwork); !ok {
		t.Fatalf("NetworkByID() failed with unexpected error: %v", err)
	}

	g, err = controller.NetworkByName("network1")
	if err != nil {
		t.Fatalf("Unexpected failure for NetworkByName(): %v", err)
	}
	if g == nil {
		t.Fatalf("NetworkByName() did not find the network")
	}

	if g != net1 {
		t.Fatalf("NetworkByName() returned the wrong network")
	}

	g, err = controller.NetworkByID(net1.ID())
	if err != nil {
		t.Fatalf("Unexpected failure for NetworkByID(): %v", err)
	}
	if net1.ID() != g.ID() {
		t.Fatalf("NetworkByID() returned unexpected element: %v", g)
	}

	g, err = controller.NetworkByName("network2")
	if err != nil {
		t.Fatalf("Unexpected failure for NetworkByName(): %v", err)
	}
	if g == nil {
		t.Fatalf("NetworkByName() did not find the network")
	}

	if g != net2 {
		t.Fatalf("NetworkByName() returned the wrong network")
	}

	g, err = controller.NetworkByID(net2.ID())
	if err != nil {
		t.Fatalf("Unexpected failure for NetworkByID(): %v", err)
	}
	if net2.ID() != g.ID() {
		t.Fatalf("NetworkByID() returned unexpected element: %v", g)
	}
}

func TestNetworkQuery(t *testing.T) {
	if !testutils.IsRunningInContainer() {
		defer testutils.SetupTestOSContext(t)()
	}

	// Create network 1 and add 2 endpoint: ep11, ep12
	netOption := options.Generic{
		netlabel.GenericData: options.Generic{
			"BridgeName": "network1",
		},
	}
	net1, err := createTestNetwork(bridgeNetType, "network1", netOption, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := net1.Delete(); err != nil {
			t.Fatal(err)
		}
	}()

	ep11, err := net1.CreateEndpoint("ep11")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := ep11.Delete(false); err != nil {
			t.Fatal(err)
		}
	}()

	ep12, err := net1.CreateEndpoint("ep12")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := ep12.Delete(false); err != nil {
			t.Fatal(err)
		}
	}()

	e, err := net1.EndpointByName("ep11")
	if err != nil {
		t.Fatal(err)
	}
	if ep11 != e {
		t.Fatalf("EndpointByName() returned %v instead of %v", e, ep11)
	}

	e, err = net1.EndpointByName("")
	if err == nil {
		t.Fatalf("EndpointByName() succeeded with invalid target name")
	}
	if _, ok := err.(libnetwork.ErrInvalidName); !ok {
		t.Fatalf("Expected EndpointByName() to fail with ErrInvalidName error. Got: %v", err)
	}

	e, err = net1.EndpointByName("IamNotAnEndpoint")
	if err == nil {
		t.Fatalf("EndpointByName() succeeded with unknown target name")
	}
	if _, ok := err.(libnetwork.ErrNoSuchEndpoint); !ok {
		t.Fatal(err)
	}
	if e != nil {
		t.Fatalf("EndpointByName(): expected nil, got %v", e)
	}

	e, err = net1.EndpointByID(ep12.ID())
	if err != nil {
		t.Fatal(err)
	}
	if ep12.ID() != e.ID() {
		t.Fatalf("EndpointByID() returned %v instead of %v", e, ep12)
	}

	e, err = net1.EndpointByID("")
	if err == nil {
		t.Fatalf("EndpointByID() succeeded with invalid target id")
	}
	if _, ok := err.(libnetwork.ErrInvalidID); !ok {
		t.Fatalf("EndpointByID() failed with unexpected error: %v", err)
	}
}

const containerID = "valid_c"

func checkSandbox(t *testing.T, info libnetwork.EndpointInfo) {
	origns, err := netns.Get()
	if err != nil {
		t.Fatalf("Could not get the current netns: %v", err)
	}
	defer origns.Close()

	key := info.Sandbox().Key()
	f, err := os.OpenFile(key, os.O_RDONLY, 0)
	if err != nil {
		t.Fatalf("Failed to open network namespace path %q: %v", key, err)
	}
	defer f.Close()

	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	nsFD := f.Fd()
	if err = netns.Set(netns.NsHandle(nsFD)); err != nil {
		t.Fatalf("Setting to the namespace pointed to by the sandbox %s failed: %v", key, err)
	}
	defer netns.Set(origns)

	_, err = netlink.LinkByName("eth0")
	if err != nil {
		t.Fatalf("Could not find the interface eth0 inside the sandbox: %v", err)
	}

	_, err = netlink.LinkByName("eth1")
	if err != nil {
		t.Fatalf("Could not find the interface eth1 inside the sandbox: %v", err)
	}
}

func TestEndpointJoin(t *testing.T) {
	if !testutils.IsRunningInContainer() {
		defer testutils.SetupTestOSContext(t)()
	}

	// Create network 1 and add 2 endpoint: ep11, ep12
	netOption := options.Generic{
		netlabel.GenericData: options.Generic{
			"BridgeName":         "testnetwork1",
			"EnableICC":          true,
			"EnableIPMasquerade": true,
		},
	}
	ipamV6ConfList := []*libnetwork.IpamConf{{PreferredPool: "fe90::/64", Gateway: "fe90::22"}}
	n1, err := controller.NewNetwork(bridgeNetType, "testnetwork1",
		libnetwork.NetworkOptionGeneric(netOption),
		libnetwork.NetworkOptionEnableIPv6(true),
		libnetwork.NetworkOptionIpam(ipamapi.DefaultIPAM, "", nil, ipamV6ConfList, nil),
		libnetwork.NetworkOptionDeferIPv6Alloc(true))
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := n1.Delete(); err != nil {
			t.Fatal(err)
		}
	}()

	ep1, err := n1.CreateEndpoint("ep1")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := ep1.Delete(false); err != nil {
			t.Fatal(err)
		}
	}()

	// Validate if ep.Info() only gives me IP address info and not names and gateway during CreateEndpoint()
	info := ep1.Info()
	iface := info.Iface()
	if iface.Address() != nil && iface.Address().IP.To4() == nil {
		t.Fatalf("Invalid IP address returned: %v", iface.Address())
	}
	if iface.AddressIPv6() != nil && iface.AddressIPv6().IP == nil {
		t.Fatalf("Invalid IPv6 address returned: %v", iface.Address())
	}

	if len(info.Gateway()) != 0 {
		t.Fatalf("Expected empty gateway for an empty endpoint. Instead found a gateway: %v", info.Gateway())
	}
	if len(info.GatewayIPv6()) != 0 {
		t.Fatalf("Expected empty gateway for an empty ipv6 endpoint. Instead found a gateway: %v", info.GatewayIPv6())
	}

	if info.Sandbox() != nil {
		t.Fatalf("Expected an empty sandbox key for an empty endpoint. Instead found a non-empty sandbox key: %s", info.Sandbox().Key())
	}

	// test invalid joins
	err = ep1.Join(nil)
	if err == nil {
		t.Fatalf("Expected to fail join with nil Sandbox")
	}
	if _, ok := err.(types.BadRequestError); !ok {
		t.Fatalf("Unexpected error type returned: %T", err)
	}

	fsbx := &fakeSandbox{}
	if err = ep1.Join(fsbx); err == nil {
		t.Fatalf("Expected to fail join with invalid Sandbox")
	}
	if _, ok := err.(types.BadRequestError); !ok {
		t.Fatalf("Unexpected error type returned: %T", err)
	}

	sb, err := controller.NewSandbox(containerID,
		libnetwork.OptionHostname("test"),
		libnetwork.OptionDomainname("docker.io"),
		libnetwork.OptionExtraHost("web", "192.168.0.1"))
	if err != nil {
		t.Fatal(err)
	}

	defer func() {
		if err := sb.Delete(); err != nil {
			t.Fatal(err)
		}
	}()

	err = ep1.Join(sb)
	runtime.LockOSThread()
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		err = ep1.Leave(sb)
		runtime.LockOSThread()
		if err != nil {
			t.Fatal(err)
		}
	}()

	// Validate if ep.Info() only gives valid gateway and sandbox key after has container has joined.
	info = ep1.Info()
	if len(info.Gateway()) == 0 {
		t.Fatalf("Expected a valid gateway for a joined endpoint. Instead found an invalid gateway: %v", info.Gateway())
	}
	if len(info.GatewayIPv6()) == 0 {
		t.Fatalf("Expected a valid ipv6 gateway for a joined endpoint. Instead found an invalid gateway: %v", info.GatewayIPv6())
	}

	if info.Sandbox() == nil {
		t.Fatalf("Expected an non-empty sandbox key for a joined endpoint. Instead found a empty sandbox key")
	}

	// Check endpoint provided container information
	if ep1.Info().Sandbox().Key() != sb.Key() {
		t.Fatalf("Endpoint Info returned unexpected sandbox key: %s", sb.Key())
	}

	// Attempt retrieval of endpoint interfaces statistics
	stats, err := sb.Statistics()
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := stats["eth0"]; !ok {
		t.Fatalf("Did not find eth0 statistics")
	}

	// Now test the container joining another network
	n2, err := createTestNetwork(bridgeNetType, "testnetwork2",
		options.Generic{
			netlabel.GenericData: options.Generic{
				"BridgeName": "testnetwork2",
			},
		}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := n2.Delete(); err != nil {
			t.Fatal(err)
		}
	}()

	ep2, err := n2.CreateEndpoint("ep2")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := ep2.Delete(false); err != nil {
			t.Fatal(err)
		}
	}()

	err = ep2.Join(sb)
	if err != nil {
		t.Fatal(err)
	}
	runtime.LockOSThread()
	defer func() {
		err = ep2.Leave(sb)
		runtime.LockOSThread()
		if err != nil {
			t.Fatal(err)
		}
	}()

	if ep1.Info().Sandbox().Key() != ep2.Info().Sandbox().Key() {
		t.Fatalf("ep1 and ep2 returned different container sandbox key")
	}

	checkSandbox(t, info)
}

type fakeSandbox struct{}

func (f *fakeSandbox) ID() string {
	return "fake sandbox"
}

func (f *fakeSandbox) ContainerID() string {
	return ""
}

func (f *fakeSandbox) Key() string {
	return "fake key"
}

func (f *fakeSandbox) Labels() map[string]interface{} {
	return nil
}

func (f *fakeSandbox) Statistics() (map[string]*types.InterfaceStatistics, error) {
	return nil, nil
}

func (f *fakeSandbox) Refresh(opts ...libnetwork.SandboxOption) error {
	return nil
}

func (f *fakeSandbox) Delete() error {
	return nil
}

func (f *fakeSandbox) Rename(name string) error {
	return nil
}

func (f *fakeSandbox) SetKey(key string) error {
	return nil
}

func (f *fakeSandbox) ResolveName(name string) net.IP {
	return nil
}

func (f *fakeSandbox) ResolveIP(ip string) string {
	return ""
}

func (f *fakeSandbox) Endpoints() []libnetwork.Endpoint {
	return nil
}

func TestExternalKey(t *testing.T) {
	externalKeyTest(t, false)
}

func TestExternalKeyWithReexec(t *testing.T) {
	externalKeyTest(t, true)
}

func externalKeyTest(t *testing.T, reexec bool) {
	if !testutils.IsRunningInContainer() {
		defer testutils.SetupTestOSContext(t)()
	}

	n, err := createTestNetwork(bridgeNetType, "testnetwork", options.Generic{
		netlabel.GenericData: options.Generic{
			"BridgeName": "testnetwork",
		},
	}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := n.Delete(); err != nil {
			t.Fatal(err)
		}
	}()

	ep, err := n.CreateEndpoint("ep1")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		err = ep.Delete(false)
		if err != nil {
			t.Fatal(err)
		}
	}()

	ep2, err := n.CreateEndpoint("ep2")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		err = ep2.Delete(false)
		if err != nil {
			t.Fatal(err)
		}
	}()

	cnt, err := controller.NewSandbox(containerID,
		libnetwork.OptionHostname("test"),
		libnetwork.OptionDomainname("docker.io"),
		libnetwork.OptionUseExternalKey(),
		libnetwork.OptionExtraHost("web", "192.168.0.1"))
	defer func() {
		if err := cnt.Delete(); err != nil {
			t.Fatal(err)
		}
		osl.GC()
	}()

	// Join endpoint to sandbox before SetKey
	err = ep.Join(cnt)
	runtime.LockOSThread()
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		err = ep.Leave(cnt)
		runtime.LockOSThread()
		if err != nil {
			t.Fatal(err)
		}
	}()

	sbox := ep.Info().Sandbox()
	if sbox == nil {
		t.Fatalf("Expected to have a valid Sandbox")
	}

	if reexec {
		err := reexecSetKey("this-must-fail", containerID, controller.ID())
		if err == nil {
			t.Fatalf("SetExternalKey must fail if the corresponding namespace is not created")
		}
	} else {
		// Setting an non-existing key (namespace) must fail
		if err := sbox.SetKey("this-must-fail"); err == nil {
			t.Fatalf("Setkey must fail if the corresponding namespace is not created")
		}
	}

	// Create a new OS sandbox using the osl API before using it in SetKey
	if extOsBox, err := osl.NewSandbox("ValidKey", true); err != nil {
		t.Fatalf("Failed to create new osl sandbox")
	} else {
		defer func() {
			if err := extOsBox.Destroy(); err != nil {
				log.Warnf("Failed to remove os sandbox: %v", err)
			}
		}()
	}

	if reexec {
		err := reexecSetKey("ValidKey", containerID, controller.ID())
		if err != nil {
			t.Fatalf("SetExternalKey failed with %v", err)
		}
	} else {
		if err := sbox.SetKey("ValidKey"); err != nil {
			t.Fatalf("Setkey failed with %v", err)
		}
	}

	// Join endpoint to sandbox after SetKey
	err = ep2.Join(sbox)
	if err != nil {
		t.Fatal(err)
	}
	runtime.LockOSThread()
	defer func() {
		err = ep2.Leave(sbox)
		runtime.LockOSThread()
		if err != nil {
			t.Fatal(err)
		}
	}()

	if ep.Info().Sandbox().Key() != ep2.Info().Sandbox().Key() {
		t.Fatalf("ep1 and ep2 returned different container sandbox key")
	}

	checkSandbox(t, ep.Info())
}

func reexecSetKey(key string, containerID string, controllerID string) error {
	var (
		state libcontainer.State
		b     []byte
		err   error
	)

	state.NamespacePaths = make(map[configs.NamespaceType]string)
	state.NamespacePaths[configs.NamespaceType("NEWNET")] = key
	if b, err = json.Marshal(state); err != nil {
		return err
	}
	cmd := &exec.Cmd{
		Path:   reexec.Self(),
		Args:   append([]string{"libnetwork-setkey"}, containerID, controllerID),
		Stdin:  strings.NewReader(string(b)),
		Stdout: os.Stdout,
		Stderr: os.Stderr,
	}
	return cmd.Run()
}

func TestEndpointDeleteWithActiveContainer(t *testing.T) {
	if !testutils.IsRunningInContainer() {
		defer testutils.SetupTestOSContext(t)()
	}

	n, err := createTestNetwork(bridgeNetType, "testnetwork", options.Generic{
		netlabel.GenericData: options.Generic{
			"BridgeName": "testnetwork",
		},
	}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := n.Delete(); err != nil {
			t.Fatal(err)
		}
	}()

	ep, err := n.CreateEndpoint("ep1")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		err = ep.Delete(false)
		if err != nil {
			t.Fatal(err)
		}
	}()

	cnt, err := controller.NewSandbox(containerID,
		libnetwork.OptionHostname("test"),
		libnetwork.OptionDomainname("docker.io"),
		libnetwork.OptionExtraHost("web", "192.168.0.1"))
	defer func() {
		if err := cnt.Delete(); err != nil {
			t.Fatal(err)
		}
	}()

	err = ep.Join(cnt)
	runtime.LockOSThread()
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		err = ep.Leave(cnt)
		runtime.LockOSThread()
		if err != nil {
			t.Fatal(err)
		}
	}()

	err = ep.Delete(false)
	if err == nil {
		t.Fatal("Expected to fail. But instead succeeded")
	}

	if _, ok := err.(*libnetwork.ActiveContainerError); !ok {
		t.Fatalf("Did not fail with expected error. Actual error: %v", err)
	}
}

func TestEndpointMultipleJoins(t *testing.T) {
	if !testutils.IsRunningInContainer() {
		defer testutils.SetupTestOSContext(t)()
	}

	n, err := createTestNetwork(bridgeNetType, "testmultiple", options.Generic{
		netlabel.GenericData: options.Generic{
			"BridgeName": "testmultiple",
		},
	}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := n.Delete(); err != nil {
			t.Fatal(err)
		}
	}()

	ep, err := n.CreateEndpoint("ep1")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := ep.Delete(false); err != nil {
			t.Fatal(err)
		}
	}()

	sbx1, err := controller.NewSandbox(containerID,
		libnetwork.OptionHostname("test"),
		libnetwork.OptionDomainname("docker.io"),
		libnetwork.OptionExtraHost("web", "192.168.0.1"))
	defer func() {
		if err := sbx1.Delete(); err != nil {
			t.Fatal(err)
		}
	}()

	sbx2, err := controller.NewSandbox("c2")
	defer func() {
		if err := sbx2.Delete(); err != nil {
			t.Fatal(err)
		}
		runtime.LockOSThread()
	}()

	err = ep.Join(sbx1)
	runtime.LockOSThread()
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		err = ep.Leave(sbx1)
		runtime.LockOSThread()
		if err != nil {
			t.Fatal(err)
		}
	}()

	err = ep.Join(sbx2)
	if err == nil {
		t.Fatal("Expected to fail multiple joins for the same endpoint")
	}

	if _, ok := err.(types.ForbiddenError); !ok {
		t.Fatalf("Failed with unexpected error type: %T. Desc: %s", err, err.Error())
	}

}

func TestLeaveAll(t *testing.T) {
	if !testutils.IsRunningInContainer() {
		defer testutils.SetupTestOSContext(t)()
	}

	n, err := createTestNetwork(bridgeNetType, "testnetwork", options.Generic{
		netlabel.GenericData: options.Generic{
			"BridgeName": "testnetwork",
		},
	}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		// If this goes through, it means cnt.Delete() effectively detached from all the endpoints
		if err := n.Delete(); err != nil {
			t.Fatal(err)
		}
	}()

	ep1, err := n.CreateEndpoint("ep1")
	if err != nil {
		t.Fatal(err)
	}

	ep2, err := n.CreateEndpoint("ep2")
	if err != nil {
		t.Fatal(err)
	}

	cnt, err := controller.NewSandbox("leaveall")
	if err != nil {
		t.Fatal(err)
	}

	err = ep1.Join(cnt)
	if err != nil {
		t.Fatalf("Failed to join ep1: %v", err)
	}
	runtime.LockOSThread()

	err = ep2.Join(cnt)
	if err != nil {
		t.Fatalf("Failed to join ep2: %v", err)
	}
	runtime.LockOSThread()

	err = cnt.Delete()
	if err != nil {
		t.Fatal(err)
	}
}

func TestontainerInvalidLeave(t *testing.T) {
	if !testutils.IsRunningInContainer() {
		defer testutils.SetupTestOSContext(t)()
	}

	n, err := createTestNetwork(bridgeNetType, "testnetwork", options.Generic{
		netlabel.GenericData: options.Generic{
			"BridgeName": "testnetwork",
		},
	}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := n.Delete(); err != nil {
			t.Fatal(err)
		}
	}()

	ep, err := n.CreateEndpoint("ep1")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := ep.Delete(false); err != nil {
			t.Fatal(err)
		}
	}()

	cnt, err := controller.NewSandbox(containerID,
		libnetwork.OptionHostname("test"),
		libnetwork.OptionDomainname("docker.io"),
		libnetwork.OptionExtraHost("web", "192.168.0.1"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := cnt.Delete(); err != nil {
			t.Fatal(err)
		}
	}()

	err = ep.Leave(cnt)
	if err == nil {
		t.Fatal("Expected to fail leave from an endpoint which has no active join")
	}
	if _, ok := err.(types.ForbiddenError); !ok {
		t.Fatalf("Failed with unexpected error type: %T. Desc: %s", err, err.Error())
	}

	if err := ep.Leave(nil); err == nil {
		t.Fatalf("Expected to fail leave nil Sandbox")
	}
	if _, ok := err.(types.BadRequestError); !ok {
		t.Fatalf("Unexpected error type returned: %T", err)
	}

	fsbx := &fakeSandbox{}
	if err = ep.Leave(fsbx); err == nil {
		t.Fatalf("Expected to fail leave with invalid Sandbox")
	}
	if _, ok := err.(types.BadRequestError); !ok {
		t.Fatalf("Unexpected error type returned: %T", err)
	}
}

func TestEndpointUpdateParent(t *testing.T) {
	if !testutils.IsRunningInContainer() {
		defer testutils.SetupTestOSContext(t)()
	}

	n, err := createTestNetwork("bridge", "testnetwork", options.Generic{
		netlabel.GenericData: options.Generic{
			"BridgeName": "testnetwork",
		},
	}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := n.Delete(); err != nil {
			t.Fatal(err)
		}
	}()

	ep1, err := n.CreateEndpoint("ep1")
	if err != nil {
		t.Fatal(err)
	}

	ep2, err := n.CreateEndpoint("ep2")
	if err != nil {
		t.Fatal(err)
	}

	sbx1, err := controller.NewSandbox(containerID,
		libnetwork.OptionHostname("test"),
		libnetwork.OptionDomainname("docker.io"),
		libnetwork.OptionExtraHost("web", "192.168.0.1"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := sbx1.Delete(); err != nil {
			t.Fatal(err)
		}
	}()

	sbx2, err := controller.NewSandbox("c2",
		libnetwork.OptionHostname("test2"),
		libnetwork.OptionDomainname("docker.io"),
		libnetwork.OptionHostsPath("/var/lib/docker/test_network/container2/hosts"),
		libnetwork.OptionExtraHost("web", "192.168.0.2"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := sbx2.Delete(); err != nil {
			t.Fatal(err)
		}
	}()

	err = ep1.Join(sbx1)
	runtime.LockOSThread()
	if err != nil {
		t.Fatal(err)
	}

	err = ep2.Join(sbx2)
	runtime.LockOSThread()
	if err != nil {
		t.Fatal(err)
	}
}

func TestEnableIPv6(t *testing.T) {
	if !testutils.IsRunningInContainer() {
		defer testutils.SetupTestOSContext(t)()
	}

	tmpResolvConf := []byte("search pommesfrites.fr\nnameserver 12.34.56.78\nnameserver 2001:4860:4860::8888\n")
	expectedResolvConf := []byte("search pommesfrites.fr\nnameserver 127.0.0.11\nnameserver 2001:4860:4860::8888\noptions ndots:0\n")
	//take a copy of resolv.conf for restoring after test completes
	resolvConfSystem, err := ioutil.ReadFile("/etc/resolv.conf")
	if err != nil {
		t.Fatal(err)
	}
	//cleanup
	defer func() {
		if err := ioutil.WriteFile("/etc/resolv.conf", resolvConfSystem, 0644); err != nil {
			t.Fatal(err)
		}
	}()

	netOption := options.Generic{
		netlabel.EnableIPv6: true,
		netlabel.GenericData: options.Generic{
			"BridgeName": "testnetwork",
		},
	}
	ipamV6ConfList := []*libnetwork.IpamConf{{PreferredPool: "fe99::/64", Gateway: "fe99::9"}}

	n, err := createTestNetwork("bridge", "testnetwork", netOption, nil, ipamV6ConfList)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := n.Delete(); err != nil {
			t.Fatal(err)
		}
	}()

	ep1, err := n.CreateEndpoint("ep1")
	if err != nil {
		t.Fatal(err)
	}

	if err := ioutil.WriteFile("/etc/resolv.conf", tmpResolvConf, 0644); err != nil {
		t.Fatal(err)
	}

	resolvConfPath := "/tmp/libnetwork_test/resolv.conf"
	defer os.Remove(resolvConfPath)

	sb, err := controller.NewSandbox(containerID, libnetwork.OptionResolvConfPath(resolvConfPath))
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := sb.Delete(); err != nil {
			t.Fatal(err)
		}
	}()

	err = ep1.Join(sb)
	if err != nil {
		t.Fatal(err)
	}

	content, err := ioutil.ReadFile(resolvConfPath)
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(content, expectedResolvConf) {
		t.Fatalf("Expected:\n%s\nGot:\n%s", string(expectedResolvConf), string(content))
	}

	if err != nil {
		t.Fatal(err)
	}
}

func TestResolvConfHost(t *testing.T) {
	if !testutils.IsRunningInContainer() {
		defer testutils.SetupTestOSContext(t)()
	}

	tmpResolvConf := []byte("search localhost.net\nnameserver 127.0.0.1\nnameserver 2001:4860:4860::8888\n")

	//take a copy of resolv.conf for restoring after test completes
	resolvConfSystem, err := ioutil.ReadFile("/etc/resolv.conf")
	if err != nil {
		t.Fatal(err)
	}
	//cleanup
	defer func() {
		if err := ioutil.WriteFile("/etc/resolv.conf", resolvConfSystem, 0644); err != nil {
			t.Fatal(err)
		}
	}()

	n, err := controller.NetworkByName("testhost")
	if err != nil {
		t.Fatal(err)
	}

	ep1, err := n.CreateEndpoint("ep1", libnetwork.CreateOptionDisableResolution())
	if err != nil {
		t.Fatal(err)
	}

	if err := ioutil.WriteFile("/etc/resolv.conf", tmpResolvConf, 0644); err != nil {
		t.Fatal(err)
	}

	resolvConfPath := "/tmp/libnetwork_test/resolv.conf"
	defer os.Remove(resolvConfPath)

	sb, err := controller.NewSandbox(containerID,
		libnetwork.OptionResolvConfPath(resolvConfPath),
		libnetwork.OptionOriginResolvConfPath("/etc/resolv.conf"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := sb.Delete(); err != nil {
			t.Fatal(err)
		}
	}()

	err = ep1.Join(sb)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		err = ep1.Leave(sb)
		if err != nil {
			t.Fatal(err)
		}
	}()

	finfo, err := os.Stat(resolvConfPath)
	if err != nil {
		t.Fatal(err)
	}

	fmode := (os.FileMode)(0644)
	if finfo.Mode() != fmode {
		t.Fatalf("Expected file mode %s, got %s", fmode.String(), finfo.Mode().String())
	}

	content, err := ioutil.ReadFile(resolvConfPath)
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(content, tmpResolvConf) {
		t.Fatalf("Expected:\n%s\nGot:\n%s", string(tmpResolvConf), string(content))
	}
}

func TestResolvConf(t *testing.T) {
	if !testutils.IsRunningInContainer() {
		defer testutils.SetupTestOSContext(t)()
	}

	tmpResolvConf1 := []byte("search pommesfrites.fr\nnameserver 12.34.56.78\nnameserver 2001:4860:4860::8888\n")
	tmpResolvConf2 := []byte("search pommesfrites.fr\nnameserver 112.34.56.78\nnameserver 2001:4860:4860::8888\n")
	expectedResolvConf1 := []byte("search pommesfrites.fr\nnameserver 127.0.0.11\noptions ndots:0\n")
	tmpResolvConf3 := []byte("search pommesfrites.fr\nnameserver 113.34.56.78\n")

	//take a copy of resolv.conf for restoring after test completes
	resolvConfSystem, err := ioutil.ReadFile("/etc/resolv.conf")
	if err != nil {
		t.Fatal(err)
	}
	//cleanup
	defer func() {
		if err := ioutil.WriteFile("/etc/resolv.conf", resolvConfSystem, 0644); err != nil {
			t.Fatal(err)
		}
	}()

	netOption := options.Generic{
		netlabel.GenericData: options.Generic{
			"BridgeName": "testnetwork",
		},
	}
	n, err := createTestNetwork("bridge", "testnetwork", netOption, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := n.Delete(); err != nil {
			t.Fatal(err)
		}
	}()

	ep, err := n.CreateEndpoint("ep")
	if err != nil {
		t.Fatal(err)
	}

	if err := ioutil.WriteFile("/etc/resolv.conf", tmpResolvConf1, 0644); err != nil {
		t.Fatal(err)
	}

	resolvConfPath := "/tmp/libnetwork_test/resolv.conf"
	defer os.Remove(resolvConfPath)

	sb1, err := controller.NewSandbox(containerID, libnetwork.OptionResolvConfPath(resolvConfPath))
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := sb1.Delete(); err != nil {
			t.Fatal(err)
		}
	}()

	err = ep.Join(sb1)
	runtime.LockOSThread()
	if err != nil {
		t.Fatal(err)
	}

	finfo, err := os.Stat(resolvConfPath)
	if err != nil {
		t.Fatal(err)
	}

	fmode := (os.FileMode)(0644)
	if finfo.Mode() != fmode {
		t.Fatalf("Expected file mode %s, got %s", fmode.String(), finfo.Mode().String())
	}

	content, err := ioutil.ReadFile(resolvConfPath)
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(content, expectedResolvConf1) {
		fmt.Printf("\n%v\n%v\n", expectedResolvConf1, content)
		t.Fatalf("Expected:\n%s\nGot:\n%s", string(expectedResolvConf1), string(content))
	}

	err = ep.Leave(sb1)
	runtime.LockOSThread()
	if err != nil {
		t.Fatal(err)
	}

	if err := ioutil.WriteFile("/etc/resolv.conf", tmpResolvConf2, 0644); err != nil {
		t.Fatal(err)
	}

	sb2, err := controller.NewSandbox(containerID+"_2", libnetwork.OptionResolvConfPath(resolvConfPath))
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := sb2.Delete(); err != nil {
			t.Fatal(err)
		}
	}()

	err = ep.Join(sb2)
	runtime.LockOSThread()
	if err != nil {
		t.Fatal(err)
	}

	content, err = ioutil.ReadFile(resolvConfPath)
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(content, expectedResolvConf1) {
		t.Fatalf("Expected:\n%s\nGot:\n%s", string(expectedResolvConf1), string(content))
	}

	if err := ioutil.WriteFile(resolvConfPath, tmpResolvConf3, 0644); err != nil {
		t.Fatal(err)
	}

	err = ep.Leave(sb2)
	runtime.LockOSThread()
	if err != nil {
		t.Fatal(err)
	}

	err = ep.Join(sb2)
	runtime.LockOSThread()
	if err != nil {
		t.Fatal(err)
	}

	content, err = ioutil.ReadFile(resolvConfPath)
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(content, tmpResolvConf3) {
		t.Fatalf("Expected:\n%s\nGot:\n%s", string(tmpResolvConf3), string(content))
	}
}

func TestInvalidRemoteDriver(t *testing.T) {
	if !testutils.IsRunningInContainer() {
		t.Skip("Skipping test when not running inside a Container")
	}

	mux := http.NewServeMux()
	server := httptest.NewServer(mux)
	if server == nil {
		t.Fatal("Failed to start a HTTP Server")
	}
	defer server.Close()

	type pluginRequest struct {
		name string
	}

	mux.HandleFunc("/Plugin.Activate", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/vnd.docker.plugins.v1+json")
		fmt.Fprintln(w, `{"Implements": ["InvalidDriver"]}`)
	})

	if err := os.MkdirAll("/etc/docker/plugins", 0755); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := os.RemoveAll("/etc/docker/plugins"); err != nil {
			t.Fatal(err)
		}
	}()

	if err := ioutil.WriteFile("/etc/docker/plugins/invalid-network-driver.spec", []byte(server.URL), 0644); err != nil {
		t.Fatal(err)
	}

	ctrlr, err := libnetwork.New()
	if err != nil {
		t.Fatal(err)
	}
	defer ctrlr.Stop()

	_, err = ctrlr.NewNetwork("invalid-network-driver", "dummy",
		libnetwork.NetworkOptionGeneric(getEmptyGenericOption()))
	if err == nil {
		t.Fatal("Expected to fail. But instead succeeded")
	}

	if err != plugins.ErrNotImplements {
		t.Fatalf("Did not fail with expected error. Actual error: %v", err)
	}
}

func TestValidRemoteDriver(t *testing.T) {
	if !testutils.IsRunningInContainer() {
		t.Skip("Skipping test when not running inside a Container")
	}

	mux := http.NewServeMux()
	server := httptest.NewServer(mux)
	if server == nil {
		t.Fatal("Failed to start a HTTP Server")
	}
	defer server.Close()

	type pluginRequest struct {
		name string
	}

	mux.HandleFunc("/Plugin.Activate", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/vnd.docker.plugins.v1+json")
		fmt.Fprintf(w, `{"Implements": ["%s"]}`, driverapi.NetworkPluginEndpointType)
	})
	mux.HandleFunc(fmt.Sprintf("/%s.CreateNetwork", driverapi.NetworkPluginEndpointType), func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/vnd.docker.plugins.v1+json")
		fmt.Fprintf(w, "null")
	})

	if err := os.MkdirAll("/etc/docker/plugins", 0755); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := os.RemoveAll("/etc/docker/plugins"); err != nil {
			t.Fatal(err)
		}
	}()

	if err := ioutil.WriteFile("/etc/docker/plugins/valid-network-driver.spec", []byte(server.URL), 0644); err != nil {
		t.Fatal(err)
	}

	n, err := controller.NewNetwork("valid-network-driver", "dummy",
		libnetwork.NetworkOptionGeneric(getEmptyGenericOption()))
	if err != nil {
		// Only fail if we could not find the plugin driver
		if _, ok := err.(types.NotFoundError); ok {
			t.Fatal(err)
		}
		return
	}
	defer func() {
		if err := n.Delete(); err != nil {
			t.Fatal(err)
		}
	}()
}

var (
	once   sync.Once
	start  = make(chan struct{})
	done   = make(chan chan struct{}, numThreads-1)
	origns = netns.None()
	testns = netns.None()
	sboxes = make([]libnetwork.Sandbox, numThreads)
)

const (
	iterCnt    = 25
	numThreads = 3
	first      = 1
	last       = numThreads
	debug      = false
)

func createGlobalInstance(t *testing.T) {
	var err error
	defer close(start)

	origns, err = netns.Get()
	if err != nil {
		t.Fatal(err)
	}

	if testutils.IsRunningInContainer() {
		testns = origns
	} else {
		testns, err = netns.New()
		if err != nil {
			t.Fatal(err)
		}
	}

	netOption := options.Generic{
		netlabel.GenericData: options.Generic{
			"BridgeName": "network",
		},
	}

	net1, err := controller.NetworkByName("testhost")
	if err != nil {
		t.Fatal(err)
	}

	net2, err := createTestNetwork("bridge", "network2", netOption, nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	_, err = net1.CreateEndpoint("pep1")
	if err != nil {
		t.Fatal(err)
	}

	_, err = net2.CreateEndpoint("pep2")
	if err != nil {
		t.Fatal(err)
	}

	_, err = net2.CreateEndpoint("pep3")
	if err != nil {
		t.Fatal(err)
	}

	if sboxes[first-1], err = controller.NewSandbox(fmt.Sprintf("%drace", first), libnetwork.OptionUseDefaultSandbox()); err != nil {
		t.Fatal(err)
	}
	for thd := first + 1; thd <= last; thd++ {
		if sboxes[thd-1], err = controller.NewSandbox(fmt.Sprintf("%drace", thd)); err != nil {
			t.Fatal(err)
		}
	}
}

func debugf(format string, a ...interface{}) (int, error) {
	if debug {
		return fmt.Printf(format, a...)
	}

	return 0, nil
}

func parallelJoin(t *testing.T, rc libnetwork.Sandbox, ep libnetwork.Endpoint, thrNumber int) {
	debugf("J%d.", thrNumber)
	var err error

	sb := sboxes[thrNumber-1]
	err = ep.Join(sb)

	runtime.LockOSThread()
	if err != nil {
		if _, ok := err.(types.ForbiddenError); !ok {
			t.Fatalf("thread %d: %v", thrNumber, err)
		}
		debugf("JE%d(%v).", thrNumber, err)
	}
	debugf("JD%d.", thrNumber)
}

func parallelLeave(t *testing.T, rc libnetwork.Sandbox, ep libnetwork.Endpoint, thrNumber int) {
	debugf("L%d.", thrNumber)
	var err error

	sb := sboxes[thrNumber-1]

	err = ep.Leave(sb)
	runtime.LockOSThread()
	if err != nil {
		if _, ok := err.(types.ForbiddenError); !ok {
			t.Fatalf("thread %d: %v", thrNumber, err)
		}
		debugf("LE%d(%v).", thrNumber, err)
	}
	debugf("LD%d.", thrNumber)
}

func runParallelTests(t *testing.T, thrNumber int) {
	var (
		ep  libnetwork.Endpoint
		sb  libnetwork.Sandbox
		err error
	)

	t.Parallel()

	pTest := flag.Lookup("test.parallel")
	if pTest == nil {
		t.Skip("Skipped because test.parallel flag not set;")
	}
	numParallel, err := strconv.Atoi(pTest.Value.String())
	if err != nil {
		t.Fatal(err)
	}
	if numParallel < numThreads {
		t.Skip("Skipped because t.parallel was less than ", numThreads)
	}

	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	if thrNumber == first {
		createGlobalInstance(t)
	}

	if thrNumber != first {
		select {
		case <-start:
		}

		thrdone := make(chan struct{})
		done <- thrdone
		defer close(thrdone)

		if thrNumber == last {
			defer close(done)
		}

		err = netns.Set(testns)
		if err != nil {
			t.Fatal(err)
		}
	}
	defer netns.Set(origns)

	net1, err := controller.NetworkByName("testhost")
	if err != nil {
		t.Fatal(err)
	}
	if net1 == nil {
		t.Fatal("Could not find testhost")
	}

	net2, err := controller.NetworkByName("network2")
	if err != nil {
		t.Fatal(err)
	}
	if net2 == nil {
		t.Fatal("Could not find network2")
	}

	epName := fmt.Sprintf("pep%d", thrNumber)

	if thrNumber == first {
		ep, err = net1.EndpointByName(epName)
	} else {
		ep, err = net2.EndpointByName(epName)
	}

	if err != nil {
		t.Fatal(err)
	}
	if ep == nil {
		t.Fatal("Got nil ep with no error")
	}

	cid := fmt.Sprintf("%drace", thrNumber)
	controller.WalkSandboxes(libnetwork.SandboxContainerWalker(&sb, cid))
	if sb == nil {
		t.Fatalf("Got nil sandbox for container: %s", cid)
	}

	for i := 0; i < iterCnt; i++ {
		parallelJoin(t, sb, ep, thrNumber)
		parallelLeave(t, sb, ep, thrNumber)
	}

	debugf("\n")

	err = sb.Delete()
	if err != nil {
		t.Fatal(err)
	}
	if thrNumber == first {
		for thrdone := range done {
			select {
			case <-thrdone:
			}
		}

		testns.Close()
		if err := net2.Delete(); err != nil {
			t.Fatal(err)
		}
	} else {
		err = ep.Delete(false)
		if err != nil {
			t.Fatal(err)
		}
	}
}

func TestParallel1(t *testing.T) {
	runParallelTests(t, 1)
}

func TestParallel2(t *testing.T) {
	runParallelTests(t, 2)
}

func TestParallel3(t *testing.T) {
	runParallelTests(t, 3)
}
