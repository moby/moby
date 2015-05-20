package libnetwork_test

import (
	"bytes"
	"flag"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"runtime"
	"strconv"
	"sync"
	"testing"

	log "github.com/Sirupsen/logrus"
	"github.com/docker/docker/pkg/plugins"
	"github.com/docker/docker/pkg/reexec"
	"github.com/docker/libnetwork"
	"github.com/docker/libnetwork/driverapi"
	"github.com/docker/libnetwork/netlabel"
	"github.com/docker/libnetwork/netutils"
	"github.com/docker/libnetwork/options"
	"github.com/vishvananda/netns"
)

const (
	bridgeNetType = "bridge"
	bridgeName    = "docker0"
)

func TestMain(m *testing.M) {
	if reexec.Init() {
		return
	}
	os.Exit(m.Run())
}

func createTestNetwork(networkType, networkName string, option options.Generic, netOption options.Generic) (libnetwork.Network, error) {
	controller, err := libnetwork.New()
	if err != nil {
		return nil, err
	}
	genericOption := make(map[string]interface{})
	genericOption[netlabel.GenericData] = option

	err = controller.ConfigureNetworkDriver(networkType, genericOption)
	if err != nil {
		return nil, err
	}

	network, err := controller.NewNetwork(networkType, networkName,
		libnetwork.NetworkOptionGeneric(netOption))
	if err != nil {
		return nil, err
	}

	return network, nil
}

func getEmptyGenericOption() map[string]interface{} {
	genericOption := make(map[string]interface{})
	genericOption[netlabel.GenericData] = options.Generic{}
	return genericOption
}

func getPortMapping() []netutils.PortBinding {
	return []netutils.PortBinding{
		netutils.PortBinding{Proto: netutils.TCP, Port: uint16(230), HostPort: uint16(23000)},
		netutils.PortBinding{Proto: netutils.UDP, Port: uint16(200), HostPort: uint16(22000)},
		netutils.PortBinding{Proto: netutils.TCP, Port: uint16(120), HostPort: uint16(12000)},
	}
}

func TestNull(t *testing.T) {
	network, err := createTestNetwork("null", "testnetwork", options.Generic{},
		options.Generic{})
	if err != nil {
		t.Fatal(err)
	}

	ep, err := network.CreateEndpoint("testep")
	if err != nil {
		t.Fatal(err)
	}

	_, err = ep.Join("null_container",
		libnetwork.JoinOptionHostname("test"),
		libnetwork.JoinOptionDomainname("docker.io"),
		libnetwork.JoinOptionExtraHost("web", "192.168.0.1"))
	if err != nil {
		t.Fatal(err)
	}

	err = ep.Leave("null_container")
	if err != nil {
		t.Fatal(err)
	}

	if err := ep.Delete(); err != nil {
		t.Fatal(err)
	}

	if err := network.Delete(); err != nil {
		t.Fatal(err)
	}
}

func TestHost(t *testing.T) {
	network, err := createTestNetwork("host", "testnetwork", options.Generic{}, options.Generic{})
	if err != nil {
		t.Fatal(err)
	}

	ep1, err := network.CreateEndpoint("testep1")
	if err != nil {
		t.Fatal(err)
	}

	_, err = ep1.Join("host_container1",
		libnetwork.JoinOptionHostname("test1"),
		libnetwork.JoinOptionDomainname("docker.io"),
		libnetwork.JoinOptionExtraHost("web", "192.168.0.1"),
		libnetwork.JoinOptionUseDefaultSandbox())
	if err != nil {
		t.Fatal(err)
	}

	ep2, err := network.CreateEndpoint("testep2")
	if err != nil {
		t.Fatal(err)
	}

	_, err = ep2.Join("host_container2",
		libnetwork.JoinOptionHostname("test2"),
		libnetwork.JoinOptionDomainname("docker.io"),
		libnetwork.JoinOptionExtraHost("web", "192.168.0.1"),
		libnetwork.JoinOptionUseDefaultSandbox())
	if err != nil {
		t.Fatal(err)
	}

	err = ep1.Leave("host_container1")
	if err != nil {
		t.Fatal(err)
	}

	err = ep2.Leave("host_container2")
	if err != nil {
		t.Fatal(err)
	}

	if err := ep1.Delete(); err != nil {
		t.Fatal(err)
	}

	if err := ep2.Delete(); err != nil {
		t.Fatal(err)
	}

	// Try to create another host endpoint and join/leave that.
	ep3, err := network.CreateEndpoint("testep3")
	if err != nil {
		t.Fatal(err)
	}

	_, err = ep3.Join("host_container3",
		libnetwork.JoinOptionHostname("test3"),
		libnetwork.JoinOptionDomainname("docker.io"),
		libnetwork.JoinOptionExtraHost("web", "192.168.0.1"),
		libnetwork.JoinOptionUseDefaultSandbox())
	if err != nil {
		t.Fatal(err)
	}

	err = ep3.Leave("host_container3")
	if err != nil {
		t.Fatal(err)
	}

	if err := ep3.Delete(); err != nil {
		t.Fatal(err)
	}

	if err := network.Delete(); err != nil {
		t.Fatal(err)
	}
}

