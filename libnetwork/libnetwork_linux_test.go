package libnetwork_test

import (
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
	"github.com/docker/docker/errdefs"
	"github.com/docker/docker/internal/nlwrap"
	"github.com/docker/docker/internal/testutils/netnsutils"
	"github.com/docker/docker/libnetwork"
	"github.com/docker/docker/libnetwork/config"
	"github.com/docker/docker/libnetwork/driverapi"
	"github.com/docker/docker/libnetwork/drivers/bridge"
	"github.com/docker/docker/libnetwork/ipams/defaultipam"
	"github.com/docker/docker/libnetwork/ipams/null"
	"github.com/docker/docker/libnetwork/ipamutils"
	"github.com/docker/docker/libnetwork/netlabel"
	"github.com/docker/docker/libnetwork/options"
	"github.com/docker/docker/libnetwork/osl"
	"github.com/docker/docker/libnetwork/types"
	"github.com/docker/docker/pkg/plugins"
	"github.com/moby/sys/reexec"
	"github.com/pkg/errors"
	"github.com/vishvananda/netns"
	"golang.org/x/sync/errgroup"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

const (
	bridgeNetType = "bridge"
)

func newController(t *testing.T) *libnetwork.Controller {
	t.Helper()
	c, err := libnetwork.New(
		context.Background(),
		config.OptionDataDir(t.TempDir()),
		config.OptionDriverConfig(bridgeNetType, map[string]interface{}{
			netlabel.GenericData: options.Generic{
				"EnableIPForwarding": true,
			},
		}),
		config.OptionDefaultAddressPoolConfig(ipamutils.GetLocalScopeDefaultNetworks()),
	)
	assert.NilError(t, err)
	t.Cleanup(c.Stop)
	return c
}

func createTestNetwork(c *libnetwork.Controller, networkType, networkName string, netOption options.Generic, ipamV4Configs, ipamV6Configs []*libnetwork.IpamConf) (*libnetwork.Network, error) {
	return c.NewNetwork(context.Background(), networkType, networkName, "",
		libnetwork.NetworkOptionGeneric(netOption),
		libnetwork.NetworkOptionIpam(defaultipam.DriverName, "", ipamV4Configs, ipamV6Configs, nil))
}

func getEmptyGenericOption() map[string]interface{} {
	return map[string]interface{}{netlabel.GenericData: map[string]string{}}
}

func getPortMapping() []types.PortBinding {
	return []types.PortBinding{
		{Proto: types.TCP, Port: 230, HostPort: 23000},
		{Proto: types.UDP, Port: 200, HostPort: 22000},
		{Proto: types.TCP, Port: 120, HostPort: 12000},
		{Proto: types.TCP, Port: 320, HostPort: 32000, HostPortEnd: 32999},
		{Proto: types.UDP, Port: 420, HostPort: 42000, HostPortEnd: 42001},
	}
}

func TestNull(t *testing.T) {
	defer netnsutils.SetupTestOSContext(t)()
	controller := newController(t)

	cnt, err := controller.NewSandbox(context.Background(), "null_container",
		libnetwork.OptionHostname("test"),
		libnetwork.OptionDomainname("example.com"),
		libnetwork.OptionExtraHost("web", "192.168.0.1"))
	assert.NilError(t, err)

	network, err := createTestNetwork(controller, "null", "testnull", options.Generic{}, nil, nil)
	assert.NilError(t, err)

	ep, err := network.CreateEndpoint(context.Background(), "testep")
	assert.NilError(t, err)

	err = ep.Join(context.Background(), cnt)
	assert.NilError(t, err)

	err = ep.Leave(context.Background(), cnt)
	assert.NilError(t, err)

	err = ep.Delete(context.Background(), false)
	assert.NilError(t, err)

	err = cnt.Delete(context.Background())
	assert.NilError(t, err)

	// host type is special network. Cannot be removed.
	err = network.Delete()

	// TODO(thaJeztah): should this be an [errdefs.ErrInvalidParameter] ?
	assert.Check(t, is.ErrorType(err, errdefs.IsForbidden))
	assert.Check(t, is.Error(err, `network of type "null" cannot be deleted`))
}

func TestUnknownDriver(t *testing.T) {
	defer netnsutils.SetupTestOSContext(t)()
	controller := newController(t)

	_, err := createTestNetwork(controller, "unknowndriver", "testnetwork", options.Generic{}, nil, nil)

	// TODO(thaJeztah): should attempting to use a non-existing plugin/driver return an [errdefs.ErrInvalidParameter] ?
	assert.Check(t, is.ErrorType(err, errdefs.IsNotFound))
	assert.Check(t, is.Error(err, "could not find plugin unknowndriver in v1 plugin registry: plugin not found"))
}

func TestNilRemoteDriver(t *testing.T) {
	defer netnsutils.SetupTestOSContext(t)()
	controller := newController(t)

	_, err := controller.NewNetwork(context.Background(), "framerelay", "dummy", "",
		libnetwork.NetworkOptionGeneric(getEmptyGenericOption()))

	// TODO(thaJeztah): should attempting to use a non-existing plugin/driver return an [errdefs.InvalidParameter] ?
	assert.Check(t, is.ErrorType(err, errdefs.IsNotFound))
	assert.Check(t, is.Error(err, "could not find plugin framerelay in v1 plugin registry: plugin not found"))
}

func TestNetworkName(t *testing.T) {
	defer netnsutils.SetupTestOSContext(t)()
	controller := newController(t)

	netOption := options.Generic{
		netlabel.EnableIPv4: true,
		netlabel.GenericData: map[string]string{
			bridge.BridgeName: "testnetwork",
		},
	}

	_, err := createTestNetwork(controller, bridgeNetType, "", netOption, nil, nil)
	assert.Check(t, is.ErrorType(err, errdefs.IsInvalidParameter), "Expected to fail with ErrInvalidName error")

	const networkName = "testnetwork"
	n, err := createTestNetwork(controller, bridgeNetType, networkName, netOption, nil, nil)
	assert.NilError(t, err)
	defer func() {
		assert.Check(t, n.Delete())
	}()

	assert.Check(t, is.Equal(n.Name(), networkName))
}

func TestNetworkType(t *testing.T) {
	defer netnsutils.SetupTestOSContext(t)()
	controller := newController(t)

	netOption := options.Generic{
		netlabel.EnableIPv4: true,
		netlabel.GenericData: map[string]string{
			bridge.BridgeName: "testnetwork",
		},
	}

	n, err := createTestNetwork(controller, bridgeNetType, "testnetwork", netOption, nil, nil)
	assert.NilError(t, err)
	defer func() {
		assert.Check(t, n.Delete())
	}()

	assert.Check(t, is.Equal(n.Type(), bridgeNetType))
}

func TestNetworkID(t *testing.T) {
	defer netnsutils.SetupTestOSContext(t)()
	controller := newController(t)

	netOption := options.Generic{
		netlabel.EnableIPv4: true,
		netlabel.GenericData: map[string]string{
			bridge.BridgeName: "testnetwork",
		},
	}

	n, err := createTestNetwork(controller, bridgeNetType, "testnetwork", netOption, nil, nil)
	assert.NilError(t, err)
	defer func() {
		assert.Check(t, n.Delete())
	}()

	assert.Check(t, n.ID() != "", "Expected non-empty network id")
}

func TestDeleteNetworkWithActiveEndpoints(t *testing.T) {
	defer netnsutils.SetupTestOSContext(t)()
	controller := newController(t)

	option := options.Generic{
		netlabel.EnableIPv4: true,
		netlabel.GenericData: map[string]string{
			bridge.BridgeName: "testnetwork",
		},
	}

	network, err := createTestNetwork(controller, bridgeNetType, "testnetwork", option, nil, nil)
	assert.NilError(t, err)

	ep, err := network.CreateEndpoint(context.Background(), "testep")
	assert.NilError(t, err)

	err = network.Delete()
	var activeEndpointsError *libnetwork.ActiveEndpointsError
	assert.Check(t, errors.As(err, &activeEndpointsError))
	assert.Check(t, is.ErrorContains(err, "has active endpoints"))
	// TODO(thaJeztah): should this be [errdefs.ErrConflict] or [errdefs.ErrInvalidParameter]?
	assert.Check(t, is.ErrorType(err, errdefs.IsForbidden))

	// Done testing. Now cleanup.
	err = ep.Delete(context.Background(), false)
	assert.NilError(t, err)

	err = network.Delete()
	assert.NilError(t, err)
}

func TestNetworkConfig(t *testing.T) {
	defer netnsutils.SetupTestOSContext(t)()
	controller := newController(t)

	// Verify config network cannot inherit another config network
	_, err := controller.NewNetwork(context.Background(), "bridge", "config_network0", "",
		libnetwork.NetworkOptionConfigOnly(),
		libnetwork.NetworkOptionConfigFrom("anotherConfigNw"),
	)

	// TODO(thaJeztah): should this be [errdefs.ErrInvalidParameter]?
	assert.Check(t, is.ErrorType(err, errdefs.IsForbidden))
	assert.Check(t, is.Error(err, "a configuration network cannot depend on another configuration network"))

	// Create supported config network
	option := options.Generic{
		netlabel.GenericData: map[string]string{
			bridge.EnableICC: "false",
		},
	}
	ipamV4ConfList := []*libnetwork.IpamConf{{PreferredPool: "192.168.100.0/24", SubPool: "192.168.100.128/25", Gateway: "192.168.100.1"}}
	ipamV6ConfList := []*libnetwork.IpamConf{{PreferredPool: "2001:db8:abcd::/64", SubPool: "2001:db8:abcd::ef99/80", Gateway: "2001:db8:abcd::22"}}

	netOptions := []libnetwork.NetworkOption{
		libnetwork.NetworkOptionConfigOnly(),
		libnetwork.NetworkOptionEnableIPv4(true),
		libnetwork.NetworkOptionEnableIPv6(true),
		libnetwork.NetworkOptionGeneric(option),
		libnetwork.NetworkOptionIpam("default", "", ipamV4ConfList, ipamV6ConfList, nil),
	}

	configNetwork, err := controller.NewNetwork(context.Background(), bridgeNetType, "config_network0", "", netOptions...)
	assert.NilError(t, err)

	// Verify a config-only network cannot be created with network operator configurations
	for i, opt := range []libnetwork.NetworkOption{
		libnetwork.NetworkOptionInternalNetwork(),
		libnetwork.NetworkOptionAttachable(true),
		libnetwork.NetworkOptionIngress(true),
	} {
		t.Run(fmt.Sprintf("config-only-%d", i), func(t *testing.T) {
			_, err = controller.NewNetwork(context.Background(), bridgeNetType, "testBR", "",
				libnetwork.NetworkOptionConfigOnly(), opt)

			// TODO(thaJeztah): should this be [errdefs.ErrInvalidParameter]?
			assert.Check(t, is.ErrorType(err, errdefs.IsForbidden))
			assert.Check(t, is.Error(err, "configuration network can only contain network specific fields. Network operator fields like [ ingress | internal | attachable | scope ] are not supported."))
		})
	}

	// Verify a network cannot be created with both config-from and network specific configurations
	for i, opt := range []libnetwork.NetworkOption{
		libnetwork.NetworkOptionEnableIPv4(false),
		libnetwork.NetworkOptionEnableIPv6(true),
		libnetwork.NetworkOptionIpam("my-ipam", "", nil, nil, nil),
		libnetwork.NetworkOptionIpam("", "", ipamV4ConfList, nil, nil),
		libnetwork.NetworkOptionIpam("", "", nil, ipamV6ConfList, nil),
		libnetwork.NetworkOptionLabels(map[string]string{"number": "two"}),
		libnetwork.NetworkOptionDriverOpts(map[string]string{"com.docker.network.driver.mtu": "1600"}),
	} {
		t.Run(fmt.Sprintf("config-from-%d", i), func(t *testing.T) {
			_, err = controller.NewNetwork(context.Background(), bridgeNetType, "testBR", "",
				libnetwork.NetworkOptionConfigFrom("config_network0"), opt)

			// TODO(thaJeztah): should this be [errdefs.ErrInvalidParameter]?
			assert.Check(t, is.ErrorType(err, errdefs.IsForbidden))

			//nolint:dupword // ignore "Duplicate words (network) found (dupword)"
			// Doing a partial match here omn the error-string here, as this produces either;
			//
			// 	user specified configurations are not supported if the network depends on a configuration network
			// 	network driver options are not supported if the network depends on a configuration network
			//
			// We can  consider changing this to a proper test-table.
			assert.Check(t, is.ErrorContains(err, `not supported if the network depends on a configuration network`))
		})
	}

	// Create a valid network
	network, err := controller.NewNetwork(context.Background(), bridgeNetType, "testBR", "",
		libnetwork.NetworkOptionConfigFrom("config_network0"))
	assert.NilError(t, err)

	// Verify the config network cannot be removed
	err = configNetwork.Delete()
	// TODO(thaJeztah): should this be [errdefs.ErrConflict] or [errdefs.ErrInvalidParameter]?
	assert.Check(t, is.ErrorType(err, errdefs.IsForbidden))
	assert.Check(t, is.Error(err, `configuration network "config_network0" is in use`))

	// Delete network
	err = network.Delete()
	assert.NilError(t, err)

	// Verify the config network can now be removed
	err = configNetwork.Delete()
	assert.NilError(t, err)
}

func TestUnknownNetwork(t *testing.T) {
	defer netnsutils.SetupTestOSContext(t)()
	controller := newController(t)

	option := options.Generic{
		netlabel.EnableIPv4: true,
		netlabel.GenericData: map[string]string{
			bridge.BridgeName: "testnetwork",
		},
	}

	network, err := createTestNetwork(controller, bridgeNetType, "testnetwork", option, nil, nil)
	assert.NilError(t, err)

	err = network.Delete()
	assert.NilError(t, err)

	err = network.Delete()
	assert.Check(t, is.ErrorType(err, errdefs.IsNotFound))
	assert.Check(t, is.ErrorContains(err, "unknown network testnetwork id"))
}

func TestUnknownEndpoint(t *testing.T) {
	defer netnsutils.SetupTestOSContext(t)()
	controller := newController(t)

	option := options.Generic{
		netlabel.EnableIPv4: true,
		netlabel.GenericData: map[string]string{
			bridge.BridgeName: "testnetwork",
		},
	}
	ipamV4ConfList := []*libnetwork.IpamConf{{PreferredPool: "192.168.100.0/24"}}

	network, err := createTestNetwork(controller, bridgeNetType, "testnetwork", option, ipamV4ConfList, nil)
	assert.NilError(t, err)

	_, err = network.CreateEndpoint(context.Background(), "")
	assert.Check(t, is.ErrorType(err, errdefs.IsInvalidParameter), "Expected to fail with ErrInvalidName error")
	assert.Check(t, is.ErrorContains(err, "invalid name:"))

	ep, err := network.CreateEndpoint(context.Background(), "testep")
	assert.NilError(t, err)

	err = ep.Delete(context.Background(), false)
	assert.NilError(t, err)

	// Done testing. Now cleanup
	err = network.Delete()
	assert.NilError(t, err)
}

func TestNetworkEndpointsWalkers(t *testing.T) {
	defer netnsutils.SetupTestOSContext(t)()
	controller := newController(t)

	// Create network 1 and add 2 endpoint: ep11, ep12
	netOption := options.Generic{
		netlabel.EnableIPv4: true,
		netlabel.GenericData: map[string]string{
			bridge.BridgeName: "network1",
		},
	}

	net1, err := createTestNetwork(controller, bridgeNetType, "network1", netOption, nil, nil)
	assert.NilError(t, err)
	defer func() {
		assert.Check(t, net1.Delete())
	}()

	ep11, err := net1.CreateEndpoint(context.Background(), "ep11")
	assert.NilError(t, err)
	defer func() {
		assert.Check(t, ep11.Delete(context.Background(), false))
	}()

	ep12, err := net1.CreateEndpoint(context.Background(), "ep12")
	assert.NilError(t, err)
	defer func() {
		assert.Check(t, ep12.Delete(context.Background(), false))
	}()

	// Test list methods on net1
	epList1 := net1.Endpoints()
	assert.Check(t, is.Len(epList1, 2), "Endpoints() returned wrong number of elements")
	// endpoint order is not guaranteed
	assert.Check(t, is.Contains(epList1, ep11), "Endpoints() did not return all the expected elements")
	assert.Check(t, is.Contains(epList1, ep12), "Endpoints() did not return all the expected elements")

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
	assert.Assert(t, epWanted != nil)
	assert.Assert(t, is.Equal(epWanted, ep11))

	ctx := context.TODO()
	current := len(controller.Networks(ctx))

	// Create network 2
	netOption = options.Generic{
		netlabel.EnableIPv4: true,
		netlabel.GenericData: map[string]string{
			bridge.BridgeName: "network2",
		},
	}

	net2, err := createTestNetwork(controller, bridgeNetType, "network2", netOption, nil, nil)
	assert.NilError(t, err)
	defer func() {
		assert.Check(t, net2.Delete())
	}()

	// Test Networks method
	assert.Assert(t, is.Len(controller.Networks(ctx), current+1))

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
	assert.Assert(t, netWanted != nil)
	assert.Check(t, is.Equal(net1.ID(), netWanted.ID()))

	netName = "network2"
	controller.WalkNetworks(nwWlk)
	assert.Assert(t, netWanted != nil)
	assert.Check(t, is.Equal(net2.ID(), netWanted.ID()))
}

func TestDuplicateEndpoint(t *testing.T) {
	defer netnsutils.SetupTestOSContext(t)()
	controller := newController(t)

	netOption := options.Generic{
		netlabel.EnableIPv4: true,
		netlabel.GenericData: map[string]string{
			bridge.BridgeName: "testnetwork",
		},
	}
	n, err := createTestNetwork(controller, bridgeNetType, "testnetwork", netOption, nil, nil)
	assert.NilError(t, err)
	defer func() {
		assert.Check(t, n.Delete())
	}()

	ep, err := n.CreateEndpoint(context.Background(), "ep1")
	assert.NilError(t, err)
	defer func() {
		assert.Check(t, ep.Delete(context.Background(), false))
	}()

	ep2, err := n.CreateEndpoint(context.Background(), "ep1")
	defer func() {
		// Cleanup ep2 as well, else network cleanup might fail for failure cases
		if ep2 != nil {
			assert.NilError(t, ep2.Delete(context.Background(), false))
		}
	}()

	// TODO(thaJeztah): should this be [errdefs.ErrConflict] or [errdefs.ErrInvalidParameter]?
	assert.Check(t, is.ErrorType(err, errdefs.IsForbidden))
	assert.Check(t, is.Error(err, "endpoint with name ep1 already exists in network testnetwork"))
}

func TestControllerQuery(t *testing.T) {
	defer netnsutils.SetupTestOSContext(t)()
	controller := newController(t)

	// Create network 1
	netOption := options.Generic{
		netlabel.EnableIPv4: true,
		netlabel.GenericData: map[string]string{
			bridge.BridgeName: "network1",
		},
	}
	net1, err := createTestNetwork(controller, bridgeNetType, "network1", netOption, nil, nil)
	assert.NilError(t, err)
	defer func() {
		assert.Check(t, net1.Delete())
	}()

	// Create network 2
	netOption = options.Generic{
		netlabel.EnableIPv4: true,
		netlabel.GenericData: map[string]string{
			bridge.BridgeName: "network2",
		},
	}
	net2, err := createTestNetwork(controller, bridgeNetType, "network2", netOption, nil, nil)
	assert.NilError(t, err)
	defer func() {
		assert.Check(t, net2.Delete())
	}()

	_, err = controller.NetworkByName("")
	assert.Check(t, is.ErrorType(err, errdefs.IsInvalidParameter))
	assert.Check(t, is.ErrorContains(err, "invalid name:"))

	_, err = controller.NetworkByID("")
	assert.Check(t, is.ErrorType(err, errdefs.IsInvalidParameter))
	assert.Check(t, is.Error(err, "invalid id: id is empty"))

	g, err := controller.NetworkByID("network1")
	assert.Check(t, is.ErrorType(err, errdefs.IsNotFound))
	assert.Check(t, is.Error(err, "network network1 not found"))
	assert.Check(t, is.Nil(g), "search network using name as ID should not yield a result")

	g, err = controller.NetworkByName("network1")
	assert.NilError(t, err)
	assert.Assert(t, g != nil, "NetworkByName() did not find the network")
	assert.Assert(t, is.Equal(g, net1), "NetworkByName() returned the wrong network")

	g, err = controller.NetworkByID(net1.ID())
	assert.NilError(t, err)
	assert.Assert(t, is.Equal(net1.ID(), g.ID()), "NetworkByID() returned unexpected element: %v", g)

	g, err = controller.NetworkByName("network2")
	assert.NilError(t, err)
	assert.Check(t, g != nil, "NetworkByName() did not find the network")
	assert.Check(t, is.Equal(g, net2), "NetworkByName() returned the wrong network")

	g, err = controller.NetworkByID(net2.ID())
	assert.NilError(t, err)
	assert.Check(t, is.Equal(g.ID(), net2.ID()), "NetworkByID() returned unexpected element: %v", g)
}

func TestNetworkQuery(t *testing.T) {
	defer netnsutils.SetupTestOSContext(t)()
	controller := newController(t)

	// Create network 1 and add 2 endpoint: ep11, ep12
	netOption := options.Generic{
		netlabel.EnableIPv4: true,
		netlabel.GenericData: map[string]string{
			bridge.BridgeName: "network1",
		},
	}
	net1, err := createTestNetwork(controller, bridgeNetType, "network1", netOption, nil, nil)
	assert.NilError(t, err)
	defer func() {
		assert.Check(t, net1.Delete())
	}()

	ep11, err := net1.CreateEndpoint(context.Background(), "ep11")
	assert.NilError(t, err)
	defer func() {
		assert.Check(t, ep11.Delete(context.Background(), false))
	}()

	ep12, err := net1.CreateEndpoint(context.Background(), "ep12")
	assert.NilError(t, err)
	defer func() {
		assert.Check(t, ep12.Delete(context.Background(), false))
	}()

	e, err := net1.EndpointByName("ep11")
	assert.NilError(t, err)
	assert.Check(t, is.Equal(e, ep11), "EndpointByName() returned the wrong endpoint")

	_, err = net1.EndpointByName("")
	assert.Check(t, is.ErrorType(err, errdefs.IsInvalidParameter))
	assert.Check(t, is.ErrorContains(err, "invalid name:"))

	e, err = net1.EndpointByName("IamNotAnEndpoint")
	assert.Check(t, is.ErrorType(err, errdefs.IsNotFound))
	assert.Check(t, is.Error(err, "endpoint IamNotAnEndpoint not found"))
	assert.Check(t, is.Nil(e), "EndpointByName() returned endpoint on error")
}

const containerID = "valid_c"

func TestEndpointDeleteWithActiveContainer(t *testing.T) {
	defer netnsutils.SetupTestOSContext(t)()
	controller := newController(t)

	n, err := createTestNetwork(controller, bridgeNetType, "testnetwork", options.Generic{
		netlabel.EnableIPv4: true,
		netlabel.GenericData: map[string]string{
			bridge.BridgeName: "testnetwork",
		},
	}, nil, nil)
	assert.NilError(t, err)
	defer func() {
		assert.Check(t, n.Delete())
	}()

	n2, err := createTestNetwork(controller, bridgeNetType, "testnetwork2", options.Generic{
		netlabel.EnableIPv4: true,
		netlabel.GenericData: map[string]string{
			bridge.BridgeName: "testnetwork2",
		},
	}, nil, nil)
	assert.NilError(t, err)
	defer func() {
		assert.Check(t, n2.Delete())
	}()

	ep, err := n.CreateEndpoint(context.Background(), "ep1")
	assert.NilError(t, err)
	defer func() {
		assert.Check(t, ep.Delete(context.Background(), false))
	}()

	cnt, err := controller.NewSandbox(context.Background(), containerID,
		libnetwork.OptionHostname("test"),
		libnetwork.OptionDomainname("example.com"),
		libnetwork.OptionExtraHost("web", "192.168.0.1"))
	assert.NilError(t, err)
	defer func() {
		assert.Check(t, cnt.Delete(context.Background()))
	}()

	err = ep.Join(context.Background(), cnt)
	assert.NilError(t, err)
	defer func() {
		assert.Check(t, ep.Leave(context.Background(), cnt))
	}()

	err = ep.Delete(context.Background(), false)

	var activeContainerError *libnetwork.ActiveContainerError
	assert.Check(t, errors.As(err, &activeContainerError))
	assert.Check(t, is.ErrorContains(err, "has active containers"))
	// TODO(thaJeztah): should this be [errdefs.ErrConflict] or [errdefs.ErrInvalidParameter]?
	assert.Check(t, is.ErrorType(err, errdefs.IsForbidden))
}

func TestEndpointMultipleJoins(t *testing.T) {
	defer netnsutils.SetupTestOSContext(t)()
	controller := newController(t)

	n, err := createTestNetwork(controller, bridgeNetType, "testmultiple", options.Generic{
		netlabel.EnableIPv4: true,
		netlabel.GenericData: map[string]string{
			bridge.BridgeName: "testmultiple",
		},
	}, nil, nil)
	assert.NilError(t, err)
	defer func() {
		assert.Check(t, n.Delete())
	}()

	ep, err := n.CreateEndpoint(context.Background(), "ep1")
	assert.NilError(t, err)
	defer func() {
		assert.Check(t, ep.Delete(context.Background(), false))
	}()

	sbx1, err := controller.NewSandbox(context.Background(), containerID,
		libnetwork.OptionHostname("test"),
		libnetwork.OptionDomainname("example.com"),
		libnetwork.OptionExtraHost("web", "192.168.0.1"),
	)
	assert.NilError(t, err)
	defer func() {
		assert.Check(t, sbx1.Delete(context.Background()))
	}()

	sbx2, err := controller.NewSandbox(context.Background(), "c2")
	assert.NilError(t, err)
	defer func() {
		assert.Check(t, sbx2.Delete(context.Background()))
	}()

	err = ep.Join(context.Background(), sbx1)
	assert.NilError(t, err)
	defer func() {
		assert.Check(t, ep.Leave(context.Background(), sbx1))
	}()

	err = ep.Join(context.Background(), sbx2)
	// TODO(thaJeztah): should this be [errdefs.ErrConflict] or [errdefs.ErrInvalidParameter]?
	assert.Check(t, is.ErrorType(err, errdefs.IsForbidden))
	assert.Check(t, is.Error(err, "another container is attached to the same network endpoint"))
}

func TestLeaveAll(t *testing.T) {
	defer netnsutils.SetupTestOSContext(t)()
	controller := newController(t)

	n, err := createTestNetwork(controller, bridgeNetType, "testnetwork", options.Generic{
		netlabel.EnableIPv4: true,
		netlabel.GenericData: map[string]string{
			bridge.BridgeName: "testnetwork",
		},
	}, nil, nil)
	assert.NilError(t, err)
	defer func() {
		// If this goes through, it means cnt.Delete() effectively detached from all the endpoints
		assert.Check(t, n.Delete())
	}()

	n2, err := createTestNetwork(controller, bridgeNetType, "testnetwork2", options.Generic{
		netlabel.EnableIPv4: true,
		netlabel.GenericData: map[string]string{
			bridge.BridgeName: "testnetwork2",
		},
	}, nil, nil)
	assert.NilError(t, err)
	defer func() {
		assert.Check(t, n2.Delete())
	}()

	ep1, err := n.CreateEndpoint(context.Background(), "ep1")
	assert.NilError(t, err)

	ep2, err := n2.CreateEndpoint(context.Background(), "ep2")
	assert.NilError(t, err)

	cnt, err := controller.NewSandbox(context.Background(), "leaveall")
	assert.NilError(t, err)

	err = ep1.Join(context.Background(), cnt)
	assert.NilError(t, err, "Failed to join ep1")

	err = ep2.Join(context.Background(), cnt)
	assert.NilError(t, err, "Failed to join ep2")

	err = cnt.Delete(context.Background())
	assert.NilError(t, err)
}

func TestContainerInvalidLeave(t *testing.T) {
	defer netnsutils.SetupTestOSContext(t)()
	controller := newController(t)

	n, err := createTestNetwork(controller, bridgeNetType, "testnetwork", options.Generic{
		netlabel.EnableIPv4: true,
		netlabel.GenericData: map[string]string{
			bridge.BridgeName: "testnetwork",
		},
	}, nil, nil)
	assert.NilError(t, err)
	defer func() {
		assert.Check(t, n.Delete())
	}()

	ep, err := n.CreateEndpoint(context.Background(), "ep1")
	assert.NilError(t, err)
	defer func() {
		assert.Check(t, ep.Delete(context.Background(), false))
	}()

	cnt, err := controller.NewSandbox(context.Background(), containerID,
		libnetwork.OptionHostname("test"),
		libnetwork.OptionDomainname("example.com"),
		libnetwork.OptionExtraHost("web", "192.168.0.1"))
	assert.NilError(t, err)
	defer func() {
		assert.Check(t, cnt.Delete(context.Background()))
	}()

	err = ep.Leave(context.Background(), cnt)
	assert.Assert(t, is.ErrorType(err, errdefs.IsForbidden), "Expected to fail leave from an endpoint which has no active join")
	assert.Check(t, is.Error(err, "cannot leave endpoint with no attached sandbox"))

	err = ep.Leave(context.Background(), nil)
	assert.Assert(t, is.ErrorType(err, errdefs.IsInvalidParameter), "Expected to fail leave with a nil Sandbox")
	// FIXME(thaJeztah): this error includes the raw data of the sandbox (as `<nil>`), which is not very informative
	assert.Check(t, is.Error(err, "invalid Sandbox passed to endpoint leave: <nil>"))

	fsbx := &libnetwork.Sandbox{}
	err = ep.Leave(context.Background(), fsbx)
	assert.Assert(t, is.ErrorType(err, errdefs.IsInvalidParameter), "Expected to fail leave with invalid Sandbox")
	//nolint:dupword // Ignore "Duplicate words (map[]) found (dupword)"
	// FIXME(thaJeztah): this error includes the raw data of the sandbox, which is not very human-readable or informative;
	//	invalid Sandbox passed to endpoint leave: &{  {{    []} {   [] [] []} map[] false false []} [] <nil> <nil> <nil> {{{} 0} {0 0}} [] map[] map[] <nil> 0 false false false false false []  {0 0} {0 0}}
	assert.Check(t, is.ErrorContains(err, "invalid Sandbox passed to endpoint leave"))
}

func TestEndpointUpdateParent(t *testing.T) {
	defer netnsutils.SetupTestOSContext(t)()
	controller := newController(t)

	n, err := createTestNetwork(controller, bridgeNetType, "testnetwork", options.Generic{
		netlabel.EnableIPv4: true,
		netlabel.GenericData: map[string]string{
			bridge.BridgeName: "testnetwork",
		},
	}, nil, nil)
	assert.NilError(t, err)
	defer func() {
		assert.Check(t, n.Delete())
	}()

	ep1, err := n.CreateEndpoint(context.Background(), "ep1")
	assert.NilError(t, err)

	ep2, err := n.CreateEndpoint(context.Background(), "ep2")
	assert.NilError(t, err)

	sbx1, err := controller.NewSandbox(context.Background(), containerID,
		libnetwork.OptionHostname("test"),
		libnetwork.OptionDomainname("example.com"),
		libnetwork.OptionExtraHost("web", "192.168.0.1"))
	assert.NilError(t, err)
	defer func() {
		assert.Check(t, sbx1.Delete(context.Background()))
	}()

	sbx2, err := controller.NewSandbox(context.Background(), "c2",
		libnetwork.OptionHostname("test2"),
		libnetwork.OptionDomainname("example.com"),
		libnetwork.OptionHostsPath("/var/lib/docker/test_network/container2/hosts"),
		libnetwork.OptionExtraHost("web", "192.168.0.2"))
	assert.NilError(t, err)
	defer func() {
		assert.Check(t, sbx2.Delete(context.Background()))
	}()

	err = ep1.Join(context.Background(), sbx1)
	assert.NilError(t, err)

	err = ep2.Join(context.Background(), sbx2)
	assert.NilError(t, err)
}

func TestInvalidRemoteDriver(t *testing.T) {
	mux := http.NewServeMux()
	server := httptest.NewServer(mux)
	defer server.Close()

	mux.HandleFunc("/Plugin.Activate", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", plugins.VersionMimetype)
		_, _ = fmt.Fprintln(w, `{"Implements": ["InvalidDriver"]}`)
	})

	err := os.MkdirAll(specPath, 0o755)
	assert.NilError(t, err)
	defer func() {
		assert.Check(t, os.RemoveAll(specPath))
	}()

	err = os.WriteFile(filepath.Join(specPath, "invalid-network-driver.spec"), []byte(server.URL), 0o644)
	assert.NilError(t, err)

	ctrlr, err := libnetwork.New(context.Background(), config.OptionDataDir(t.TempDir()))
	assert.NilError(t, err)
	defer ctrlr.Stop()

	_, err = ctrlr.NewNetwork(context.Background(), "invalid-network-driver", "dummy", "",
		libnetwork.NetworkOptionGeneric(getEmptyGenericOption()))
	assert.Check(t, is.ErrorIs(err, plugins.ErrNotImplements))
}

