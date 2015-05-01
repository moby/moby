package libnetwork_test

import (
	"net"
	"testing"

	log "github.com/Sirupsen/logrus"
	"github.com/docker/libnetwork"
	"github.com/docker/libnetwork/netutils"
	"github.com/docker/libnetwork/pkg/options"
)

const (
	netType    = "bridge"
	bridgeName = "dockertest0"
)

func createTestNetwork(networkType, networkName string, option options.Generic) (libnetwork.Network, error) {
	controller := libnetwork.New()
	genericOption := make(map[string]interface{})
	genericOption[options.GenericData] = option

	err := controller.ConfigureNetworkDriver(networkType, genericOption)
	if err != nil {
		return nil, err
	}

	network, err := controller.NewNetwork(networkType, networkName)
	if err != nil {
		return nil, err
	}

	return network, nil
}

func getEmptyGenericOption() map[string]interface{} {
	genericOption := make(map[string]interface{})
	genericOption[options.GenericData] = options.Generic{}
	return genericOption
}

func TestNull(t *testing.T) {
	network, err := createTestNetwork("null", "testnetwork", options.Generic{})
	if err != nil {
		t.Fatal(err)
	}

	ep, err := network.CreateEndpoint("testep")
	if err != nil {
		t.Fatal(err)
	}

	_, err = ep.Join(containerID,
		libnetwork.JoinOptionHostname("test"),
		libnetwork.JoinOptionDomainname("docker.io"))
	if err != nil {
		t.Fatal(err)
	}

	err = ep.Leave(containerID)
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

func TestBridge(t *testing.T) {
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

	network, err := createTestNetwork(netType, "testnetwork", option)
	if err != nil {
		t.Fatal(err)
	}

	ep, err := network.CreateEndpoint("testep")
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

	_, err := controller.NewNetwork("framerelay", "dummy",
		libnetwork.NetworkOptionGeneric(getEmptyGenericOption()))
	if err == nil {
		t.Fatal("Expected to fail. But instead succeeded")
	}

	if err != libnetwork.ErrInvalidNetworkDriver {
		t.Fatalf("Did not fail with expected error. Actual error: %v", err)
	}
}

func TestNoInitDriver(t *testing.T) {
	controller := libnetwork.New()

	_, err := controller.NewNetwork("ppp", "dummy",
		libnetwork.NetworkOptionGeneric(getEmptyGenericOption()))
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

	genericOption := make(map[string]interface{})
	genericOption[options.GenericData] = options.Generic{}

	err := controller.ConfigureNetworkDriver(netType, genericOption)
	if err != nil {
		t.Fatal(err.Error())
	}

	_, err = controller.NewNetwork(netType, "testnetwork", nil)
	if err != nil {
		t.Fatal("Expected to fail. But instead succeeded")
	}

	_, err = controller.NewNetwork(netType, "testnetwork")
	if err == nil {
		t.Fatal("Expected to fail. But instead succeeded")
	}

	if _, ok := err.(libnetwork.NetworkNameError); !ok {
		t.Fatalf("Did not fail with expected error. Actual error: %v", err)
	}
}

func TestNetworkName(t *testing.T) {
	networkName := "testnetwork"

	n, err := createTestNetwork(netType, networkName, options.Generic{})
	if err != nil {
		t.Fatal(err)
	}

	if n.Name() != networkName {
		t.Fatalf("Expected network name %s, got %s", networkName, n.Name())
	}
}

func TestNetworkType(t *testing.T) {
	networkType := netType

	n, err := createTestNetwork(networkType, "testnetwork", options.Generic{})
	if err != nil {
		t.Fatal(err)
	}

	if n.Type() != networkType {
		t.Fatalf("Expected network type %s, got %s", networkType, n.Type())
	}
}

func TestNetworkID(t *testing.T) {
	networkType := netType

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

	network, err := createTestNetwork(netType, "testnetwork", option)
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
	defer netutils.SetupTestNetNS(t)()
	option := options.Generic{
		"BridgeName":            bridgeName,
		"AllowNonDefaultBridge": true}

	network, err := createTestNetwork(netType, "testnetwork", option)
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

	network, err := createTestNetwork(netType, "testnetwork", option)
	if err != nil {
		t.Fatal(err)
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
	defer netutils.SetupTestNetNS(t)()
	controller := libnetwork.New()
	netType := "bridge"

	err := controller.ConfigureNetworkDriver(netType, getEmptyGenericOption())
	if err != nil {
		t.Fatal(err)
	}

	// Create network 1 and add 2 endpoint: ep11, ep12
	net1, err := controller.NewNetwork(netType, "network1")
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
	defer netutils.SetupTestNetNS(t)()
	controller := libnetwork.New()
	netType := "bridge"

	err := controller.ConfigureNetworkDriver(netType, getEmptyGenericOption())
	if err != nil {
		t.Fatal(err)
	}

	// Create network 1
	net1, err := controller.NewNetwork(netType, "network1")
	if err != nil {
		t.Fatal(err)
	}

	g := controller.NetworkByName("")
	if g != nil {
		t.Fatalf("NetworkByName() succeeded with invalid target name")
	}

	g = controller.NetworkByID("")
	if g != nil {
		t.Fatalf("NetworkByID() succeeded with invalid target id: %v", g)
	}

	g = controller.NetworkByID("network1")
	if g != nil {
		t.Fatalf("NetworkByID() succeeded with invalid target name")
	}

	g = controller.NetworkByName("network1")
	if g == nil {
		t.Fatalf("NetworkByName() did not find the network")
	}
	if g != net1 {
		t.Fatalf("NetworkByName() returned the wrong network")
	}

	g = controller.NetworkByID(net1.ID())
	if net1 != g {
		t.Fatalf("NetworkByID() returned unexpected element: %v", g)
	}
}

func TestNetworkQuery(t *testing.T) {
	defer netutils.SetupTestNetNS(t)()
	controller := libnetwork.New()
	netType := "bridge"

	err := controller.ConfigureNetworkDriver(netType, getEmptyGenericOption())
	if err != nil {
		t.Fatal(err)
	}

	// Create network 1 and add 2 endpoint: ep11, ep12
	net1, err := controller.NewNetwork(netType, "network1")
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

	e := net1.EndpointByName("ep11")
	if ep11 != e {
		t.Fatalf("EndpointByName() returned %v instead of %v", e, ep11)
	}

	e = net1.EndpointByName("")
	if e != nil {
		t.Fatalf("EndpointByName(): expected nil, got %v", e)
	}

	e = net1.EndpointByName("IamNotAnEndpoint")
	if e != nil {
		t.Fatalf("EndpointByName(): expected nil, got %v", e)
	}

	e = net1.EndpointByID(ep12.ID())
	if ep12 != e {
		t.Fatalf("EndpointByID() returned %v instead of %v", e, ep12)
	}

	e = net1.EndpointByID("")
	if e != nil {
		t.Fatalf("EndpointByID(): expected nil, got %v", e)
	}

}

const containerID = "valid_container"

func TestEndpointJoin(t *testing.T) {
	defer netutils.SetupTestNetNS(t)()

	n, err := createTestNetwork("bridge", "testnetwork", options.Generic{})
	if err != nil {
		t.Fatal(err)
	}

	ep, err := n.CreateEndpoint("ep1")
	if err != nil {
		t.Fatal(err)
	}

	_, err = ep.Join(containerID,
		libnetwork.JoinOptionHostname("test"),
		libnetwork.JoinOptionDomainname("docker.io"))
	if err != nil {
		t.Fatal(err)
	}

	err = ep.Leave(containerID)
	if err != nil {
		t.Fatal(err)
	}
}

func TestEndpointJoinInvalidContainerId(t *testing.T) {
	defer netutils.SetupTestNetNS(t)()

	n, err := createTestNetwork("bridge", "testnetwork", options.Generic{})
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

func TestEndpointMultipleJoins(t *testing.T) {
	defer netutils.SetupTestNetNS(t)()

	n, err := createTestNetwork("bridge", "testnetwork", options.Generic{})
	if err != nil {
		t.Fatal(err)
	}

	ep, err := n.CreateEndpoint("ep1")
	if err != nil {
		t.Fatal(err)
	}

	_, err = ep.Join(containerID,
		libnetwork.JoinOptionHostname("test"),
		libnetwork.JoinOptionDomainname("docker.io"))

	if err != nil {
		t.Fatal(err)
	}

	_, err = ep.Join("container2")
	if err == nil {
		t.Fatal("Expected to fail multiple joins for the same endpoint")
	}

	if err != libnetwork.ErrInvalidJoin {
		t.Fatalf("Failed for unexpected reason: %v", err)
	}

	err = ep.Leave(containerID)
	if err != nil {
		t.Fatal(err)
	}
}

func TestEndpointInvalidLeave(t *testing.T) {
	defer netutils.SetupTestNetNS(t)()

	n, err := createTestNetwork("bridge", "testnetwork", options.Generic{})
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
		t.Fatalf("Failed for unexpected reason: %v", err)
	}

	_, err = ep.Join(containerID,
		libnetwork.JoinOptionHostname("test"),
		libnetwork.JoinOptionDomainname("docker.io"))

	if err != nil {
		t.Fatal(err)
	}

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

	err = ep.Leave(containerID)
	if err != nil {
		t.Fatal(err)
	}
}