func TestBridge(t *testing.T) {
	if !netutils.IsRunningInContainer() {
		defer netutils.SetupTestNetNS(t)()
	}

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
		"EnableIPForwarding": true,
	}

	netOption := options.Generic{
		"BridgeName":            bridgeName,
		"AddressIPv4":           subnet,
		"FixedCIDR":             cidr,
		"FixedCIDRv6":           cidrv6,
		"EnableIPv6":            true,
		"EnableIPTables":        true,
		"EnableIPMasquerade":    true,
		"EnableICC":             true,
		"AllowNonDefaultBridge": true}

	network, err := createTestNetwork(bridgeNetType, "testnetwork", option, netOption)
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
	pm, ok := pmd.([]netutils.PortBinding)
	if !ok {
		t.Fatalf("Unexpected format for port mapping in endpoint operational data")
	}
	if len(pm) != 3 {
		t.Fatalf("Incomplete data for port mapping in endpoint operational data: %d", len(pm))
	}

	if err := ep.Delete(); err != nil {
		t.Fatal(err)
	}

	if err := network.Delete(); err != nil {
		t.Fatal(err)
	}
}

func TestUnknownDriver(t *testing.T) {
	if !netutils.IsRunningInContainer() {
		defer netutils.SetupTestNetNS(t)()
	}

	_, err := createTestNetwork("unknowndriver", "testnetwork", options.Generic{}, options.Generic{})
	if err == nil {
		t.Fatal("Expected to fail. But instead succeeded")
	}

	if _, ok := err.(libnetwork.NetworkTypeError); !ok {
		t.Fatalf("Did not fail with expected error. Actual error: %v", err)
	}
}

func TestNilRemoteDriver(t *testing.T) {
	controller, err := libnetwork.New()
	if err != nil {
		t.Fatal(err)
	}

	_, err = controller.NewNetwork("framerelay", "dummy",
		libnetwork.NetworkOptionGeneric(getEmptyGenericOption()))
	if err == nil {
		t.Fatal("Expected to fail. But instead succeeded")
	}

	if err != plugins.ErrNotFound {
		t.Fatalf("Did not fail with expected error. Actual error: %v", err)
	}
}

func TestDuplicateNetwork(t *testing.T) {
	if !netutils.IsRunningInContainer() {
		defer netutils.SetupTestNetNS(t)()
	}

	controller, err := libnetwork.New()
	if err != nil {
		t.Fatal(err)
	}

	genericOption := make(map[string]interface{})
	genericOption[netlabel.GenericData] = options.Generic{}

	err = controller.ConfigureNetworkDriver(bridgeNetType, genericOption)
	if err != nil {
		t.Fatal(err)
	}

	_, err = controller.NewNetwork(bridgeNetType, "testnetwork",
		libnetwork.NetworkOptionGeneric(genericOption))
	if err != nil {
		t.Fatal(err)
	}

	_, err = controller.NewNetwork(bridgeNetType, "testnetwork")
	if err == nil {
		t.Fatal("Expected to fail. But instead succeeded")
	}

	if _, ok := err.(libnetwork.NetworkNameError); !ok {
		t.Fatalf("Did not fail with expected error. Actual error: %v", err)
	}
}