func TestValidRemoteDriver(t *testing.T) {
	mux := http.NewServeMux()
	server := httptest.NewServer(mux)
	defer server.Close()

	mux.HandleFunc("/Plugin.Activate", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", plugins.VersionMimetype)
		_, _ = fmt.Fprintf(w, `{"Implements": ["%s"]}`, driverapi.NetworkPluginEndpointType)
	})
	mux.HandleFunc(fmt.Sprintf("/%s.GetCapabilities", driverapi.NetworkPluginEndpointType), func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", plugins.VersionMimetype)
		_, _ = fmt.Fprintf(w, `{"Scope":"local"}`)
	})
	mux.HandleFunc(fmt.Sprintf("/%s.CreateNetwork", driverapi.NetworkPluginEndpointType), func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", plugins.VersionMimetype)
		_, _ = fmt.Fprintf(w, "null")
	})
	mux.HandleFunc(fmt.Sprintf("/%s.DeleteNetwork", driverapi.NetworkPluginEndpointType), func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", plugins.VersionMimetype)
		_, _ = fmt.Fprintf(w, "null")
	})

	err := os.MkdirAll(specPath, 0o755)
	assert.NilError(t, err)
	defer func() {
		assert.Check(t, os.RemoveAll(specPath))
	}()

	err = os.WriteFile(filepath.Join(specPath, "valid-network-driver.spec"), []byte(server.URL), 0o644)
	assert.NilError(t, err)

	controller := newController(t)
	n, err := controller.NewNetwork(context.Background(), "valid-network-driver", "dummy", "",
		libnetwork.NetworkOptionGeneric(getEmptyGenericOption()))
	if err != nil {
		// Only fail if we could not find the plugin driver
		if errdefs.IsNotFound(err) {
			t.Fatal(err)
		}
		return
	}
	defer func() {
		assert.Check(t, n.Delete())
	}()
}

