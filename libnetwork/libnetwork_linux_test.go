package libnetwork_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"

	"github.com/containerd/log"
	"github.com/docker/docker/internal/testutils/netnsutils"
	"github.com/docker/docker/libnetwork"
	"github.com/docker/docker/libnetwork/config"
	"github.com/docker/docker/libnetwork/datastore"
	"github.com/docker/docker/libnetwork/driverapi"
	"github.com/docker/docker/libnetwork/ipams/defaultipam"
	"github.com/docker/docker/libnetwork/ipams/null"
	"github.com/docker/docker/libnetwork/ipamutils"
	"github.com/docker/docker/libnetwork/netlabel"
	"github.com/docker/docker/libnetwork/options"
	"github.com/docker/docker/libnetwork/osl"
	"github.com/docker/docker/libnetwork/types"
	"github.com/docker/docker/pkg/plugins"
	"github.com/docker/docker/pkg/reexec"
	"github.com/pkg/errors"
	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netns"
	"golang.org/x/sync/errgroup"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

const (
	bridgeNetType = "bridge"
)

func TestMain(m *testing.M) {
	// Cleanup local datastore file
	_ = os.Remove(datastore.DefaultScope("").Client.Address)

	os.Exit(m.Run())
}

func newController(t *testing.T) *libnetwork.Controller {
	t.Helper()
	c, err := libnetwork.New(
		libnetwork.OptionBoltdbWithRandomDBFile(t),
		config.OptionDriverConfig(bridgeNetType, map[string]interface{}{
			netlabel.GenericData: options.Generic{
				"EnableIPForwarding": true,
			},
		}),
		config.OptionDefaultAddressPoolConfig(ipamutils.GetLocalScopeDefaultNetworks()),
	)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(c.Stop)
	return c
}

func createTestNetwork(c *libnetwork.Controller, networkType, networkName string, netOption options.Generic, ipamV4Configs, ipamV6Configs []*libnetwork.IpamConf) (*libnetwork.Network, error) {
	return c.NewNetwork(networkType, networkName, "",
		libnetwork.NetworkOptionGeneric(netOption),
		libnetwork.NetworkOptionIpam(defaultipam.DriverName, "", ipamV4Configs, ipamV6Configs, nil))
}