func TestNetworkName(t *testing.T) {
	if !netutils.IsRunningInContainer() {
		defer netutils.SetupTestNetNS(t)()
	}

	_, err := createTestNetwork(bridgeNetType, "", options.Generic{}, options.Generic{})
	if err == nil {
		t.Fatal("Expected to fail. But instead succeeded")
	}
	if err != libnetwork.ErrInvalidName {
		t.Fatal("Expected to fail with ErrInvalidName error")
	}

	networkName := "testnetwork"
	n, err := createTestNetwork(bridgeNetType, networkName, options.Generic{}, options.Generic{})
	if err != nil {
		t.Fatal(err)
	}

	if n.Name() != networkName {
		t.Fatalf("Expected network name %s, got %s", networkName, n.Name())
	}
}

func TestNetworkType(t *testing.T) {
	if !netutils.IsRunningInContainer() {
		defer netutils.SetupTestNetNS(t)()
	}

	n, err := createTestNetwork(bridgeNetType, "testnetwork", options.Generic{}, options.Generic{})
	if err != nil {
		t.Fatal(err)
	}

	if n.Type() != bridgeNetType {
		t.Fatalf("Expected network type %s, got %s", bridgeNetType, n.Type())
	}
}

func TestNetworkID(t *testing.T) {
	if !netutils.IsRunningInContainer() {
		defer netutils.SetupTestNetNS(t)()
	}

	n, err := createTestNetwork(bridgeNetType, "testnetwork", options.Generic{}, options.Generic{})
	if err != nil {
		t.Fatal(err)
	}

	if n.ID() == "" {
		t.Fatal("Expected non-empty network id")
	}
}

func TestDeleteNetworkWithActiveEndpoints(t *testing.T) {
	if !netutils.IsRunningInContainer() {
		defer netutils.SetupTestNetNS(t)()
	}

	option := options.Generic{
		"BridgeName":            bridgeName,
		"AllowNonDefaultBridge": true}

	network, err := createTestNetwork(bridgeNetType, "testnetwork", options.Generic{}, option)
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
	if err := ep.Delete(); err != nil {
		t.Fatal(err)
	}

	if err := network.Delete(); err != nil {
		t.Fatal(err)
	}
}