func makeTesthostNetwork(t *testing.T, c *libnetwork.Controller) *libnetwork.Network {
	t.Helper()
	n, err := createTestNetwork(c, "host", "testhost", options.Generic{}, nil, nil)
	assert.NilError(t, err)
	return n
}

func makeTestIPv6Network(t *testing.T, c *libnetwork.Controller) *libnetwork.Network {
	t.Helper()
	netOptions := options.Generic{
		netlabel.EnableIPv4: true,
		netlabel.EnableIPv6: true,
		netlabel.GenericData: map[string]string{
			bridge.BridgeName: "testnetwork",
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
	assert.NilError(t, err)
	defer func() {
		assert.Check(t, sbx1.Delete(context.Background()))
	}()

	sbx2, err := controller.NewSandbox(context.Background(), "host_c2",
		libnetwork.OptionHostname("test2"),
		libnetwork.OptionDomainname("example.com"),
		libnetwork.OptionExtraHost("web", "192.168.0.1"),
		libnetwork.OptionUseDefaultSandbox())
	assert.NilError(t, err)
	defer func() {
		assert.Check(t, sbx2.Delete(context.Background()))
	}()

	network := makeTesthostNetwork(t, controller)
	ep1, err := network.CreateEndpoint(context.Background(), "testep1")
	assert.NilError(t, err)

	err = ep1.Join(context.Background(), sbx1)
	assert.NilError(t, err)

	ep2, err := network.CreateEndpoint(context.Background(), "testep2")
	assert.NilError(t, err)

	err = ep2.Join(context.Background(), sbx2)
	assert.NilError(t, err)

	err = ep1.Leave(context.Background(), sbx1)
	assert.NilError(t, err)

	err = ep2.Leave(context.Background(), sbx2)
	assert.NilError(t, err)

	err = ep1.Delete(context.Background(), false)
	assert.NilError(t, err)

	err = ep2.Delete(context.Background(), false)
	assert.NilError(t, err)

	// Try to create another host endpoint and join/leave that.
	cnt3, err := controller.NewSandbox(context.Background(), "host_c3",
		libnetwork.OptionHostname("test3"),
		libnetwork.OptionDomainname("example.com"),
		libnetwork.OptionExtraHost("web", "192.168.0.1"),
		libnetwork.OptionUseDefaultSandbox())
	assert.NilError(t, err)
	defer func() {
		assert.Check(t, cnt3.Delete(context.Background()))
	}()

	ep3, err := network.CreateEndpoint(context.Background(), "testep3")
	assert.NilError(t, err)

	err = ep3.Join(context.Background(), sbx2)
	assert.NilError(t, err)

	err = ep3.Leave(context.Background(), sbx2)
	assert.NilError(t, err)

	err = ep3.Delete(context.Background(), false)
	assert.NilError(t, err)
}

func checkSandbox(t *testing.T, info libnetwork.EndpointInfo) {
	key := info.Sandbox().Key()
	sbNs, err := netns.GetFromPath(key)
	assert.NilError(t, err, "Failed to get network namespace path %q", key)
	defer func() {
		assert.Check(t, sbNs.Close())
	}()

	nh, err := nlwrap.NewHandleAt(sbNs)
	assert.NilError(t, err)

	_, err = nh.LinkByName("eth0")
	assert.NilError(t, err, "Could not find the interface eth0 inside the sandbox")

	_, err = nh.LinkByName("eth1")
	assert.NilError(t, err, "Could not find the interface eth1 inside the sandbox")
}

func TestEndpointJoin(t *testing.T) {
	defer netnsutils.SetupTestOSContext(t)()
	controller := newController(t)

	// Create network 1 and add 2 endpoint: ep11, ep12
	netOption := options.Generic{
		netlabel.GenericData: map[string]string{
			bridge.BridgeName:         "testnetwork1",
			bridge.EnableICC:          "true",
			bridge.EnableIPMasquerade: "true",
		},
	}
	ipamV6ConfList := []*libnetwork.IpamConf{{PreferredPool: "fe90::/64", Gateway: "fe90::22"}}
	n1, err := controller.NewNetwork(context.Background(), bridgeNetType, "testnetwork1", "",
		libnetwork.NetworkOptionGeneric(netOption),
		libnetwork.NetworkOptionEnableIPv4(true),
		libnetwork.NetworkOptionEnableIPv6(true),
		libnetwork.NetworkOptionIpam(defaultipam.DriverName, "", nil, ipamV6ConfList, nil),
	)
	assert.NilError(t, err)
	defer func() {
		assert.Check(t, n1.Delete())
	}()

	ep1, err := n1.CreateEndpoint(context.Background(), "ep1")
	assert.NilError(t, err)
	defer func() {
		assert.Check(t, ep1.Delete(context.Background(), false))
	}()

	// Validate if ep.Info() only gives me IP address info and not names and gateway during CreateEndpoint()
	info := ep1.Info()
	iface := info.Iface()
	if iface.Address() != nil {
		assert.Check(t, iface.Address().IP.To4() != nil, "Invalid IP address returned: %v", iface.Address())
	}
	if iface.AddressIPv6() != nil {
		// Should be nil if it's an IPv6 address;https://github.com/moby/moby/pull/49329#discussion_r1925981233
		assert.Check(t, iface.AddressIPv6().IP.To4() == nil, "Invalid IPv6 address returned: %v", iface.AddressIPv6())
	}

	assert.Check(t, is.Len(info.Gateway(), 0), "Expected empty gateway for an empty endpoint. Instead found a gateway: %v", info.Gateway())
	assert.Check(t, is.Len(info.GatewayIPv6(), 0), "Expected empty gateway for an empty ipv6 endpoint. Instead found a gateway: %v", info.GatewayIPv6())
	assert.Check(t, is.Nil(info.Sandbox()), "Expected an empty sandbox key for an empty endpoint")

	// test invalid joins
	err = ep1.Join(context.Background(), nil)
	assert.Assert(t, is.ErrorType(err, errdefs.IsInvalidParameter), "Expected to fail join with nil Sandbox")
	// FIXME(thaJeztah): this error includes the raw data of the sandbox (as `<nil>`), which is not very informative
	assert.Check(t, is.Error(err, "invalid Sandbox passed to endpoint join: <nil>"))

	fsbx := &libnetwork.Sandbox{}
	err = ep1.Join(context.Background(), fsbx)
	assert.Assert(t, is.ErrorType(err, errdefs.IsInvalidParameter), "Expected to fail join with invalid Sandbox")

	//nolint:dupword // ignore "Duplicate words (map[]) found (dupword)"
	// FIXME(thaJeztah): this error includes the raw data of the sandbox, which is not very human-readable or informative;
	//	invalid Sandbox passed to endpoint join: &{  {{    []} {   [] [] []} map[] false false []} [] <nil> <nil> <nil> {{{} 0} {0 0}} [] map[] map[] <nil> 0 false false false false false []  {0 0} {0 0}}
	assert.Check(t, is.ErrorContains(err, "invalid Sandbox passed to endpoint join"))

	sb, err := controller.NewSandbox(context.Background(), containerID,
		libnetwork.OptionHostname("test"),
		libnetwork.OptionDomainname("example.com"),
		libnetwork.OptionExtraHost("web", "192.168.0.1"))
	assert.NilError(t, err)
	defer func() {
		assert.Check(t, sb.Delete(context.Background()))
	}()

	err = ep1.Join(context.Background(), sb)
	assert.NilError(t, err)
	defer func() {
		assert.Check(t, ep1.Leave(context.Background(), sb))
	}()

	// Validate if ep.Info() only gives valid gateway and sandbox key after has container has joined.
	info = ep1.Info()
	assert.Check(t, len(info.Gateway()) > 0, "Expected a valid gateway for a joined endpoint")
	assert.Check(t, len(info.GatewayIPv6()) > 0, "Expected a valid ipv6 gateway for a joined endpoint")
	assert.Check(t, info.Sandbox() != nil, "Expected an non-empty sandbox key for a joined endpoint")

	// Check endpoint provided container information
	assert.Check(t, is.Equal(sb.Key(), ep1.Info().Sandbox().Key()), "Endpoint Info returned unexpected sandbox key: %s", sb.Key())

	// Attempt retrieval of endpoint interfaces statistics
	stats, err := sb.Statistics()
	assert.NilError(t, err)
	_, ok := stats["eth0"]
	assert.Assert(t, ok, "Did not find eth0 statistics")

	// Now test the container joining another network
	n2, err := createTestNetwork(controller, bridgeNetType, "testnetwork2",
		options.Generic{
			netlabel.EnableIPv4: true,
			netlabel.GenericData: map[string]string{
				bridge.BridgeName: "testnetwork2",
			},
		}, nil, nil)
	assert.NilError(t, err)
	defer func() {
		assert.Check(t, n2.Delete())
	}()

	ep2, err := n2.CreateEndpoint(context.Background(), "ep2")
	assert.NilError(t, err)
	defer func() {
		assert.Check(t, ep2.Delete(context.Background(), false))
	}()

	err = ep2.Join(context.Background(), sb)
	assert.NilError(t, err)
	defer func() {
		assert.Check(t, ep2.Leave(context.Background(), sb))
	}()

	assert.Check(t, is.Equal(ep1.Info().Sandbox().Key(), ep2.Info().Sandbox().Key()), "ep1 and ep2 returned different container sandbox key")

	checkSandbox(t, info)
}

func TestExternalKey(t *testing.T) {
	externalKeyTest(t, false)
}

func externalKeyTest(t *testing.T, reexec bool) {
	defer netnsutils.SetupTestOSContext(t)()
	controller := newController(t)

	n, err := createTestNetwork(controller, bridgeNetType, "testnetwork", options.Generic{
		netlabel.EnableIPv4: true,
		netlabel.GenericData: map[string]string{
			bridge.BridgeName: "testnetwork",
		},
	}, nil, nil)
	assert.NilError(t, err)
	defer func() {
		assert.Check(t, n.Delete())
	}()

	n2, err := createTestNetwork(controller, bridgeNetType, "testnetwork2", options.Generic{
		netlabel.EnableIPv4: true,
		netlabel.GenericData: map[string]string{
			bridge.BridgeName: "testnetwork2",
		},
	}, nil, nil)
	assert.NilError(t, err)
	defer func() {
		assert.Check(t, n2.Delete())
	}()

	ep, err := n.CreateEndpoint(context.Background(), "ep1")
	assert.NilError(t, err)
	defer func() {
		assert.Check(t, ep.Delete(context.Background(), false))
	}()

	ep2, err := n2.CreateEndpoint(context.Background(), "ep2")
	assert.NilError(t, err)
	defer func() {
		assert.Check(t, ep2.Delete(context.Background(), false))
	}()

	cnt, err := controller.NewSandbox(context.Background(), containerID,
		libnetwork.OptionHostname("test"),
		libnetwork.OptionDomainname("example.com"),
		libnetwork.OptionUseExternalKey(),
		libnetwork.OptionExtraHost("web", "192.168.0.1"))
	assert.NilError(t, err)
	defer func() {
		assert.Check(t, cnt.Delete(context.Background()))
	}()

	// Join endpoint to sandbox before SetKey
	err = ep.Join(context.Background(), cnt)
	assert.NilError(t, err)
	defer func() {
		assert.Check(t, ep.Leave(context.Background(), cnt))
	}()

	sbox := ep.Info().Sandbox()
	assert.Assert(t, sbox != nil, "Expected to have a valid Sandbox")

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
	extOsBox, err := osl.NewSandbox("ValidKey", true, false)
	assert.NilError(t, err, "Failed to create new osl sandbox")
	defer func() {
		if err := extOsBox.Destroy(); err != nil {
			log.G(context.TODO()).Warnf("Failed to remove os sandbox: %v", err)
		}
	}()

	if reexec {
		err = reexecSetKey("ValidKey", containerID, controller.ID())
		assert.NilError(t, err, "libnetwork-setkey failed")
	} else {
		err = sbox.SetKey(context.Background(), "ValidKey")
		assert.NilError(t, err, "setkey failed")
	}

	// Join endpoint to sandbox after SetKey
	err = ep2.Join(context.Background(), sbox)
	assert.NilError(t, err)
	defer func() {
		assert.Check(t, ep2.Leave(context.Background(), sbox))
	}()

	assert.Assert(t, is.Equal(ep.Info().Sandbox().Key(), ep2.Info().Sandbox().Key()), "ep1 and ep2 returned different container sandbox key")

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
					assert.Check(t, n.Delete())
				}()
			}

			sbOpts := append(tc.sbOpts,
				libnetwork.OptionResolvConfPath(resolvConfPath),
				libnetwork.OptionOriginResolvConfPath(originResolvConfPath),
			)
			sb, err := c.NewSandbox(context.Background(), containerID, sbOpts...)
			assert.NilError(t, err)
			defer func() {
				assert.Check(t, sb.Delete(context.Background()))
			}()

			ep, err := n.CreateEndpoint(context.Background(), "ep", tc.epOpts...)
			assert.NilError(t, err)
			defer func() {
				assert.Check(t, ep.Delete(context.Background(), false))
			}()

			err = ep.Join(context.Background(), sb)
			assert.NilError(t, err)
			defer func() {
				assert.Check(t, ep.Leave(context.Background(), sb))
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
			if !errdefs.IsForbidden(err) {
				return errors.Wrapf(err, "thread %d", thrNumber)
			}
		}
		if err := ep.Leave(context.Background(), sb); err != nil {
			if !errdefs.IsForbidden(err) {
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
		netlabel.EnableIPv4: true,
		netlabel.GenericData: map[string]string{
			bridge.BridgeName: "network",
		},
	}

	net1 := makeTesthostNetwork(t, controller)
	defer net1.Delete()
	net2, err := createTestNetwork(controller, "bridge", "network2", netOption, nil, nil)
	assert.NilError(t, err)
	defer net2.Delete()

	_, err = net1.CreateEndpoint(context.Background(), "pep1")
	assert.NilError(t, err)

	_, err = net2.CreateEndpoint(context.Background(), "pep2")
	assert.NilError(t, err)

	_, err = net2.CreateEndpoint(context.Background(), "pep3")
	assert.NilError(t, err)

	sboxes := make([]*libnetwork.Sandbox, numThreads)
	sboxes[first-1], err = controller.NewSandbox(context.Background(), fmt.Sprintf("%drace", first), libnetwork.OptionUseDefaultSandbox())
	assert.NilError(t, err)

	for thd := first + 1; thd <= last; thd++ {
		sboxes[thd-1], err = controller.NewSandbox(context.Background(), fmt.Sprintf("%drace", thd))
		assert.NilError(t, err)
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
		eg.Go(func() error { return pt.Do(t, i) })
	}
	err = eg.Wait()
	assert.NilError(t, err)
}

func TestBridge(t *testing.T) {
	defer netnsutils.SetupTestOSContext(t)()
	controller := newController(t)

	netOption := options.Generic{
		netlabel.EnableIPv4: true,
		netlabel.EnableIPv6: true,
		netlabel.GenericData: map[string]string{
			bridge.BridgeName:         "testnetwork",
			bridge.EnableICC:          "true",
			bridge.EnableIPMasquerade: "true",
		},
	}
	ipamV4ConfList := []*libnetwork.IpamConf{{PreferredPool: "192.168.100.0/24", Gateway: "192.168.100.1"}}
	ipamV6ConfList := []*libnetwork.IpamConf{{PreferredPool: "fe90::/64", Gateway: "fe90::22"}}

	network, err := createTestNetwork(controller, bridgeNetType, "testnetwork", netOption, ipamV4ConfList, ipamV6ConfList)
	assert.NilError(t, err)
	defer func() {
		assert.Check(t, network.Delete())
	}()

	ep, err := network.CreateEndpoint(context.Background(), "testep")
	assert.NilError(t, err)

	sb, err := controller.NewSandbox(context.Background(), containerID, libnetwork.OptionPortMapping(getPortMapping()))
	assert.NilError(t, err)
	defer func() {
		assert.Check(t, sb.Delete(context.Background()))
	}()

	err = ep.Join(context.Background(), sb)
	assert.NilError(t, err)

	epInfo, err := ep.DriverInfo()
	assert.NilError(t, err)

	pmd, ok := epInfo[netlabel.PortMap]
	assert.Assert(t, ok, "Could not find expected info in endpoint data")

	pm, ok := pmd.([]types.PortBinding)
	assert.Assert(t, ok, "Unexpected format for port mapping in endpoint operational data")

	expectedLen := 10
	if !isV6Listenable() {
		expectedLen = 5
	}
	assert.Check(t, is.Len(pm, expectedLen), "Incomplete data for port mapping in endpoint operational data")
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
			_ = ln.Close()
		}
	})
	return v6ListenableCached
}