func getEmptyGenericOption() map[string]interface{} {
	return map[string]interface{}{netlabel.GenericData: map[string]string{}}
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

func isNotFound(err error) bool {
	_, ok := (err).(types.NotFoundError)
	return ok
}

func TestNull(t *testing.T) {
	defer netnsutils.SetupTestOSContext(t)()
	controller := newController(t)

	cnt, err := controller.NewSandbox(context.Background(), "null_container",
		libnetwork.OptionHostname("test"),
		libnetwork.OptionDomainname("example.com"),
		libnetwork.OptionExtraHost("web", "192.168.0.1"))
	if err != nil {
		t.Fatal(err)
	}

	network, err := createTestNetwork(controller, "null", "testnull", options.Generic{}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	ep, err := network.CreateEndpoint(context.Background(), "testep")
	if err != nil {
		t.Fatal(err)
	}

	err = ep.Join(context.Background(), cnt)
	if err != nil {
		t.Fatal(err)
	}

	err = ep.Leave(context.Background(), cnt)
	if err != nil {
		t.Fatal(err)
	}

	if err := ep.Delete(context.Background(), false); err != nil {
		t.Fatal(err)
	}

	if err := cnt.Delete(context.Background()); err != nil {
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

func TestUnknownDriver(t *testing.T) {
	defer netnsutils.SetupTestOSContext(t)()
	controller := newController(t)

	_, err := createTestNetwork(controller, "unknowndriver", "testnetwork", options.Generic{}, nil, nil)
	if err == nil {
		t.Fatal("Expected to fail. But instead succeeded")
	}

	if !isNotFound(err) {
		t.Fatalf("Did not fail with expected error. Actual error: %v", err)
	}
}

func TestNilRemoteDriver(t *testing.T) {
	defer netnsutils.SetupTestOSContext(t)()
	controller := newController(t)

	_, err := controller.NewNetwork("framerelay", "dummy", "",
		libnetwork.NetworkOptionGeneric(getEmptyGenericOption()))
	if err == nil {
		t.Fatal("Expected to fail. But instead succeeded")
	}

	if !isNotFound(err) {
		t.Fatalf("Did not fail with expected error. Actual error: %v", err)
	}
}

func TestNetworkName(t *testing.T) {
	defer netnsutils.SetupTestOSContext(t)()
	controller := newController(t)

	netOption := options.Generic{
		netlabel.GenericData: options.Generic{
			"BridgeName": "testnetwork",
		},
	}

	_, err := createTestNetwork(controller, bridgeNetType, "", netOption, nil, nil)
	if err == nil {
		t.Fatal("Expected to fail. But instead succeeded")
	}

	if _, ok := err.(libnetwork.ErrInvalidName); !ok {
		t.Fatalf("Expected to fail with ErrInvalidName error. Got %v", err)
	}

	networkName := "testnetwork"
	n, err := createTestNetwork(controller, bridgeNetType, networkName, netOption, nil, nil)
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
	defer netnsutils.SetupTestOSContext(t)()
	controller := newController(t)

	netOption := options.Generic{
		netlabel.GenericData: options.Generic{
			"BridgeName": "testnetwork",
		},
	}

	n, err := createTestNetwork(controller, bridgeNetType, "testnetwork", netOption, nil, nil)
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
	defer netnsutils.SetupTestOSContext(t)()
	controller := newController(t)

	netOption := options.Generic{
		netlabel.GenericData: options.Generic{
			"BridgeName": "testnetwork",
		},
	}

	n, err := createTestNetwork(controller, bridgeNetType, "testnetwork", netOption, nil, nil)
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
	defer netnsutils.SetupTestOSContext(t)()
	controller := newController(t)

	netOption := options.Generic{
		"BridgeName": "testnetwork",
	}
	option := options.Generic{
		netlabel.GenericData: netOption,
	}

	network, err := createTestNetwork(controller, bridgeNetType, "testnetwork", option, nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	ep, err := network.CreateEndpoint(context.Background(), "testep")
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
	if err := ep.Delete(context.Background(), false); err != nil {
		t.Fatal(err)
	}

	if err := network.Delete(); err != nil {
		t.Fatal(err)
	}
}

func TestNetworkConfig(t *testing.T) {
	defer netnsutils.SetupTestOSContext(t)()
	controller := newController(t)

	// Verify config network cannot inherit another config network
	_, err := controller.NewNetwork("bridge", "config_network0", "",
		libnetwork.NetworkOptionConfigOnly(),
		libnetwork.NetworkOptionConfigFrom("anotherConfigNw"))

	if err == nil {
		t.Fatal("Expected to fail. But instead succeeded")
	}
	if _, ok := err.(types.ForbiddenError); !ok {
		t.Fatalf("Did not fail with expected error. Actual error: %v", err)
	}

	// Create supported config network
	netOption := options.Generic{
		"EnableICC": false,
	}
	option := options.Generic{
		netlabel.GenericData: netOption,
	}
	ipamV4ConfList := []*libnetwork.IpamConf{{PreferredPool: "192.168.100.0/24", SubPool: "192.168.100.128/25", Gateway: "192.168.100.1"}}
	ipamV6ConfList := []*libnetwork.IpamConf{{PreferredPool: "2001:db8:abcd::/64", SubPool: "2001:db8:abcd::ef99/80", Gateway: "2001:db8:abcd::22"}}

	netOptions := []libnetwork.NetworkOption{
		libnetwork.NetworkOptionConfigOnly(),
		libnetwork.NetworkOptionEnableIPv6(true),
		libnetwork.NetworkOptionGeneric(option),
		libnetwork.NetworkOptionIpam("default", "", ipamV4ConfList, ipamV6ConfList, nil),
	}

	configNetwork, err := controller.NewNetwork(bridgeNetType, "config_network0", "", netOptions...)
	if err != nil {
		t.Fatal(err)
	}

	// Verify a config-only network cannot be created with network operator configurations
	for i, opt := range []libnetwork.NetworkOption{
		libnetwork.NetworkOptionInternalNetwork(),
		libnetwork.NetworkOptionAttachable(true),
		libnetwork.NetworkOptionIngress(true),
	} {
		_, err = controller.NewNetwork(bridgeNetType, "testBR", "",
			libnetwork.NetworkOptionConfigOnly(), opt)
		if err == nil {
			t.Fatalf("Expected to fail. But instead succeeded for option: %d", i)
		}
		if _, ok := err.(types.ForbiddenError); !ok {
			t.Fatalf("Did not fail with expected error. Actual error: %v", err)
		}
	}

	// Verify a network cannot be created with both config-from and network specific configurations
	for i, opt := range []libnetwork.NetworkOption{
		libnetwork.NetworkOptionEnableIPv6(true),
		libnetwork.NetworkOptionIpam("my-ipam", "", nil, nil, nil),
		libnetwork.NetworkOptionIpam("", "", ipamV4ConfList, nil, nil),
		libnetwork.NetworkOptionIpam("", "", nil, ipamV6ConfList, nil),
		libnetwork.NetworkOptionLabels(map[string]string{"number": "two"}),
		libnetwork.NetworkOptionDriverOpts(map[string]string{"com.docker.network.driver.mtu": "1600"}),
	} {
		_, err = controller.NewNetwork(bridgeNetType, "testBR", "",
			libnetwork.NetworkOptionConfigFrom("config_network0"), opt)
		if err == nil {
			t.Fatalf("Expected to fail. But instead succeeded for option: %d", i)
		}
		if _, ok := err.(types.ForbiddenError); !ok {
			t.Fatalf("Did not fail with expected error. Actual error: %v", err)
		}
	}

	// Create a valid network
	network, err := controller.NewNetwork(bridgeNetType, "testBR", "",
		libnetwork.NetworkOptionConfigFrom("config_network0"))
	if err != nil {
		t.Fatal(err)
	}

	// Verify the config network cannot be removed
	err = configNetwork.Delete()
	if err == nil {
		t.Fatal("Expected to fail. But instead succeeded")
	}

	if _, ok := err.(types.ForbiddenError); !ok {
		t.Fatalf("Did not fail with expected error. Actual error: %v", err)
	}

	// Delete network
	if err := network.Delete(); err != nil {
		t.Fatal(err)
	}

	// Verify the config network can now be removed
	if err := configNetwork.Delete(); err != nil {
		t.Fatal(err)
	}
}

func TestUnknownNetwork(t *testing.T) {
	defer netnsutils.SetupTestOSContext(t)()
	controller := newController(t)

	netOption := options.Generic{
		"BridgeName": "testnetwork",
	}
	option := options.Generic{
		netlabel.GenericData: netOption,
	}

	network, err := createTestNetwork(controller, bridgeNetType, "testnetwork", option, nil, nil)
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
	defer netnsutils.SetupTestOSContext(t)()
	controller := newController(t)

	netOption := options.Generic{
		"BridgeName": "testnetwork",
	}
	option := options.Generic{
		netlabel.GenericData: netOption,
	}
	ipamV4ConfList := []*libnetwork.IpamConf{{PreferredPool: "192.168.100.0/24"}}

	network, err := createTestNetwork(controller, bridgeNetType, "testnetwork", option, ipamV4ConfList, nil)
	if err != nil {
		t.Fatal(err)
	}

	_, err = network.CreateEndpoint(context.Background(), "")
	if err == nil {
		t.Fatal("Expected to fail. But instead succeeded")
	}
	if _, ok := err.(libnetwork.ErrInvalidName); !ok {
		t.Fatalf("Expected to fail with ErrInvalidName error. Actual error: %v", err)
	}

	ep, err := network.CreateEndpoint(context.Background(), "testep")
	if err != nil {
		t.Fatal(err)
	}

	err = ep.Delete(context.Background(), false)
	if err != nil {
		t.Fatal(err)
	}

	// Done testing. Now cleanup
	if err := network.Delete(); err != nil {
		t.Fatal(err)
	}
}

func TestNetworkEndpointsWalkers(t *testing.T) {
	defer netnsutils.SetupTestOSContext(t)()
	controller := newController(t)

	// Create network 1 and add 2 endpoint: ep11, ep12
	netOption := options.Generic{
		netlabel.GenericData: options.Generic{
			"BridgeName": "network1",
		},
	}

	net1, err := createTestNetwork(controller, bridgeNetType, "network1", netOption, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := net1.Delete(); err != nil {
			t.Fatal(err)
		}
	}()

	ep11, err := net1.CreateEndpoint(context.Background(), "ep11")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := ep11.Delete(context.Background(), false); err != nil {
			t.Fatal(err)
		}
	}()

	ep12, err := net1.CreateEndpoint(context.Background(), "ep12")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := ep12.Delete(context.Background(), false); err != nil {
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
	var epWanted *libnetwork.Endpoint
	wlk := func(ep *libnetwork.Endpoint) bool {
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

	ctx := context.TODO()
	current := len(controller.Networks(ctx))

	// Create network 2
	netOption = options.Generic{
		netlabel.GenericData: options.Generic{
			"BridgeName": "network2",
		},
	}

	net2, err := createTestNetwork(controller, bridgeNetType, "network2", netOption, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := net2.Delete(); err != nil {
			t.Fatal(err)
		}
	}()

	// Test Networks method
	if len(controller.Networks(ctx)) != current+1 {
		t.Fatalf("Did not find the expected number of networks")
	}

	// Test Network Walk method
	var netName string
	var netWanted *libnetwork.Network
	nwWlk := func(nw *libnetwork.Network) bool {
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
	defer netnsutils.SetupTestOSContext(t)()
	controller := newController(t)

	netOption := options.Generic{
		netlabel.GenericData: options.Generic{
			"BridgeName": "testnetwork",
		},
	}
	n, err := createTestNetwork(controller, bridgeNetType, "testnetwork", netOption, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := n.Delete(); err != nil {
			t.Fatal(err)
		}
	}()

	ep, err := n.CreateEndpoint(context.Background(), "ep1")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := ep.Delete(context.Background(), false); err != nil {
			t.Fatal(err)
		}
	}()

	ep2, err := n.CreateEndpoint(context.Background(), "ep1")
	defer func() {
		// Cleanup ep2 as well, else network cleanup might fail for failure cases
		if ep2 != nil {
			if err := ep2.Delete(context.Background(), false); err != nil {
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
	defer netnsutils.SetupTestOSContext(t)()
	controller := newController(t)

	// Create network 1
	netOption := options.Generic{
		netlabel.GenericData: options.Generic{
			"BridgeName": "network1",
		},
	}
	net1, err := createTestNetwork(controller, bridgeNetType, "network1", netOption, nil, nil)
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
	net2, err := createTestNetwork(controller, bridgeNetType, "network2", netOption, nil, nil)
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
	defer netnsutils.SetupTestOSContext(t)()
	controller := newController(t)

	// Create network 1 and add 2 endpoint: ep11, ep12
	netOption := options.Generic{
		netlabel.GenericData: options.Generic{
			"BridgeName": "network1",
		},
	}
	net1, err := createTestNetwork(controller, bridgeNetType, "network1", netOption, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := net1.Delete(); err != nil {
			t.Fatal(err)
		}
	}()

	ep11, err := net1.CreateEndpoint(context.Background(), "ep11")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := ep11.Delete(context.Background(), false); err != nil {
			t.Fatal(err)
		}
	}()

	ep12, err := net1.CreateEndpoint(context.Background(), "ep12")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := ep12.Delete(context.Background(), false); err != nil {
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

	_, err = net1.EndpointByName("")
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

	_, err = net1.EndpointByID("")
	if err == nil {
		t.Fatalf("EndpointByID() succeeded with invalid target id")
	}
	if _, ok := err.(libnetwork.ErrInvalidID); !ok {
		t.Fatalf("EndpointByID() failed with unexpected error: %v", err)
	}
}

const containerID = "valid_c"

func TestEndpointDeleteWithActiveContainer(t *testing.T) {
	defer netnsutils.SetupTestOSContext(t)()
	controller := newController(t)

	n, err := createTestNetwork(controller, bridgeNetType, "testnetwork", options.Generic{
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

	n2, err := createTestNetwork(controller, bridgeNetType, "testnetwork2", options.Generic{
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

	ep, err := n.CreateEndpoint(context.Background(), "ep1")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		err = ep.Delete(context.Background(), false)
		if err != nil {
			t.Fatal(err)
		}
	}()

	cnt, err := controller.NewSandbox(context.Background(), containerID,
		libnetwork.OptionHostname("test"),
		libnetwork.OptionDomainname("example.com"),
		libnetwork.OptionExtraHost("web", "192.168.0.1"))
	defer func() {
		if err := cnt.Delete(context.Background()); err != nil {
			t.Fatal(err)
		}
	}()

	err = ep.Join(context.Background(), cnt)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		err = ep.Leave(context.Background(), cnt)
		if err != nil {
			t.Fatal(err)
		}
	}()

	err = ep.Delete(context.Background(), false)
	if err == nil {
		t.Fatal("Expected to fail. But instead succeeded")
	}

	if _, ok := err.(*libnetwork.ActiveContainerError); !ok {
		t.Fatalf("Did not fail with expected error. Actual error: %v", err)
	}
}

func TestEndpointMultipleJoins(t *testing.T) {
	defer netnsutils.SetupTestOSContext(t)()
	controller := newController(t)

	n, err := createTestNetwork(controller, bridgeNetType, "testmultiple", options.Generic{
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

	ep, err := n.CreateEndpoint(context.Background(), "ep1")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := ep.Delete(context.Background(), false); err != nil {
			t.Fatal(err)
		}
	}()

	sbx1, err := controller.NewSandbox(context.Background(), containerID,
		libnetwork.OptionHostname("test"),
		libnetwork.OptionDomainname("example.com"),
		libnetwork.OptionExtraHost("web", "192.168.0.1"),
	)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := sbx1.Delete(context.Background()); err != nil {
			t.Fatal(err)
		}
	}()

	sbx2, err := controller.NewSandbox(context.Background(), "c2")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := sbx2.Delete(context.Background()); err != nil {
			t.Fatal(err)
		}
	}()

	err = ep.Join(context.Background(), sbx1)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		err = ep.Leave(context.Background(), sbx1)
		if err != nil {
			t.Fatal(err)
		}
	}()

	err = ep.Join(context.Background(), sbx2)
	if err == nil {
		t.Fatal("Expected to fail multiple joins for the same endpoint")
	}

	if _, ok := err.(types.ForbiddenError); !ok {
		t.Fatalf("Failed with unexpected error type: %T. Desc: %s", err, err.Error())
	}
}

func TestLeaveAll(t *testing.T) {
	defer netnsutils.SetupTestOSContext(t)()
	controller := newController(t)

	n, err := createTestNetwork(controller, bridgeNetType, "testnetwork", options.Generic{
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

	n2, err := createTestNetwork(controller, bridgeNetType, "testnetwork2", options.Generic{
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

	ep1, err := n.CreateEndpoint(context.Background(), "ep1")
	if err != nil {
		t.Fatal(err)
	}

	ep2, err := n2.CreateEndpoint(context.Background(), "ep2")
	if err != nil {
		t.Fatal(err)
	}

	cnt, err := controller.NewSandbox(context.Background(), "leaveall")
	if err != nil {
		t.Fatal(err)
	}

	err = ep1.Join(context.Background(), cnt)
	if err != nil {
		t.Fatalf("Failed to join ep1: %v", err)
	}

	err = ep2.Join(context.Background(), cnt)
	if err != nil {
		t.Fatalf("Failed to join ep2: %v", err)
	}

	err = cnt.Delete(context.Background())
	if err != nil {
		t.Fatal(err)
	}
}

func TestContainerInvalidLeave(t *testing.T) {
	defer netnsutils.SetupTestOSContext(t)()
	controller := newController(t)

	n, err := createTestNetwork(controller, bridgeNetType, "testnetwork", options.Generic{
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

	ep, err := n.CreateEndpoint(context.Background(), "ep1")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := ep.Delete(context.Background(), false); err != nil {
			t.Fatal(err)
		}
	}()

	cnt, err := controller.NewSandbox(context.Background(), containerID,
		libnetwork.OptionHostname("test"),
		libnetwork.OptionDomainname("example.com"),
		libnetwork.OptionExtraHost("web", "192.168.0.1"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := cnt.Delete(context.Background()); err != nil {
			t.Fatal(err)
		}
	}()

	err = ep.Leave(context.Background(), cnt)
	if err == nil {
		t.Fatal("Expected to fail leave from an endpoint which has no active join")
	}
	if _, ok := err.(types.ForbiddenError); !ok {
		t.Fatalf("Failed with unexpected error type: %T. Desc: %s", err, err.Error())
	}

	if err = ep.Leave(context.Background(), nil); err == nil {
		t.Fatalf("Expected to fail leave nil Sandbox")
	}
	if _, ok := err.(types.InvalidParameterError); !ok {
		t.Fatalf("Unexpected error type returned: %T. Desc: %s", err, err.Error())
	}

	fsbx := &libnetwork.Sandbox{}
	if err = ep.Leave(context.Background(), fsbx); err == nil {
		t.Fatalf("Expected to fail leave with invalid Sandbox")
	}
	if _, ok := err.(types.InvalidParameterError); !ok {
		t.Fatalf("Unexpected error type returned: %T. Desc: %s", err, err.Error())
	}
}

func TestEndpointUpdateParent(t *testing.T) {
	defer netnsutils.SetupTestOSContext(t)()
	controller := newController(t)

	n, err := createTestNetwork(controller, bridgeNetType, "testnetwork", options.Generic{
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

	ep1, err := n.CreateEndpoint(context.Background(), "ep1")
	if err != nil {
		t.Fatal(err)
	}

	ep2, err := n.CreateEndpoint(context.Background(), "ep2")
	if err != nil {
		t.Fatal(err)
	}

	sbx1, err := controller.NewSandbox(context.Background(), containerID,
		libnetwork.OptionHostname("test"),
		libnetwork.OptionDomainname("example.com"),
		libnetwork.OptionExtraHost("web", "192.168.0.1"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := sbx1.Delete(context.Background()); err != nil {
			t.Fatal(err)
		}
	}()

	sbx2, err := controller.NewSandbox(context.Background(), "c2",
		libnetwork.OptionHostname("test2"),
		libnetwork.OptionDomainname("example.com"),
		libnetwork.OptionHostsPath("/var/lib/docker/test_network/container2/hosts"),
		libnetwork.OptionExtraHost("web", "192.168.0.2"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := sbx2.Delete(context.Background()); err != nil {
			t.Fatal(err)
		}
	}()

	err = ep1.Join(context.Background(), sbx1)
	if err != nil {
		t.Fatal(err)
	}

	err = ep2.Join(context.Background(), sbx2)
	if err != nil {
		t.Fatal(err)
	}
}

func TestInvalidRemoteDriver(t *testing.T) {
	mux := http.NewServeMux()
	server := httptest.NewServer(mux)
	if server == nil {
		t.Fatal("Failed to start an HTTP Server")
	}
	defer server.Close()

	mux.HandleFunc("/Plugin.Activate", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", plugins.VersionMimetype)
		fmt.Fprintln(w, `{"Implements": ["InvalidDriver"]}`)
	})

	if err := os.MkdirAll(specPath, 0o755); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := os.RemoveAll(specPath); err != nil {
			t.Fatal(err)
		}
	}()

	if err := os.WriteFile(filepath.Join(specPath, "invalid-network-driver.spec"), []byte(server.URL), 0o644); err != nil {
		t.Fatal(err)
	}

	ctrlr, err := libnetwork.New(libnetwork.OptionBoltdbWithRandomDBFile(t))
	if err != nil {
		t.Fatal(err)
	}
	defer ctrlr.Stop()

	_, err = ctrlr.NewNetwork("invalid-network-driver", "dummy", "",
		libnetwork.NetworkOptionGeneric(getEmptyGenericOption()))
	if err == nil {
		t.Fatal("Expected to fail. But instead succeeded")
	}

	if !errors.Is(err, plugins.ErrNotImplements) {
		t.Fatalf("Did not fail with expected error. Actual error: %v", err)
	}
}

func TestValidRemoteDriver(t *testing.T) {
	mux := http.NewServeMux()
	server := httptest.NewServer(mux)
	if server == nil {
		t.Fatal("Failed to start an HTTP Server")
	}
	defer server.Close()

	mux.HandleFunc("/Plugin.Activate", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", plugins.VersionMimetype)
		fmt.Fprintf(w, `{"Implements": ["%s"]}`, driverapi.NetworkPluginEndpointType)
	})
	mux.HandleFunc(fmt.Sprintf("/%s.GetCapabilities", driverapi.NetworkPluginEndpointType), func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", plugins.VersionMimetype)
		fmt.Fprintf(w, `{"Scope":"local"}`)
	})
	mux.HandleFunc(fmt.Sprintf("/%s.CreateNetwork", driverapi.NetworkPluginEndpointType), func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", plugins.VersionMimetype)
		fmt.Fprintf(w, "null")
	})
	mux.HandleFunc(fmt.Sprintf("/%s.DeleteNetwork", driverapi.NetworkPluginEndpointType), func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", plugins.VersionMimetype)
		fmt.Fprintf(w, "null")
	})

	if err := os.MkdirAll(specPath, 0o755); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := os.RemoveAll(specPath); err != nil {
			t.Fatal(err)
		}
	}()

	if err := os.WriteFile(filepath.Join(specPath, "valid-network-driver.spec"), []byte(server.URL), 0o644); err != nil {
		t.Fatal(err)
	}

	controller := newController(t)
	n, err := controller.NewNetwork("valid-network-driver", "dummy", "",
		libnetwork.NetworkOptionGeneric(getEmptyGenericOption()))
	if err != nil {
		// Only fail if we could not find the plugin driver
		if isNotFound(err) {
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

func makeTesthostNetwork(t *testing.T, c *libnetwork.Controller) *libnetwork.Network {
	t.Helper()
	n, err := createTestNetwork(c, "host", "testhost", options.Generic{}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	return n
}

func makeTestIPv6Network(t *testing.T, c *libnetwork.Controller) *libnetwork.Network {
	t.Helper()
	netOptions := options.Generic{
		netlabel.EnableIPv6: true,
		netlabel.GenericData: options.Generic{
			"BridgeName": "testnetwork",
		},
	}
	ipamV6ConfList := []*libnetwork.IpamConf{
		{PreferredPool: "fd81:fb6e:38ba:abcd::/64", Gateway: "fd81:fb6e:38ba:abcd::9"},
	}
	n, err := createTestNetwork(c,
		"bridge",
		"testnetwork",
		netOptions,
		nil,
		ipamV6ConfList,
	)
	assert.NilError(t, err)
	return n
}

func TestHost(t *testing.T) {
	defer netnsutils.SetupTestOSContext(t)()
	controller := newController(t)

	sbx1, err := controller.NewSandbox(context.Background(), "host_c1",
		libnetwork.OptionHostname("test1"),
		libnetwork.OptionDomainname("example.com"),
		libnetwork.OptionExtraHost("web", "192.168.0.1"),
		libnetwork.OptionUseDefaultSandbox())
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := sbx1.Delete(context.Background()); err != nil {
			t.Fatal(err)
		}
	}()

	sbx2, err := controller.NewSandbox(context.Background(), "host_c2",
		libnetwork.OptionHostname("test2"),
		libnetwork.OptionDomainname("example.com"),
		libnetwork.OptionExtraHost("web", "192.168.0.1"),
		libnetwork.OptionUseDefaultSandbox())
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := sbx2.Delete(context.Background()); err != nil {
			t.Fatal(err)
		}
	}()

	network := makeTesthostNetwork(t, controller)
	ep1, err := network.CreateEndpoint(context.Background(), "testep1")
	if err != nil {
		t.Fatal(err)
	}

	if err := ep1.Join(context.Background(), sbx1); err != nil {
		t.Fatal(err)
	}

	ep2, err := network.CreateEndpoint(context.Background(), "testep2")
	if err != nil {
		t.Fatal(err)
	}

	if err := ep2.Join(context.Background(), sbx2); err != nil {
		t.Fatal(err)
	}

	if err := ep1.Leave(context.Background(), sbx1); err != nil {
		t.Fatal(err)
	}

	if err := ep2.Leave(context.Background(), sbx2); err != nil {
		t.Fatal(err)
	}

	if err := ep1.Delete(context.Background(), false); err != nil {
		t.Fatal(err)
	}

	if err := ep2.Delete(context.Background(), false); err != nil {
		t.Fatal(err)
	}

	// Try to create another host endpoint and join/leave that.
	cnt3, err := controller.NewSandbox(context.Background(), "host_c3",
		libnetwork.OptionHostname("test3"),
		libnetwork.OptionDomainname("example.com"),
		libnetwork.OptionExtraHost("web", "192.168.0.1"),
		libnetwork.OptionUseDefaultSandbox())
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := cnt3.Delete(context.Background()); err != nil {
			t.Fatal(err)
		}
	}()

	ep3, err := network.CreateEndpoint(context.Background(), "testep3")
	if err != nil {
		t.Fatal(err)
	}

	if err := ep3.Join(context.Background(), sbx2); err != nil {
		t.Fatal(err)
	}

	if err := ep3.Leave(context.Background(), sbx2); err != nil {
		t.Fatal(err)
	}

	if err := ep3.Delete(context.Background(), false); err != nil {
		t.Fatal(err)
	}
}

// Testing IPV6 from MAC address
func TestBridgeIpv6FromMac(t *testing.T) {
	defer netnsutils.SetupTestOSContext(t)()
	controller := newController(t)

	netOption := options.Generic{
		netlabel.GenericData: options.Generic{
			"BridgeName":         "testipv6mac",
			"EnableICC":          true,
			"EnableIPMasquerade": true,
		},
	}
	ipamV4ConfList := []*libnetwork.IpamConf{{PreferredPool: "192.168.100.0/24", Gateway: "192.168.100.1"}}
	ipamV6ConfList := []*libnetwork.IpamConf{{PreferredPool: "fe90::/64", Gateway: "fe90::22"}}

	network, err := controller.NewNetwork(bridgeNetType, "testipv6mac", "",
		libnetwork.NetworkOptionGeneric(netOption),
		libnetwork.NetworkOptionEnableIPv6(true),
		libnetwork.NetworkOptionIpam(defaultipam.DriverName, "", ipamV4ConfList, ipamV6ConfList, nil),
		libnetwork.NetworkOptionDeferIPv6Alloc(true))
	if err != nil {
		t.Fatal(err)
	}

	mac := net.HardwareAddr{0xaa, 0xbb, 0xcc, 0xdd, 0xee, 0xff}
	epOption := options.Generic{netlabel.MacAddress: mac}

	ep, err := network.CreateEndpoint(context.Background(), "testep", libnetwork.EndpointOptionGeneric(epOption))
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

	if err := ep.Delete(context.Background(), false); err != nil {
		t.Fatal(err)
	}

	if err := network.Delete(); err != nil {
		t.Fatal(err)
	}
}

func checkSandbox(t *testing.T, info libnetwork.EndpointInfo) {
	key := info.Sandbox().Key()
	sbNs, err := netns.GetFromPath(key)
	if err != nil {
		t.Fatalf("Failed to get network namespace path %q: %v", key, err)
	}
	defer sbNs.Close()

	nh, err := netlink.NewHandleAt(sbNs)
	if err != nil {
		t.Fatal(err)
	}

	_, err = nh.LinkByName("eth0")
	if err != nil {
		t.Fatalf("Could not find the interface eth0 inside the sandbox: %v", err)
	}

	_, err = nh.LinkByName("eth1")
	if err != nil {
		t.Fatalf("Could not find the interface eth1 inside the sandbox: %v", err)
	}
}

func TestEndpointJoin(t *testing.T) {
	defer netnsutils.SetupTestOSContext(t)()
	controller := newController(t)

	// Create network 1 and add 2 endpoint: ep11, ep12
	netOption := options.Generic{
		netlabel.GenericData: options.Generic{
			"BridgeName":         "testnetwork1",
			"EnableICC":          true,
			"EnableIPMasquerade": true,
		},
	}
	ipamV6ConfList := []*libnetwork.IpamConf{{PreferredPool: "fe90::/64", Gateway: "fe90::22"}}
	n1, err := controller.NewNetwork(bridgeNetType, "testnetwork1", "",
		libnetwork.NetworkOptionGeneric(netOption),
		libnetwork.NetworkOptionEnableIPv6(true),
		libnetwork.NetworkOptionIpam(defaultipam.DriverName, "", nil, ipamV6ConfList, nil),
		libnetwork.NetworkOptionDeferIPv6Alloc(true))
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := n1.Delete(); err != nil {
			t.Fatal(err)
		}
	}()

	ep1, err := n1.CreateEndpoint(context.Background(), "ep1")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := ep1.Delete(context.Background(), false); err != nil {
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
	err = ep1.Join(context.Background(), nil)
	if err == nil {
		t.Fatalf("Expected to fail join with nil Sandbox")
	}
	if _, ok := err.(types.InvalidParameterError); !ok {
		t.Fatalf("Unexpected error type returned: %T", err)
	}

	fsbx := &libnetwork.Sandbox{}
	if err = ep1.Join(context.Background(), fsbx); err == nil {
		t.Fatalf("Expected to fail join with invalid Sandbox")
	}
	if _, ok := err.(types.InvalidParameterError); !ok {
		t.Fatalf("Unexpected error type returned: %T", err)
	}

	sb, err := controller.NewSandbox(context.Background(), containerID,
		libnetwork.OptionHostname("test"),
		libnetwork.OptionDomainname("example.com"),
		libnetwork.OptionExtraHost("web", "192.168.0.1"))
	if err != nil {
		t.Fatal(err)
	}

	defer func() {
		if err := sb.Delete(context.Background()); err != nil {
			t.Fatal(err)
		}
	}()

	err = ep1.Join(context.Background(), sb)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		err = ep1.Leave(context.Background(), sb)
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
		t.Fatalf("Expected an non-empty sandbox key for a joined endpoint. Instead found an empty sandbox key")
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
	n2, err := createTestNetwork(controller, bridgeNetType, "testnetwork2",
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

	ep2, err := n2.CreateEndpoint(context.Background(), "ep2")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := ep2.Delete(context.Background(), false); err != nil {
			t.Fatal(err)
		}
	}()

	err = ep2.Join(context.Background(), sb)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		err = ep2.Leave(context.Background(), sb)
		if err != nil {
			t.Fatal(err)
		}
	}()

	if ep1.Info().Sandbox().Key() != ep2.Info().Sandbox().Key() {
		t.Fatalf("ep1 and ep2 returned different container sandbox key")
	}

	checkSandbox(t, info)
}

func TestExternalKey(t *testing.T) {
	externalKeyTest(t, false)
}

func externalKeyTest(t *testing.T, reexec bool) {
	defer netnsutils.SetupTestOSContext(t)()
	controller := newController(t)

	n, err := createTestNetwork(controller, bridgeNetType, "testnetwork", options.Generic{
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

	n2, err := createTestNetwork(controller, bridgeNetType, "testnetwork2", options.Generic{
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

	ep, err := n.CreateEndpoint(context.Background(), "ep1")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		err = ep.Delete(context.Background(), false)
		if err != nil {
			t.Fatal(err)
		}
	}()

	ep2, err := n2.CreateEndpoint(context.Background(), "ep2")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		err = ep2.Delete(context.Background(), false)
		if err != nil {
			t.Fatal(err)
		}
	}()

	cnt, err := controller.NewSandbox(context.Background(), containerID,
		libnetwork.OptionHostname("test"),
		libnetwork.OptionDomainname("example.com"),
		libnetwork.OptionUseExternalKey(),
		libnetwork.OptionExtraHost("web", "192.168.0.1"))
	defer func() {
		if err := cnt.Delete(context.Background()); err != nil {
			t.Fatal(err)
		}
		osl.GC()
	}()

	// Join endpoint to sandbox before SetKey
	err = ep.Join(context.Background(), cnt)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		err = ep.Leave(context.Background(), cnt)
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
			t.Fatalf("libnetwork-setkey must fail if the corresponding namespace is not created")
		}
	} else {
		// Setting an non-existing key (namespace) must fail
		if err := sbox.SetKey(context.Background(), "this-must-fail"); err == nil {
			t.Fatalf("Setkey must fail if the corresponding namespace is not created")
		}
	}

	// Create a new OS sandbox using the osl API before using it in SetKey
	if extOsBox, err := osl.NewSandbox("ValidKey", true, false); err != nil {
		t.Fatalf("Failed to create new osl sandbox")
	} else {
		defer func() {
			if err := extOsBox.Destroy(); err != nil {
				log.G(context.TODO()).Warnf("Failed to remove os sandbox: %v", err)
			}
		}()
	}

	if reexec {
		err := reexecSetKey("ValidKey", containerID, controller.ID())
		if err != nil {
			t.Fatalf("libnetwork-setkey failed with %v", err)
		}
	} else {
		if err := sbox.SetKey(context.Background(), "ValidKey"); err != nil {
			t.Fatalf("Setkey failed with %v", err)
		}
	}

	// Join endpoint to sandbox after SetKey
	err = ep2.Join(context.Background(), sbox)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		err = ep2.Leave(context.Background(), sbox)
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
	type libcontainerState struct {
		NamespacePaths map[string]string
	}
	var (
		state libcontainerState
		b     []byte
		err   error
	)

	state.NamespacePaths = make(map[string]string)
	state.NamespacePaths["NEWNET"] = key
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

func TestResolvConf(t *testing.T) {
	tmpDir := t.TempDir()
	originResolvConfPath := filepath.Join(tmpDir, "origin_resolv.conf")
	resolvConfPath := filepath.Join(tmpDir, "resolv.conf")

	// Strip comments that end in a newline (a comment with no newline at the end
	// of the file will not be stripped).
	stripCommentsRE := regexp.MustCompile(`(?m)^#.*\n`)

	testcases := []struct {
		name             string
		makeNet          func(t *testing.T, c *libnetwork.Controller) *libnetwork.Network
		delNet           bool
		epOpts           []libnetwork.EndpointOption
		sbOpts           []libnetwork.SandboxOption
		originResolvConf string
		expResolvConf    string
	}{
		{
			name:             "IPv6 network",
			makeNet:          makeTestIPv6Network,
			delNet:           true,
			originResolvConf: "search pommesfrites.fr\nnameserver 12.34.56.78\nnameserver 2001:4860:4860::8888\n",
			expResolvConf:    "nameserver 127.0.0.11\nsearch pommesfrites.fr\noptions ndots:0",
		},
		{
			name:             "host network",
			makeNet:          makeTesthostNetwork,
			epOpts:           []libnetwork.EndpointOption{libnetwork.CreateOptionDisableResolution()},
			sbOpts:           []libnetwork.SandboxOption{libnetwork.OptionUseDefaultSandbox()},
			originResolvConf: "search localhost.net\nnameserver 127.0.0.1\nnameserver 2001:4860:4860::8888\n",
			expResolvConf:    "nameserver 127.0.0.1\nnameserver 2001:4860:4860::8888\nsearch localhost.net",
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			defer netnsutils.SetupTestOSContext(t)()
			c := newController(t)

			err := os.WriteFile(originResolvConfPath, []byte(tc.originResolvConf), 0o644)
			assert.NilError(t, err)

			n := tc.makeNet(t, c)
			if tc.delNet {
				defer func() {
					err := n.Delete()
					assert.Check(t, err)
				}()
			}

			sbOpts := append(tc.sbOpts,
				libnetwork.OptionResolvConfPath(resolvConfPath),
				libnetwork.OptionOriginResolvConfPath(originResolvConfPath),
			)
			sb, err := c.NewSandbox(context.Background(), containerID, sbOpts...)
			assert.NilError(t, err)
			defer func() {
				err := sb.Delete(context.Background())
				assert.Check(t, err)
			}()

			ep, err := n.CreateEndpoint(context.Background(), "ep", tc.epOpts...)
			assert.NilError(t, err)
			defer func() {
				err := ep.Delete(context.Background(), false)
				assert.Check(t, err)
			}()

			err = ep.Join(context.Background(), sb)
			assert.NilError(t, err)
			defer func() {
				err := ep.Leave(context.Background(), sb)
				assert.Check(t, err)
			}()

			finfo, err := os.Stat(resolvConfPath)
			assert.NilError(t, err)
			expFMode := (os.FileMode)(0o644)
			assert.Check(t, is.Equal(finfo.Mode().String(), expFMode.String()))
			content, err := os.ReadFile(resolvConfPath)
			assert.NilError(t, err)
			actual := stripCommentsRE.ReplaceAllString(string(content), "")
			actual = strings.TrimSpace(actual)
			assert.Check(t, is.Equal(actual, tc.expResolvConf))
		})
	}
}

type parallelTester struct {
	osctx      *netnsutils.OSContext
	controller *libnetwork.Controller
	net1, net2 *libnetwork.Network
	iterCnt    int
}

func (pt parallelTester) Do(t *testing.T, thrNumber int) error {
	teardown, err := pt.osctx.Set()
	if err != nil {
		return err
	}
	defer teardown(t)

	var ep *libnetwork.Endpoint
	if thrNumber == 1 {
		ep, err = pt.net1.EndpointByName(fmt.Sprintf("pep%d", thrNumber))
	} else {
		ep, err = pt.net2.EndpointByName(fmt.Sprintf("pep%d", thrNumber))
	}

	if err != nil {
		return errors.WithStack(err)
	}
	if ep == nil {
		return errors.New("got nil ep with no error")
	}

	cid := fmt.Sprintf("%drace", thrNumber)
	sb, err := pt.controller.GetSandbox(cid)
	if err != nil {
		return err
	}

	for i := 0; i < pt.iterCnt; i++ {
		if err := ep.Join(context.Background(), sb); err != nil {
			if _, ok := err.(types.ForbiddenError); !ok {
				return errors.Wrapf(err, "thread %d", thrNumber)
			}
		}
		if err := ep.Leave(context.Background(), sb); err != nil {
			if _, ok := err.(types.ForbiddenError); !ok {
				return errors.Wrapf(err, "thread %d", thrNumber)
			}
		}
	}

	if err := errors.WithStack(sb.Delete(context.Background())); err != nil {
		return err
	}
	return errors.WithStack(ep.Delete(context.Background(), false))
}

func TestParallel(t *testing.T) {
	const (
		first      = 1
		last       = 3
		numThreads = last - first + 1
		iterCnt    = 25
	)

	osctx := netnsutils.SetupTestOSContextEx(t)
	defer osctx.Cleanup(t)
	controller := newController(t)

	netOption := options.Generic{
		netlabel.GenericData: options.Generic{
			"BridgeName": "network",
		},
	}

	net1 := makeTesthostNetwork(t, controller)
	defer net1.Delete()
	net2, err := createTestNetwork(controller, "bridge", "network2", netOption, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer net2.Delete()

	_, err = net1.CreateEndpoint(context.Background(), "pep1")
	if err != nil {
		t.Fatal(err)
	}

	_, err = net2.CreateEndpoint(context.Background(), "pep2")
	if err != nil {
		t.Fatal(err)
	}

	_, err = net2.CreateEndpoint(context.Background(), "pep3")
	if err != nil {
		t.Fatal(err)
	}

	sboxes := make([]*libnetwork.Sandbox, numThreads)
	if sboxes[first-1], err = controller.NewSandbox(context.Background(), fmt.Sprintf("%drace", first), libnetwork.OptionUseDefaultSandbox()); err != nil {
		t.Fatal(err)
	}
	for thd := first + 1; thd <= last; thd++ {
		if sboxes[thd-1], err = controller.NewSandbox(context.Background(), fmt.Sprintf("%drace", thd)); err != nil {
			t.Fatal(err)
		}
	}

	pt := parallelTester{
		osctx:      osctx,
		controller: controller,
		net1:       net1,
		net2:       net2,
		iterCnt:    iterCnt,
	}

	var eg errgroup.Group
	for i := first; i <= last; i++ {
		i := i
		eg.Go(func() error { return pt.Do(t, i) })
	}
	if err := eg.Wait(); err != nil {
		t.Fatalf("%+v", err)
	}
}

func TestBridge(t *testing.T) {
	defer netnsutils.SetupTestOSContext(t)()
	controller := newController(t)

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

	network, err := createTestNetwork(controller, bridgeNetType, "testnetwork", netOption, ipamV4ConfList, ipamV6ConfList)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := network.Delete(); err != nil {
			t.Fatal(err)
		}
	}()

	ep, err := network.CreateEndpoint(context.Background(), "testep")
	if err != nil {
		t.Fatal(err)
	}

	sb, err := controller.NewSandbox(context.Background(), containerID, libnetwork.OptionPortMapping(getPortMapping()))
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := sb.Delete(context.Background()); err != nil {
			t.Fatal(err)
		}
	}()

	err = ep.Join(context.Background(), sb)
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
	expectedLen := 10
	if !isV6Listenable() {
		expectedLen = 5
	}
	if len(pm) != expectedLen {
		t.Fatalf("Incomplete data for port mapping in endpoint operational data: %d", len(pm))
	}
}

var (
	v6ListenableCached bool
	v6ListenableOnce   sync.Once
)

// This is copied from the bridge driver package b/c the bridge driver is not platform agnostic.
func isV6Listenable() bool {
	v6ListenableOnce.Do(func() {
		ln, err := net.Listen("tcp6", "[::1]:0")
		if err != nil {
			// When the kernel was booted with `ipv6.disable=1`,
			// we get err "listen tcp6 [::1]:0: socket: address family not supported by protocol"
			// https://github.com/moby/moby/issues/42288
			log.G(context.TODO()).Debugf("port_mapping: v6Listenable=false (%v)", err)
		} else {
			v6ListenableCached = true
			ln.Close()
		}
	})
	return v6ListenableCached
}

func TestNullIpam(t *testing.T) {
	defer netnsutils.SetupTestOSContext(t)()
	controller := newController(t)

	_, err := controller.NewNetwork(bridgeNetType, "testnetworkinternal", "", libnetwork.NetworkOptionIpam(null.DriverName, "", nil, nil, nil))
	if err == nil || err.Error() != "ipv4 pool is empty" {
		t.Fatal("bridge network should complain empty pool")
	}
}