func TestUnknownNetwork(t *testing.T) {
	if !netutils.IsRunningInContainer() {
		defer netutils.SetupTestNetNS(t)()
	}

	option := options.Generic{
		"BridgeName":            bridgeName,
		"AllowNonDefaultBridge": true}

	network, err := createTestNetwork(bridgeNetType, "testnetwork", options.Generic{}, option)
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
	if !netutils.IsRunningInContainer() {
		defer netutils.SetupTestNetNS(t)()
	}

	ip, subnet, err := net.ParseCIDR("192.168.100.1/24")
	if err != nil {
		t.Fatal(err)
	}
	subnet.IP = ip

	option := options.Generic{
		"BridgeName":            bridgeName,
		"AddressIPv4":           subnet,
		"AllowNonDefaultBridge": true}

	network, err := createTestNetwork(bridgeNetType, "testnetwork", options.Generic{}, option)
	if err != nil {
		t.Fatal(err)
	}

	_, err = network.CreateEndpoint("")
	if err == nil {
		t.Fatal("Expected to fail. But instead succeeded")
	}
	if err != libnetwork.ErrInvalidName {
		t.Fatal("Expected to fail with ErrInvalidName error")
	}

	ep, err := network.CreateEndpoint("testep")
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

func TestNetworkEndpointsWalkers(t *testing.T) {
	if !netutils.IsRunningInContainer() {
		defer netutils.SetupTestNetNS(t)()
	}

	controller, err := libnetwork.New()
	if err != nil {
		t.Fatal(err)
	}

	err = controller.ConfigureNetworkDriver(bridgeNetType, getEmptyGenericOption())
	if err != nil {
		t.Fatal(err)
	}

	// Create network 1 and add 2 endpoint: ep11, ep12
	net1, err := controller.NewNetwork(bridgeNetType, "network1")
	if err != nil {
		t.Fatal(err)
	}
	ep11, err := net1.CreateEndpoint("ep11")
	if err != nil {
		t.Fatal(err)
	}
	ep12, err := net1.CreateEndpoint("ep12")
	if err != nil {
		t.Fatal(err)
	}

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

	// Look for network named "network1"
	netName = "network1"
	controller.WalkNetworks(nwWlk)
	if netWanted == nil {
		t.Fatal(err)
	}
	if net1 != netWanted {
		t.Fatal(err)
	}
}

func TestControllerQuery(t *testing.T) {
	if !netutils.IsRunningInContainer() {
		defer netutils.SetupTestNetNS(t)()
	}

	controller, err := libnetwork.New()
	if err != nil {
		t.Fatal(err)
	}

	err = controller.ConfigureNetworkDriver(bridgeNetType, getEmptyGenericOption())
	if err != nil {
		t.Fatal(err)
	}

	// Create network 1
	net1, err := controller.NewNetwork(bridgeNetType, "network1")
	if err != nil {
		t.Fatal(err)
	}

	_, err = controller.NetworkByName("")
	if err == nil {
		t.Fatalf("NetworkByName() succeeded with invalid target name")
	}
	if err != libnetwork.ErrInvalidName {
		t.Fatalf("NetworkByName() failed with unexpected error: %v", err)
	}

	_, err = controller.NetworkByID("")
	if err == nil {
		t.Fatalf("NetworkByID() succeeded with invalid target id")
	}
	if err != libnetwork.ErrInvalidID {
		t.Fatalf("NetworkByID() failed with unexpected error: %v", err)
	}

	g, err := controller.NetworkByID("network1")
	if err == nil {
		t.Fatalf("Unexpected success for NetworkByID(): %v", g)
	}
	if err != libnetwork.ErrNoSuchNetwork {
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
	if net1 != g {
		t.Fatalf("NetworkByID() returned unexpected element: %v", g)
	}
}

func TestNetworkQuery(t *testing.T) {
	if !netutils.IsRunningInContainer() {
		defer netutils.SetupTestNetNS(t)()
	}

	controller, err := libnetwork.New()
	if err != nil {
		t.Fatal(err)
	}

	err = controller.ConfigureNetworkDriver(bridgeNetType, getEmptyGenericOption())
	if err != nil {
		t.Fatal(err)
	}

	// Create network 1 and add 2 endpoint: ep11, ep12
	net1, err := controller.NewNetwork(bridgeNetType, "network1")
	if err != nil {
		t.Fatal(err)
	}
	ep11, err := net1.CreateEndpoint("ep11")
	if err != nil {
		t.Fatal(err)
	}
	ep12, err := net1.CreateEndpoint("ep12")
	if err != nil {
		t.Fatal(err)
	}

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
	if err != libnetwork.ErrInvalidName {
		t.Fatalf("EndpointByName() failed with unexpected error: %v", err)
	}

	e, err = net1.EndpointByName("IamNotAnEndpoint")
	if err == nil {
		t.Fatalf("EndpointByName() succeeded with unknown target name")
	}
	if err != libnetwork.ErrNoSuchEndpoint {
		t.Fatal(err)
	}
	if e != nil {
		t.Fatalf("EndpointByName(): expected nil, got %v", e)
	}

	e, err = net1.EndpointByID(ep12.ID())
	if err != nil {
		t.Fatal(err)
	}
	if ep12 != e {
		t.Fatalf("EndpointByID() returned %v instead of %v", e, ep12)
	}

	e, err = net1.EndpointByID("")
	if err == nil {
		t.Fatalf("EndpointByID() succeeded with invalid target id")
	}
	if err != libnetwork.ErrInvalidID {
		t.Fatalf("EndpointByID() failed with unexpected error: %v", err)
	}
}

const containerID = "valid_container"

func TestEndpointJoin(t *testing.T) {
	if !netutils.IsRunningInContainer() {
		defer netutils.SetupTestNetNS(t)()
	}

	n, err := createTestNetwork(bridgeNetType, "testnetwork", options.Generic{}, options.Generic{})
	if err != nil {
		t.Fatal(err)
	}

	ep, err := n.CreateEndpoint("ep1")
	if err != nil {
		t.Fatal(err)
	}

	// Validate if ep.Info() only gives me IP address info and not names and gateway during CreateEndpoint()
	info := ep.Info()

	for _, iface := range info.InterfaceList() {
		if iface.Address().IP.To4() == nil {
			t.Fatalf("Invalid IP address returned: %v", iface.Address())
		}
	}

	if info.Gateway().To4() != nil {
		t.Fatalf("Expected empty gateway for an empty endpoint. Instead found a gateway: %v", info.Gateway())
	}

	if info.SandboxKey() != "" {
		t.Fatalf("Expected an empty sandbox key for an empty endpoint. Instead found a non-empty sandbox key: %s", info.SandboxKey())
	}

	_, err = ep.Join(containerID,
		libnetwork.JoinOptionHostname("test"),
		libnetwork.JoinOptionDomainname("docker.io"),
		libnetwork.JoinOptionExtraHost("web", "192.168.0.1"))
	if err != nil {
		t.Fatal(err)
	}

	defer func() {
		err = ep.Leave(containerID)
		if err != nil {
			t.Fatal(err)
		}
	}()

	// Validate if ep.Info() only gives valid gateway and sandbox key after has container has joined.
	info = ep.Info()
	if info.Gateway().To4() == nil {
		t.Fatalf("Expected a valid gateway for a joined endpoint. Instead found an invalid gateway: %v", info.Gateway())
	}

	if info.SandboxKey() == "" {
		t.Fatalf("Expected an non-empty sandbox key for a joined endpoint. Instead found a empty sandbox key")
	}
}

func TestEndpointJoinInvalidContainerId(t *testing.T) {
	if !netutils.IsRunningInContainer() {
		defer netutils.SetupTestNetNS(t)()
	}

	n, err := createTestNetwork(bridgeNetType, "testnetwork", options.Generic{}, options.Generic{})
	if err != nil {
		t.Fatal(err)
	}

	ep, err := n.CreateEndpoint("ep1")
	if err != nil {
		t.Fatal(err)
	}

	_, err = ep.Join("")
	if err == nil {
		t.Fatal("Expected to fail join with empty container id string")
	}

	if _, ok := err.(libnetwork.InvalidContainerIDError); !ok {
		t.Fatalf("Failed for unexpected reason: %v", err)
	}
}

func TestEndpointDeleteWithActiveContainer(t *testing.T) {
	if !netutils.IsRunningInContainer() {
		defer netutils.SetupTestNetNS(t)()
	}

	n, err := createTestNetwork(bridgeNetType, "testnetwork", options.Generic{}, options.Generic{})
	if err != nil {
		t.Fatal(err)
	}

	ep, err := n.CreateEndpoint("ep1")
	if err != nil {
		t.Fatal(err)
	}

	_, err = ep.Join(containerID,
		libnetwork.JoinOptionHostname("test"),
		libnetwork.JoinOptionDomainname("docker.io"),
		libnetwork.JoinOptionExtraHost("web", "192.168.0.1"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		err = ep.Leave(containerID)
		if err != nil {
			t.Fatal(err)
		}

		err = ep.Delete()
		if err != nil {
			t.Fatal(err)
		}
	}()

	err = ep.Delete()
	if err == nil {
		t.Fatal("Expected to fail. But instead succeeded")
	}

	if _, ok := err.(*libnetwork.ActiveContainerError); !ok {
		t.Fatalf("Did not fail with expected error. Actual error: %v", err)
	}
}

func TestEndpointMultipleJoins(t *testing.T) {
	if !netutils.IsRunningInContainer() {
		defer netutils.SetupTestNetNS(t)()
	}

	n, err := createTestNetwork(bridgeNetType, "testnetwork", options.Generic{}, options.Generic{})
	if err != nil {
		t.Fatal(err)
	}

	ep, err := n.CreateEndpoint("ep1")
	if err != nil {
		t.Fatal(err)
	}

	_, err = ep.Join(containerID,
		libnetwork.JoinOptionHostname("test"),
		libnetwork.JoinOptionDomainname("docker.io"),
		libnetwork.JoinOptionExtraHost("web", "192.168.0.1"))

	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		err = ep.Leave(containerID)
		if err != nil {
			t.Fatal(err)
		}
	}()

	_, err = ep.Join("container2")
	if err == nil {
		t.Fatal("Expected to fail multiple joins for the same endpoint")
	}

	if err != libnetwork.ErrInvalidJoin {
		t.Fatalf("Failed for unexpected reason: %v", err)
	}
}

func TestEndpointInvalidLeave(t *testing.T) {
	if !netutils.IsRunningInContainer() {
		defer netutils.SetupTestNetNS(t)()
	}

	n, err := createTestNetwork(bridgeNetType, "testnetwork", options.Generic{}, options.Generic{})
	if err != nil {
		t.Fatal(err)
	}

	ep, err := n.CreateEndpoint("ep1")
	if err != nil {
		t.Fatal(err)
	}

	err = ep.Leave(containerID)
	if err == nil {
		t.Fatal("Expected to fail leave from an endpoint which has no active join")
	}

	if _, ok := err.(libnetwork.InvalidContainerIDError); !ok {
		if err != libnetwork.ErrNoContainer {
			t.Fatalf("Failed for unexpected reason: %v", err)
		}
	}

	_, err = ep.Join(containerID,
		libnetwork.JoinOptionHostname("test"),
		libnetwork.JoinOptionDomainname("docker.io"),
		libnetwork.JoinOptionExtraHost("web", "192.168.0.1"))

	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		err = ep.Leave(containerID)
		if err != nil {
			t.Fatal(err)
		}
	}()

	err = ep.Leave("")
	if err == nil {
		t.Fatal("Expected to fail leave with empty container id")
	}

	if _, ok := err.(libnetwork.InvalidContainerIDError); !ok {
		t.Fatalf("Failed for unexpected reason: %v", err)
	}

	err = ep.Leave("container2")
	if err == nil {
		t.Fatal("Expected to fail leave with wrong container id")
	}

	if _, ok := err.(libnetwork.InvalidContainerIDError); !ok {
		t.Fatalf("Failed for unexpected reason: %v", err)
	}

}

func TestEndpointUpdateParent(t *testing.T) {
	if !netutils.IsRunningInContainer() {
		defer netutils.SetupTestNetNS(t)()
	}

	n, err := createTestNetwork("bridge", "testnetwork", options.Generic{}, options.Generic{})
	if err != nil {
		t.Fatal(err)
	}

	ep1, err := n.CreateEndpoint("ep1", nil)
	if err != nil {
		t.Fatal(err)
	}

	_, err = ep1.Join(containerID,
		libnetwork.JoinOptionHostname("test1"),
		libnetwork.JoinOptionDomainname("docker.io"),
		libnetwork.JoinOptionExtraHost("web", "192.168.0.1"))

	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		err = ep1.Leave(containerID)
		if err != nil {
			t.Fatal(err)
		}
	}()

	ep2, err := n.CreateEndpoint("ep2", nil)
	if err != nil {
		t.Fatal(err)
	}

	_, err = ep2.Join("container2",
		libnetwork.JoinOptionHostname("test2"),
		libnetwork.JoinOptionDomainname("docker.io"),
		libnetwork.JoinOptionHostsPath("/var/lib/docker/test_network/container2/hosts"),
		libnetwork.JoinOptionParentUpdate(ep1.ID(), "web", "192.168.0.2"))

	if err != nil {
		t.Fatal(err)
	}

	defer func() {
		err = ep2.Leave("container2")
		if err != nil {
			t.Fatal(err)
		}
	}()

}