func TestBridgeRequiresIPAM(t *testing.T) {
	defer netnsutils.SetupTestOSContext(t)()
	controller := newController(t)

	_, err := controller.NewNetwork(context.Background(), bridgeNetType, "testnetwork", "",
		libnetwork.NetworkOptionIpam(null.DriverName, "", nil, nil, nil),
	)
	assert.Check(t, is.ErrorContains(err, "IPv4 or IPv6 must be enabled"))
}

func TestNullIpam(t *testing.T) {
	defer netnsutils.SetupTestOSContext(t)()
	controller := newController(t)

	tests := []struct {
		networkType string
	}{
		{networkType: bridgeNetType},
		{networkType: "macvlan"},
		{networkType: "ipvlan"},
	}

	for _, tc := range tests {
		t.Run(tc.networkType, func(t *testing.T) {
			_, err := controller.NewNetwork(context.Background(), tc.networkType, "tnet1-"+tc.networkType, "",
				libnetwork.NetworkOptionEnableIPv4(true),
				libnetwork.NetworkOptionIpam(null.DriverName, "", nil, nil, nil),
			)
			assert.Check(t, is.ErrorContains(err, "ipv4 pool is empty"))

			_, err = controller.NewNetwork(context.Background(), tc.networkType, "tnet2-"+tc.networkType, "",
				libnetwork.NetworkOptionEnableIPv6(true),
				libnetwork.NetworkOptionIpam(null.DriverName, "", nil, nil, nil),
			)
			assert.Check(t, is.ErrorContains(err, "ipv6 pool is empty"))
		})
	}
}