func TestEnableIPv6(t *testing.T) {
	if !netutils.IsRunningInContainer() {
		defer netutils.SetupTestNetNS(t)()
	}

	tmpResolvConf := []byte("search pommesfrites.fr\nnameserver 12.34.56.78\nnameserver 2001:4860:4860::8888")
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

	ip, cidrv6, err := net.ParseCIDR("fe80::1/64")
	if err != nil {
		t.Fatal(err)
	}
	cidrv6.IP = ip

	netOption := options.Generic{
		netlabel.EnableIPv6: true,
		netlabel.GenericData: options.Generic{
			"FixedCIDRv6": cidrv6,
		},
	}

	n, err := createTestNetwork("bridge", "testnetwork", options.Generic{}, netOption)
	if err != nil {
		t.Fatal(err)
	}

	ep1, err := n.CreateEndpoint("ep1", nil)
	if err != nil {
		t.Fatal(err)
	}

	if err := ioutil.WriteFile("/etc/resolv.conf", tmpResolvConf, 0644); err != nil {
		t.Fatal(err)
	}

	resolvConfPath := "/tmp/libnetwork_test/resolv.conf"
	defer os.Remove(resolvConfPath)

	_, err = ep1.Join(containerID,
		libnetwork.JoinOptionResolvConfPath(resolvConfPath))

	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		err = ep1.Leave(containerID)
		if err != nil {
			t.Fatal(err)
		}
	}()

	content, err := ioutil.ReadFile(resolvConfPath)
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(content, tmpResolvConf) {
		t.Fatalf("Expected %s, Got %s", string(tmpResolvConf), string(content))
	}

	if err != nil {
		t.Fatal(err)
	}
}

func TestResolvConf(t *testing.T) {
	if !netutils.IsRunningInContainer() {
		defer netutils.SetupTestNetNS(t)()
	}

	tmpResolvConf1 := []byte("search pommesfrites.fr\nnameserver 12.34.56.78\nnameserver 2001:4860:4860::8888")
	expectedResolvConf1 := []byte("search pommesfrites.fr\nnameserver 12.34.56.78\n")
	tmpResolvConf2 := []byte("search pommesfrites.fr\nnameserver 112.34.56.78\nnameserver 2001:4860:4860::8888")
	expectedResolvConf2 := []byte("search pommesfrites.fr\nnameserver 112.34.56.78\n")
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

	n, err := createTestNetwork("bridge", "testnetwork", options.Generic{}, options.Generic{})
	if err != nil {
		t.Fatal(err)
	}

	ep1, err := n.CreateEndpoint("ep1", nil)
	if err != nil {
		t.Fatal(err)
	}

	if err := ioutil.WriteFile("/etc/resolv.conf", tmpResolvConf1, 0644); err != nil {
		t.Fatal(err)
	}

	resolvConfPath := "/tmp/libnetwork_test/resolv.conf"
	defer os.Remove(resolvConfPath)

	_, err = ep1.Join(containerID,
		libnetwork.JoinOptionResolvConfPath(resolvConfPath))
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		err = ep1.Leave(containerID)
		if err != nil {
			t.Fatal(err)
		}
	}()

	content, err := ioutil.ReadFile(resolvConfPath)
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(content, expectedResolvConf1) {
		t.Fatalf("Expected %s, Got %s", string(expectedResolvConf1), string(content))
	}

	err = ep1.Leave(containerID)
	if err != nil {
		t.Fatal(err)
	}

	if err := ioutil.WriteFile("/etc/resolv.conf", tmpResolvConf2, 0644); err != nil {
		t.Fatal(err)
	}

	_, err = ep1.Join(containerID,
		libnetwork.JoinOptionResolvConfPath(resolvConfPath))
	if err != nil {
		t.Fatal(err)
	}

	content, err = ioutil.ReadFile(resolvConfPath)
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(content, expectedResolvConf2) {
		t.Fatalf("Expected %s, Got %s", string(expectedResolvConf2), string(content))
	}

	if err := ioutil.WriteFile(resolvConfPath, tmpResolvConf3, 0644); err != nil {
		t.Fatal(err)
	}

	err = ep1.Leave(containerID)
	if err != nil {
		t.Fatal(err)
	}

	_, err = ep1.Join(containerID,
		libnetwork.JoinOptionResolvConfPath(resolvConfPath))
	if err != nil {
		t.Fatal(err)
	}

	content, err = ioutil.ReadFile(resolvConfPath)
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(content, tmpResolvConf3) {
		t.Fatalf("Expected %s, Got %s", string(tmpResolvConf3), string(content))
	}
}

func TestInvalidRemoteDriver(t *testing.T) {
	if !netutils.IsRunningInContainer() {
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

	if err := os.MkdirAll("/usr/share/docker/plugins", 0755); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := os.RemoveAll("/usr/share/docker/plugins"); err != nil {
			t.Fatal(err)
		}
	}()

	if err := ioutil.WriteFile("/usr/share/docker/plugins/invalid-network-driver.spec", []byte(server.URL), 0644); err != nil {
		t.Fatal(err)
	}

	controller, err := libnetwork.New()
	if err != nil {
		t.Fatal(err)
	}

	_, err = controller.NewNetwork("invalid-network-driver", "dummy",
		libnetwork.NetworkOptionGeneric(getEmptyGenericOption()))
	if err == nil {
		t.Fatal("Expected to fail. But instead succeeded")
	}

	if err != plugins.ErrNotImplements {
		t.Fatalf("Did not fail with expected error. Actual error: %v", err)
	}
}

func TestValidRemoteDriver(t *testing.T) {
	if !netutils.IsRunningInContainer() {
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

	if err := os.MkdirAll("/usr/share/docker/plugins", 0755); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := os.RemoveAll("/usr/share/docker/plugins"); err != nil {
			t.Fatal(err)
		}
	}()

	if err := ioutil.WriteFile("/usr/share/docker/plugins/valid-network-driver.spec", []byte(server.URL), 0644); err != nil {
		t.Fatal(err)
	}

	controller, err := libnetwork.New()
	if err != nil {
		t.Fatal(err)
	}

	_, err = controller.NewNetwork("valid-network-driver", "dummy",
		libnetwork.NetworkOptionGeneric(getEmptyGenericOption()))
	if err != nil && err != driverapi.ErrNotImplemented {
		t.Fatal(err)
	}
}

var (
	once   sync.Once
	ctrlr  libnetwork.NetworkController
	start  = make(chan struct{})
	done   = make(chan chan struct{}, numThreads-1)
	origns = netns.None()
	testns = netns.None()
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

	if netutils.IsRunningInContainer() {
		testns = origns
	} else {
		testns, err = netns.New()
		if err != nil {
			t.Fatal(err)
		}
	}

	ctrlr, err = libnetwork.New()
	if err != nil {
		t.Fatal(err)
	}

	err = ctrlr.ConfigureNetworkDriver(bridgeNetType, getEmptyGenericOption())
	if err != nil {
		t.Fatal("configure driver")
	}

	net, err := ctrlr.NewNetwork(bridgeNetType, "network1")
	if err != nil {
		t.Fatal("new network")
	}

	_, err = net.CreateEndpoint("ep1")
	if err != nil {
		t.Fatal("createendpoint")
	}
}

func debugf(format string, a ...interface{}) (int, error) {
	if debug {
		return fmt.Printf(format, a...)
	}

	return 0, nil
}

func parallelJoin(t *testing.T, ep libnetwork.Endpoint, thrNumber int) {
	debugf("J%d.", thrNumber)
	_, err := ep.Join("racing_container")
	runtime.LockOSThread()
	if err != nil {
		if err != libnetwork.ErrNoContainer && err != libnetwork.ErrInvalidJoin {
			t.Fatal(err)
		}
		debugf("JE%d(%v).", thrNumber, err)
	}
	debugf("JD%d.", thrNumber)
}

func parallelLeave(t *testing.T, ep libnetwork.Endpoint, thrNumber int) {
	debugf("L%d.", thrNumber)
	err := ep.Leave("racing_container")
	runtime.LockOSThread()
	if err != nil {
		if err != libnetwork.ErrNoContainer && err != libnetwork.ErrInvalidJoin {
			t.Fatal(err)
		}
		debugf("LE%d(%v).", thrNumber, err)
	}
	debugf("LD%d.", thrNumber)
}

func runParallelTests(t *testing.T, thrNumber int) {
	var err error

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

	net, err := ctrlr.NetworkByName("network1")
	if err != nil {
		t.Fatal(err)
	}
	if net == nil {
		t.Fatal("Could not find network1")
	}

	ep, err := net.EndpointByName("ep1")
	if err != nil {
		t.Fatal(err)
	}
	if ep == nil {
		t.Fatal("Got nil ep with no error")
	}

	for i := 0; i < iterCnt; i++ {
		parallelJoin(t, ep, thrNumber)
		parallelLeave(t, ep, thrNumber)
	}

	debugf("\n")

	if thrNumber == first {
		for thrdone := range done {
			select {
			case <-thrdone:
			}
		}

		testns.Close()
		err = ep.Delete()
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
