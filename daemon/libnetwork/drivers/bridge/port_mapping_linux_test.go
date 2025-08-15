package bridge

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"os"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"testing"

	"github.com/containerd/log"
	"github.com/moby/moby/v2/daemon/internal/sliceutil"
	"github.com/moby/moby/v2/daemon/libnetwork/drivers/bridge/internal/firewaller"
	"github.com/moby/moby/v2/daemon/libnetwork/drvregistry"
	"github.com/moby/moby/v2/daemon/libnetwork/netlabel"
	"github.com/moby/moby/v2/daemon/libnetwork/ns"
	"github.com/moby/moby/v2/daemon/libnetwork/portallocator"
	"github.com/moby/moby/v2/daemon/libnetwork/portmapperapi"
	"github.com/moby/moby/v2/daemon/libnetwork/portmappers/nat"
	"github.com/moby/moby/v2/daemon/libnetwork/portmappers/routed"
	"github.com/moby/moby/v2/daemon/libnetwork/types"
	"github.com/moby/moby/v2/internal/testutils/netnsutils"
	"github.com/moby/moby/v2/internal/testutils/storeutils"
	"github.com/sirupsen/logrus"
	"github.com/vishvananda/netlink"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestPortMappingConfig(t *testing.T) {
	defer netnsutils.SetupTestOSContext(t)()
	useStubFirewaller(t)

	pms := drvregistry.PortMappers{}
	pm := &stubPortMapper{}
	err := pms.Register("nat", pm)
	assert.NilError(t, err)

	d := newDriver(storeutils.NewTempStore(t), &pms)

	config := &configuration{
		EnableIPTables: true,
		Hairpin:        true,
	}
	genericOption := make(map[string]any)
	genericOption[netlabel.GenericData] = config

	if err := d.configure(genericOption); err != nil {
		t.Fatalf("Failed to setup driver config: %v", err)
	}

	binding1 := types.PortBinding{Proto: types.SCTP, Port: 300, HostPort: 65000}
	binding2 := types.PortBinding{Proto: types.UDP, Port: 400, HostPort: 54000}
	binding3 := types.PortBinding{Proto: types.TCP, Port: 500, HostPort: 65000}
	portBindings := []types.PortBinding{binding1, binding2, binding3}

	sbOptions := make(map[string]any)
	sbOptions[netlabel.PortMap] = portBindings

	netOptions := map[string]any{
		netlabel.GenericData: &networkConfiguration{
			BridgeName: DefaultBridgeName,
			EnableIPv4: true,
		},
	}

	ipdList4 := getIPv4Data(t)
	err = d.CreateNetwork(context.Background(), "dummy", netOptions, nil, ipdList4, getIPv6Data(t))
	if err != nil {
		t.Fatalf("Failed to create bridge: %v", err)
	}

	te := newTestEndpoint(ipdList4[0].Pool, 11)
	err = d.CreateEndpoint(context.Background(), "dummy", "ep1", te.Interface(), nil)
	if err != nil {
		t.Fatalf("Failed to create the endpoint: %s", err.Error())
	}

	if err = d.Join(context.Background(), "dummy", "ep1", "sbox", te, nil, sbOptions); err != nil {
		t.Fatalf("Failed to join the endpoint: %v", err)
	}

	if err = d.ProgramExternalConnectivity(context.Background(), "dummy", "ep1", "ep1", ""); err != nil {
		t.Fatalf("Failed to program external connectivity: %v", err)
	}

	network, ok := d.networks["dummy"]
	if !ok {
		t.Fatalf("Cannot find network %s inside driver", "dummy")
	}
	ep := network.endpoints["ep1"]
	if len(ep.portMapping) != 3 {
		t.Fatalf("Failed to store the port bindings into the sandbox info. Found: %v", ep.portMapping)
	}
	if ep.portMapping[0].Proto != binding1.Proto || ep.portMapping[0].Port != binding1.Port ||
		ep.portMapping[1].Proto != binding2.Proto || ep.portMapping[1].Port != binding2.Port ||
		ep.portMapping[2].Proto != binding3.Proto || ep.portMapping[2].Port != binding3.Port {
		t.Fatal("bridgeEndpoint has incorrect port mapping values")
	}
	if ep.portMapping[0].HostIP == nil || ep.portMapping[0].HostPort == 0 ||
		ep.portMapping[1].HostIP == nil || ep.portMapping[1].HostPort == 0 ||
		ep.portMapping[2].HostIP == nil || ep.portMapping[2].HostPort == 0 {
		t.Fatal("operational port mapping data not found on bridgeEndpoint")
	}

	// release host mapped ports
	err = d.Leave("dummy", "ep1")
	if err != nil {
		t.Fatal(err)
	}

	err = d.ProgramExternalConnectivity(context.Background(), "dummy", "ep1", "", "")
	if err != nil {
		t.Fatal(err)
	}
}

func TestPortMappingV6Config(t *testing.T) {
	defer netnsutils.SetupTestOSContext(t)()
	if err := loopbackUp(); err != nil {
		t.Fatalf("Could not bring loopback iface up: %v", err)
	}

	pms := drvregistry.PortMappers{}
	pm := &stubPortMapper{}
	err := pms.Register("nat", pm)
	assert.NilError(t, err)

	d := newDriver(storeutils.NewTempStore(t), &pms)

	config := &configuration{
		EnableIPTables:  true,
		EnableIP6Tables: true,
	}
	genericOption := make(map[string]any)
	genericOption[netlabel.GenericData] = config

	if err := d.configure(genericOption); err != nil {
		t.Fatalf("Failed to setup driver config: %v", err)
	}

	portBindings := []types.PortBinding{
		{Proto: types.UDP, Port: 400, HostPort: 54000},
		{Proto: types.TCP, Port: 500, HostPort: 65000},
		{Proto: types.SCTP, Port: 500, HostPort: 65000},
	}

	sbOptions := make(map[string]any)
	sbOptions[netlabel.PortMap] = portBindings
	netConfig := &networkConfiguration{
		BridgeName: DefaultBridgeName,
		EnableIPv6: true,
	}
	netOptions := make(map[string]any)
	netOptions[netlabel.GenericData] = netConfig

	ipdList4 := getIPv4Data(t)
	ipdList6 := getIPv6Data(t)
	err = d.CreateNetwork(context.Background(), "dummy", netOptions, nil, ipdList4, ipdList6)
	if err != nil {
		t.Fatalf("Failed to create bridge: %v", err)
	}

	te := newTestEndpoint46(ipdList4[0].Pool, ipdList6[0].Pool, 11)
	err = d.CreateEndpoint(context.Background(), "dummy", "ep1", te.Interface(), nil)
	if err != nil {
		t.Fatalf("Failed to create the endpoint: %s", err.Error())
	}

	if err = d.Join(context.Background(), "dummy", "ep1", "sbox", te, nil, sbOptions); err != nil {
		t.Fatalf("Failed to join the endpoint: %v", err)
	}

	if err = d.ProgramExternalConnectivity(context.Background(), "dummy", "ep1", "ep1", "ep1"); err != nil {
		t.Fatalf("Failed to program external connectivity: %v", err)
	}

	network, ok := d.networks["dummy"]
	if !ok {
		t.Fatalf("Cannot find network %s inside driver", "dummy")
	}
	ep := network.endpoints["ep1"]
	if len(ep.portMapping) != 6 {
		t.Fatalf("Failed to store the port bindings into the sandbox info. Found: %v", ep.portMapping)
	}

	// release host mapped ports
	err = d.Leave("dummy", "ep1")
	if err != nil {
		t.Fatal(err)
	}

	err = d.ProgramExternalConnectivity(context.Background(), "dummy", "ep1", "", "")
	if err != nil {
		t.Fatal(err)
	}
}

func loopbackUp() error {
	nlHandle := ns.NlHandle()
	iface, err := nlHandle.LinkByName("lo")
	if err != nil {
		return err
	}
	return nlHandle.LinkSetUp(iface)
}

func newIPNet(t *testing.T, cidr string) *net.IPNet {
	t.Helper()
	ip, ipNet, err := net.ParseCIDR(cidr)
	assert.NilError(t, err)
	ipNet.IP = ip
	return ipNet
}

func TestAddPortMappings(t *testing.T) {
	ctrIP4 := newIPNet(t, "172.19.0.2/16")
	ctrIP4Mapped := newIPNet(t, "::ffff:172.19.0.2/112")
	ctrIP6 := newIPNet(t, "fdf8:b88e:bb5c:3483::2/64")
	firstEphemPort, _ := portallocator.GetPortRange()

	testcases := []struct {
		name         string
		epAddrV4     *net.IPNet
		epAddrV6     *net.IPNet
		gwMode4      gwMode
		gwMode6      gwMode
		cfg          []portmapperapi.PortBindingReq
		defHostIP    net.IP
		enableProxy  bool
		hairpin      bool
		busyPortIPv4 int
		rootless     bool
		hostAddrs    []string
		noProxy6To4  bool

		expErr          string
		expLogs         []string
		expPBs          []types.PortBinding
		expProxyRunning bool
		expReleaseErr   string
		expNAT4Rules    []string
		expFilter4Rules []string
		expNAT6Rules    []string
		expFilter6Rules []string
	}{
		{
			name:     "defaults",
			epAddrV4: ctrIP4,
			epAddrV6: ctrIP6,
			cfg: []portmapperapi.PortBindingReq{
				{PortBinding: types.PortBinding{Proto: types.TCP, Port: 22}},
				{PortBinding: types.PortBinding{Proto: types.TCP, Port: 80}},
			},
			enableProxy: true,
			expPBs: []types.PortBinding{
				{Proto: types.TCP, IP: ctrIP4.IP, Port: 22, HostIP: net.IPv4zero, HostPort: firstEphemPort},
				{Proto: types.TCP, IP: ctrIP6.IP, Port: 22, HostIP: net.IPv6zero, HostPort: firstEphemPort},
				{Proto: types.TCP, IP: ctrIP4.IP, Port: 80, HostIP: net.IPv4zero, HostPort: firstEphemPort + 1},
				{Proto: types.TCP, IP: ctrIP6.IP, Port: 80, HostIP: net.IPv6zero, HostPort: firstEphemPort + 1},
			},
		},
		{
			name:        "specific host port",
			epAddrV4:    ctrIP4,
			epAddrV6:    ctrIP6,
			cfg:         []portmapperapi.PortBindingReq{{PortBinding: types.PortBinding{Proto: types.TCP, Port: 80, HostPort: 8080}}},
			enableProxy: true,
			expPBs: []types.PortBinding{
				{Proto: types.TCP, IP: ctrIP4.IP, Port: 80, HostIP: net.IPv4zero, HostPort: 8080, HostPortEnd: 8080},
				{Proto: types.TCP, IP: ctrIP6.IP, Port: 80, HostIP: net.IPv6zero, HostPort: 8080, HostPortEnd: 8080},
			},
		},
		{
			name:        "nat explicitly enabled",
			epAddrV4:    ctrIP4,
			epAddrV6:    ctrIP6,
			cfg:         []portmapperapi.PortBindingReq{{PortBinding: types.PortBinding{Proto: types.TCP, Port: 80, HostPort: 8080}}},
			gwMode4:     gwModeNAT,
			gwMode6:     gwModeNAT,
			enableProxy: true,
			expPBs: []types.PortBinding{
				{Proto: types.TCP, IP: ctrIP4.IP, Port: 80, HostIP: net.IPv4zero, HostPort: 8080, HostPortEnd: 8080},
				{Proto: types.TCP, IP: ctrIP6.IP, Port: 80, HostIP: net.IPv6zero, HostPort: 8080, HostPortEnd: 8080},
			},
		},
		{
			name:         "specific host port in-use",
			epAddrV4:     ctrIP4,
			epAddrV6:     ctrIP6,
			cfg:          []portmapperapi.PortBindingReq{{PortBinding: types.PortBinding{Proto: types.TCP, Port: 80, HostPort: 8080}}},
			enableProxy:  true,
			busyPortIPv4: 8080,
			expErr:       "failed to bind host port 0.0.0.0:8080/tcp: address already in use",
		},
		{
			name:        "ipv4 mapped container address with specific host port",
			epAddrV4:    ctrIP4Mapped,
			epAddrV6:    ctrIP6,
			cfg:         []portmapperapi.PortBindingReq{{PortBinding: types.PortBinding{Proto: types.TCP, Port: 80, HostPort: 8080}}},
			enableProxy: true,
			expPBs: []types.PortBinding{
				{Proto: types.TCP, IP: ctrIP4.IP, Port: 80, HostIP: net.IPv4zero, HostPort: 8080, HostPortEnd: 8080},
				{Proto: types.TCP, IP: ctrIP6.IP, Port: 80, HostIP: net.IPv6zero, HostPort: 8080, HostPortEnd: 8080},
			},
		},
		{
			name:        "ipv4 mapped host address with specific host port",
			epAddrV4:    ctrIP4,
			epAddrV6:    ctrIP6,
			cfg:         []portmapperapi.PortBindingReq{{PortBinding: types.PortBinding{Proto: types.TCP, Port: 80, HostIP: newIPNet(t, "::ffff:127.0.0.1/128").IP, HostPort: 8080}}},
			enableProxy: true,
			expPBs: []types.PortBinding{
				{Proto: types.TCP, IP: ctrIP4.IP, Port: 80, HostIP: newIPNet(t, "127.0.0.1/32").IP, HostPort: 8080, HostPortEnd: 8080},
			},
		},
		{
			name:         "host port range with first port in-use",
			epAddrV4:     ctrIP4,
			epAddrV6:     ctrIP6,
			cfg:          []portmapperapi.PortBindingReq{{PortBinding: types.PortBinding{Proto: types.TCP, Port: 80, HostPort: 8080, HostPortEnd: 8081}}},
			enableProxy:  true,
			busyPortIPv4: 8080,
			expPBs: []types.PortBinding{
				{Proto: types.TCP, IP: ctrIP4.IP, Port: 80, HostIP: net.IPv4zero, HostPort: 8081, HostPortEnd: 8081},
				{Proto: types.TCP, IP: ctrIP6.IP, Port: 80, HostIP: net.IPv6zero, HostPort: 8081, HostPortEnd: 8081},
			},
		},
		{
			name:     "multi host ips with host port range and first port in-use",
			epAddrV4: ctrIP4,
			epAddrV6: ctrIP6,
			cfg: []portmapperapi.PortBindingReq{
				{PortBinding: types.PortBinding{Proto: types.TCP, Port: 80, HostIP: net.IPv4zero, HostPort: 8080, HostPortEnd: 8081}},
				{PortBinding: types.PortBinding{Proto: types.TCP, Port: 80, HostIP: net.IPv6zero, HostPort: 8080, HostPortEnd: 8081}},
			},
			enableProxy:  true,
			busyPortIPv4: 8080,
			expPBs: []types.PortBinding{
				{Proto: types.TCP, IP: ctrIP4.IP, Port: 80, HostIP: net.IPv4zero, HostPort: 8081},
				{Proto: types.TCP, IP: ctrIP6.IP, Port: 80, HostIP: net.IPv6zero, HostPort: 8081},
			},
		},
		{
			name:     "host port range with busy port",
			epAddrV4: ctrIP4,
			epAddrV6: ctrIP6,
			cfg: []portmapperapi.PortBindingReq{
				{PortBinding: types.PortBinding{Proto: types.TCP, Port: 80, HostPort: 8080, HostPortEnd: 8083}},
				{PortBinding: types.PortBinding{Proto: types.TCP, Port: 81, HostPort: 8080, HostPortEnd: 8083}},
				{PortBinding: types.PortBinding{Proto: types.TCP, Port: 82, HostPort: 8080, HostPortEnd: 8083}},
				{PortBinding: types.PortBinding{Proto: types.UDP, Port: 80, HostPort: 8080, HostPortEnd: 8083}},
				{PortBinding: types.PortBinding{Proto: types.UDP, Port: 81, HostPort: 8080, HostPortEnd: 8083}},
				{PortBinding: types.PortBinding{Proto: types.UDP, Port: 82, HostPort: 8080, HostPortEnd: 8083}},
			},
			enableProxy:  true,
			busyPortIPv4: 8082,
			expPBs: []types.PortBinding{
				{Proto: types.TCP, IP: ctrIP4.IP, Port: 80, HostIP: net.IPv4zero, HostPort: 8080, HostPortEnd: 8080},
				{Proto: types.TCP, IP: ctrIP6.IP, Port: 80, HostIP: net.IPv6zero, HostPort: 8080, HostPortEnd: 8080},
				{Proto: types.UDP, IP: ctrIP4.IP, Port: 80, HostIP: net.IPv4zero, HostPort: 8080, HostPortEnd: 8080},
				{Proto: types.UDP, IP: ctrIP6.IP, Port: 80, HostIP: net.IPv6zero, HostPort: 8080, HostPortEnd: 8080},
				{Proto: types.TCP, IP: ctrIP4.IP, Port: 81, HostIP: net.IPv4zero, HostPort: 8081, HostPortEnd: 8081},
				{Proto: types.TCP, IP: ctrIP6.IP, Port: 81, HostIP: net.IPv6zero, HostPort: 8081, HostPortEnd: 8081},
				{Proto: types.UDP, IP: ctrIP4.IP, Port: 81, HostIP: net.IPv4zero, HostPort: 8081, HostPortEnd: 8081},
				{Proto: types.UDP, IP: ctrIP6.IP, Port: 81, HostIP: net.IPv6zero, HostPort: 8081, HostPortEnd: 8081},
				{Proto: types.TCP, IP: ctrIP4.IP, Port: 82, HostIP: net.IPv4zero, HostPort: 8083, HostPortEnd: 8083},
				{Proto: types.TCP, IP: ctrIP6.IP, Port: 82, HostIP: net.IPv6zero, HostPort: 8083, HostPortEnd: 8083},
				{Proto: types.UDP, IP: ctrIP4.IP, Port: 82, HostIP: net.IPv4zero, HostPort: 8083, HostPortEnd: 8083},
				{Proto: types.UDP, IP: ctrIP6.IP, Port: 82, HostIP: net.IPv6zero, HostPort: 8083, HostPortEnd: 8083},
			},
		},
		{
			name:     "host port range exhausted",
			epAddrV4: ctrIP4,
			epAddrV6: ctrIP6,
			cfg: []portmapperapi.PortBindingReq{
				{PortBinding: types.PortBinding{Proto: types.TCP, Port: 80, HostPort: 8080, HostPortEnd: 8082}},
				{PortBinding: types.PortBinding{Proto: types.TCP, Port: 81, HostPort: 8080, HostPortEnd: 8082}},
				{PortBinding: types.PortBinding{Proto: types.TCP, Port: 82, HostPort: 8080, HostPortEnd: 8082}},
			},
			enableProxy:  true,
			busyPortIPv4: 8081,
			expErr:       "failed to bind host port 0.0.0.0:8081/tcp: address already in use",
		},
		{
			name:     "map host ipv6 to ipv4 container with proxy",
			epAddrV4: ctrIP4,
			cfg: []portmapperapi.PortBindingReq{
				{PortBinding: types.PortBinding{Proto: types.TCP, HostIP: net.IPv4zero, Port: 80}},
				{PortBinding: types.PortBinding{Proto: types.TCP, HostIP: net.IPv6zero, Port: 80}},
			},
			enableProxy: true,
			expPBs: []types.PortBinding{
				{Proto: types.TCP, IP: ctrIP4.IP, Port: 80, HostIP: net.IPv4zero, HostPort: firstEphemPort},
				{Proto: types.TCP, IP: ctrIP4.IP, Port: 80, HostIP: net.IPv6zero, HostPort: firstEphemPort},
			},
		},
		{
			name:     "map to ipv4 container with proxy but noProxy6To4",
			epAddrV4: ctrIP4,
			cfg: []portmapperapi.PortBindingReq{
				{PortBinding: types.PortBinding{Proto: types.TCP, Port: 80}},
			},
			enableProxy: true,
			noProxy6To4: true,
			expPBs: []types.PortBinding{
				{Proto: types.TCP, IP: ctrIP4.IP, Port: 80, HostIP: net.IPv4zero, HostPort: firstEphemPort},
			},
		},
		{
			name:     "map host ipv6 to ipv4 container without proxy",
			epAddrV4: ctrIP4,
			cfg: []portmapperapi.PortBindingReq{
				{PortBinding: types.PortBinding{Proto: types.TCP, HostIP: net.IPv4zero, Port: 80}},
				{PortBinding: types.PortBinding{Proto: types.TCP, HostIP: net.IPv6zero, Port: 80}}, // silently ignored
			},
			hairpin: true,
			expPBs: []types.PortBinding{
				{Proto: types.TCP, IP: ctrIP4.IP, Port: 80, HostIP: net.IPv4zero, HostPort: firstEphemPort},
			},
		},
		{
			name:        "default host ip is nonzero v4",
			epAddrV4:    ctrIP4,
			epAddrV6:    ctrIP6,
			cfg:         []portmapperapi.PortBindingReq{{PortBinding: types.PortBinding{Proto: types.TCP, Port: 80}}},
			enableProxy: true,
			defHostIP:   newIPNet(t, "127.0.0.1/8").IP,
			expPBs: []types.PortBinding{
				{Proto: types.TCP, IP: ctrIP4.IP, Port: 80, HostIP: newIPNet(t, "127.0.0.1/8").IP, HostPort: firstEphemPort},
			},
		},
		{
			name:        "default host ip is nonzero IPv4-mapped IPv6",
			epAddrV4:    ctrIP4,
			epAddrV6:    ctrIP6,
			cfg:         []portmapperapi.PortBindingReq{{PortBinding: types.PortBinding{Proto: types.TCP, Port: 80}}},
			enableProxy: true,
			defHostIP:   newIPNet(t, "::ffff:127.0.0.1/72").IP,
			expPBs: []types.PortBinding{
				{Proto: types.TCP, IP: ctrIP4.IP, Port: 80, HostIP: newIPNet(t, "127.0.0.1/8").IP, HostPort: firstEphemPort},
			},
		},
		{
			name:        "default host ip is v6",
			epAddrV4:    ctrIP4,
			epAddrV6:    ctrIP6,
			cfg:         []portmapperapi.PortBindingReq{{PortBinding: types.PortBinding{Proto: types.TCP, Port: 80}}},
			enableProxy: true,
			defHostIP:   net.IPv6zero,
			expPBs: []types.PortBinding{
				{Proto: types.TCP, IP: ctrIP6.IP, Port: 80, HostIP: net.IPv6zero, HostPort: firstEphemPort},
			},
		},
		{
			name:        "default host ip is nonzero v6",
			epAddrV4:    ctrIP4,
			epAddrV6:    ctrIP6,
			cfg:         []portmapperapi.PortBindingReq{{PortBinding: types.PortBinding{Proto: types.TCP, Port: 80}}},
			enableProxy: true,
			defHostIP:   newIPNet(t, "::1/128").IP,
			expPBs: []types.PortBinding{
				{Proto: types.TCP, IP: ctrIP6.IP, Port: 80, HostIP: newIPNet(t, "::1/128").IP, HostPort: firstEphemPort},
			},
		},
		{
			name:     "error releasing bindings",
			epAddrV4: ctrIP4,
			epAddrV6: ctrIP6,
			cfg: []portmapperapi.PortBindingReq{
				{PortBinding: types.PortBinding{Proto: types.TCP, Port: 80, HostPort: 8080}},
				{PortBinding: types.PortBinding{Proto: types.TCP, Port: 22, HostPort: 2222}},
			},
			enableProxy: true,
			expPBs: []types.PortBinding{
				{Proto: types.TCP, IP: ctrIP4.IP, Port: 22, HostIP: net.IPv4zero, HostPort: 2222},
				{Proto: types.TCP, IP: ctrIP6.IP, Port: 22, HostIP: net.IPv6zero, HostPort: 2222},
				{Proto: types.TCP, IP: ctrIP4.IP, Port: 80, HostIP: net.IPv4zero, HostPort: 8080},
				{Proto: types.TCP, IP: ctrIP6.IP, Port: 80, HostIP: net.IPv6zero, HostPort: 8080},
			},
			expReleaseErr: "unmapping port binding 0.0.0.0:2222:172.19.0.2:22/tcp: failed to stop userland proxy: can't stop now\n" +
				"unmapping port binding [::]:2222:[fdf8:b88e:bb5c:3483::2]:22/tcp: failed to stop userland proxy: can't stop now\n" +
				"unmapping port binding 0.0.0.0:8080:172.19.0.2:80/tcp: failed to stop userland proxy: can't stop now\n" +
				"unmapping port binding [::]:8080:[fdf8:b88e:bb5c:3483::2]:80/tcp: failed to stop userland proxy: can't stop now",
		},
		{
			name:     "disable nat6",
			epAddrV4: ctrIP4,
			epAddrV6: ctrIP6,
			cfg: []portmapperapi.PortBindingReq{
				{PortBinding: types.PortBinding{Proto: types.TCP, Port: 22}},
				{PortBinding: types.PortBinding{Proto: types.TCP, Port: 80}},
			},
			enableProxy: true,
			gwMode6:     gwModeRouted,
			expPBs: []types.PortBinding{
				{Proto: types.TCP, IP: ctrIP4.IP, Port: 22, HostIP: net.IPv4zero, HostPort: firstEphemPort},
				{Proto: types.TCP, IP: ctrIP6.IP, Port: 22, HostIP: net.IPv6zero},
				{Proto: types.TCP, IP: ctrIP4.IP, Port: 80, HostIP: net.IPv4zero, HostPort: firstEphemPort + 1},
				{Proto: types.TCP, IP: ctrIP6.IP, Port: 80, HostIP: net.IPv6zero},
			},
		},
		{
			name:     "disable nat6 with ipv6 default binding",
			epAddrV4: ctrIP4,
			epAddrV6: ctrIP6,
			cfg: []portmapperapi.PortBindingReq{
				{PortBinding: types.PortBinding{Proto: types.TCP, Port: 22}},
				{PortBinding: types.PortBinding{Proto: types.TCP, Port: 80}},
			},
			enableProxy: true,
			gwMode6:     gwModeRouted,
			defHostIP:   net.IPv6loopback,
			expPBs: []types.PortBinding{
				{Proto: types.TCP, IP: ctrIP6.IP, Port: 22, HostIP: net.IPv6zero},
				{Proto: types.TCP, IP: ctrIP6.IP, Port: 80, HostIP: net.IPv6zero},
			},
		},
		{
			name:     "disable nat4",
			epAddrV4: ctrIP4,
			epAddrV6: ctrIP6,
			cfg: []portmapperapi.PortBindingReq{
				{PortBinding: types.PortBinding{Proto: types.TCP, Port: 22}},
				{PortBinding: types.PortBinding{Proto: types.TCP, Port: 80}},
			},
			enableProxy: true,
			gwMode4:     gwModeRouted,
			expPBs: []types.PortBinding{
				{Proto: types.TCP, IP: ctrIP4.IP, Port: 22, HostIP: net.IPv4zero},
				{Proto: types.TCP, IP: ctrIP6.IP, Port: 22, HostIP: net.IPv6zero, HostPort: firstEphemPort},
				{Proto: types.TCP, IP: ctrIP4.IP, Port: 80, HostIP: net.IPv4zero},
				{Proto: types.TCP, IP: ctrIP6.IP, Port: 80, HostIP: net.IPv6zero, HostPort: firstEphemPort + 1},
			},
		},
		{
			name:     "disable nat",
			epAddrV4: ctrIP4,
			epAddrV6: ctrIP6,
			cfg: []portmapperapi.PortBindingReq{
				{PortBinding: types.PortBinding{Proto: types.TCP, Port: 22}},
				{PortBinding: types.PortBinding{Proto: types.TCP, Port: 80}},
			},
			enableProxy: true,
			gwMode4:     gwModeRouted,
			gwMode6:     gwModeRouted,
			expPBs: []types.PortBinding{
				{Proto: types.TCP, IP: ctrIP4.IP, Port: 22, HostIP: net.IPv4zero},
				{Proto: types.TCP, IP: ctrIP6.IP, Port: 22, HostIP: net.IPv6zero},
				{Proto: types.TCP, IP: ctrIP4.IP, Port: 80, HostIP: net.IPv4zero},
				{Proto: types.TCP, IP: ctrIP6.IP, Port: 80, HostIP: net.IPv6zero},
			},
		},
		{
			name:     "ipv6 mapping to ipv4 container no proxy",
			epAddrV4: ctrIP4,
			cfg: []portmapperapi.PortBindingReq{
				{PortBinding: types.PortBinding{Proto: types.TCP, Port: 22, HostIP: net.IPv6loopback}},
			},
			hairpin: true,
			expLogs: []string{"Cannot map from IPv6 to an IPv4-only container because the userland proxy is disabled"},
		},
		{
			name:      "ipv6 default mapping to ipv4 container no proxy",
			epAddrV4:  ctrIP4,
			defHostIP: net.IPv6loopback,
			cfg: []portmapperapi.PortBindingReq{
				{PortBinding: types.PortBinding{Proto: types.TCP, Port: 22}},
			},
			hairpin: true,
			expLogs: []string{"Cannot map from default host binding address to an IPv4-only container because the userland proxy is disabled"},
		},
		{
			name:        "routed mode specific address",
			epAddrV4:    ctrIP4,
			epAddrV6:    ctrIP6,
			gwMode4:     gwModeRouted,
			gwMode6:     gwModeRouted,
			enableProxy: true,
			cfg: []portmapperapi.PortBindingReq{
				{PortBinding: types.PortBinding{Proto: types.TCP, Port: 22, HostIP: newIPNet(t, "127.0.0.1/8").IP}},
				{PortBinding: types.PortBinding{Proto: types.TCP, Port: 22, HostIP: net.IPv6loopback}},
			},
			expLogs: []string{
				"Using address 0.0.0.0 because NAT is disabled",
				"Using address [::] because NAT is disabled",
			},
			expPBs: []types.PortBinding{
				{Proto: types.TCP, IP: ctrIP4.IP, Port: 22, HostIP: net.IPv4zero},
				{Proto: types.TCP, IP: ctrIP6.IP, Port: 22, HostIP: net.IPv6zero},
			},
		},
		{
			name:        "routed4 nat6 with ipv4 default binding",
			epAddrV4:    ctrIP4,
			epAddrV6:    ctrIP6,
			gwMode4:     gwModeRouted,
			defHostIP:   newIPNet(t, "127.0.0.1/8").IP,
			enableProxy: true,
			cfg: []portmapperapi.PortBindingReq{
				{PortBinding: types.PortBinding{Proto: types.TCP, Port: 22}},
			},
			expPBs: []types.PortBinding{
				{Proto: types.TCP, IP: ctrIP4.IP, Port: 22, HostIP: net.IPv4zero},
			},
		},
		{
			name:        "routed4 nat6 with ipv6 default binding",
			epAddrV4:    ctrIP4,
			epAddrV6:    ctrIP6,
			gwMode4:     gwModeRouted,
			defHostIP:   net.IPv6loopback,
			enableProxy: true,
			cfg: []portmapperapi.PortBindingReq{
				{PortBinding: types.PortBinding{Proto: types.TCP, Port: 22}},
			},
			expPBs: []types.PortBinding{
				{Proto: types.TCP, IP: ctrIP6.IP, Port: 22, HostIP: net.IPv6loopback, HostPort: firstEphemPort},
			},
		},
		{
			name:     "routed with host port",
			epAddrV4: ctrIP4,
			epAddrV6: ctrIP6,
			gwMode4:  gwModeRouted,
			gwMode6:  gwModeRouted,
			cfg: []portmapperapi.PortBindingReq{
				{PortBinding: types.PortBinding{Proto: types.TCP, Port: 22, HostPort: 2222}},
			},
			hairpin: true,
			expPBs: []types.PortBinding{
				{Proto: types.TCP, IP: ctrIP4.IP, Port: 22, HostIP: net.IPv4zero},
				{Proto: types.TCP, IP: ctrIP6.IP, Port: 22, HostIP: net.IPv6zero},
			},
			expLogs: []string{
				"Host port ignored, because NAT is disabled",
				"0.0.0.0:2222:172.19.0.2:22/tcp",
				"[::]:2222:[fdf8:b88e:bb5c:3483::2]:22/tcp",
			},
		},
		{
			name:      "same ports for matching mappings with different host addresses",
			epAddrV4:  ctrIP4,
			epAddrV6:  ctrIP6,
			hostAddrs: []string{"192.168.1.2/24", "fd0c:9167:5b11::2/64", "fd0c:9167:5b11::3/64"},
			cfg: []portmapperapi.PortBindingReq{
				// These two should both get the same host port.
				{PortBinding: types.PortBinding{Proto: types.TCP, Port: 80, HostIP: newIPNet(t, "fd0c:9167:5b11::2/64").IP}},
				{PortBinding: types.PortBinding{Proto: types.TCP, Port: 80, HostIP: newIPNet(t, "192.168.1.2/24").IP}},
				// These three should all get the same host port.
				{PortBinding: types.PortBinding{Proto: types.TCP, Port: 22, HostIP: newIPNet(t, "fd0c:9167:5b11::2/64").IP}},
				{PortBinding: types.PortBinding{Proto: types.TCP, Port: 22, HostIP: newIPNet(t, "fd0c:9167:5b11::3/64").IP}},
				{PortBinding: types.PortBinding{Proto: types.TCP, Port: 22, HostIP: newIPNet(t, "192.168.1.2/24").IP}},
				// These two should get different host ports, and the exact-port should be allocated
				// before the range.
				{PortBinding: types.PortBinding{Proto: types.TCP, Port: 12345, HostPort: 12345, HostPortEnd: 12346}},
				{PortBinding: types.PortBinding{Proto: types.TCP, Port: 12345, HostPort: 12345}},
			},
			enableProxy: true,
			expPBs: []types.PortBinding{
				{Proto: types.TCP, IP: ctrIP4.IP, Port: 12345, HostIP: net.IPv4zero, HostPort: 12345},
				{Proto: types.TCP, IP: ctrIP6.IP, Port: 12345, HostIP: net.IPv6zero, HostPort: 12345},
				{Proto: types.TCP, IP: ctrIP4.IP, Port: 22, HostIP: newIPNet(t, "192.168.1.2/24").IP, HostPort: firstEphemPort},
				{Proto: types.TCP, IP: ctrIP6.IP, Port: 22, HostIP: newIPNet(t, "fd0c:9167:5b11::2/64").IP, HostPort: firstEphemPort},
				{Proto: types.TCP, IP: ctrIP6.IP, Port: 22, HostIP: newIPNet(t, "fd0c:9167:5b11::3/64").IP, HostPort: firstEphemPort},
				{Proto: types.TCP, IP: ctrIP4.IP, Port: 80, HostIP: newIPNet(t, "192.168.1.2/24").IP, HostPort: firstEphemPort + 1},
				{Proto: types.TCP, IP: ctrIP6.IP, Port: 80, HostIP: newIPNet(t, "fd0c:9167:5b11::2/64").IP, HostPort: firstEphemPort + 1},
				{Proto: types.TCP, IP: ctrIP4.IP, Port: 12345, HostIP: net.IPv4zero, HostPort: 12346},
				{Proto: types.TCP, IP: ctrIP6.IP, Port: 12345, HostIP: net.IPv6zero, HostPort: 12346},
			},
		},
		{
			name:     "rootless",
			epAddrV4: ctrIP4,
			epAddrV6: ctrIP6,
			cfg: []portmapperapi.PortBindingReq{
				{PortBinding: types.PortBinding{Proto: types.TCP, Port: 22}},
				{PortBinding: types.PortBinding{Proto: types.TCP, Port: 80}},
			},
			enableProxy: true,
			rootless:    true,
			expPBs: []types.PortBinding{
				{Proto: types.TCP, IP: ctrIP4.IP, Port: 22, HostIP: net.IPv4zero, HostPort: firstEphemPort},
				{Proto: types.TCP, IP: ctrIP6.IP, Port: 22, HostIP: net.IPv6zero, HostPort: firstEphemPort},
				{Proto: types.TCP, IP: ctrIP4.IP, Port: 80, HostIP: net.IPv4zero, HostPort: firstEphemPort + 1},
				{Proto: types.TCP, IP: ctrIP6.IP, Port: 80, HostIP: net.IPv6zero, HostPort: firstEphemPort + 1},
			},
		},
		{
			name:     "rootless without proxy",
			epAddrV4: ctrIP4,
			epAddrV6: ctrIP6,
			cfg: []portmapperapi.PortBindingReq{
				{PortBinding: types.PortBinding{Proto: types.TCP, Port: 22}},
				{PortBinding: types.PortBinding{Proto: types.TCP, Port: 80}},
			},
			rootless: true,
			hairpin:  true,
			expPBs: []types.PortBinding{
				{Proto: types.TCP, IP: ctrIP4.IP, Port: 22, HostIP: net.IPv4zero, HostPort: firstEphemPort},
				{Proto: types.TCP, IP: ctrIP6.IP, Port: 22, HostIP: net.IPv6zero, HostPort: firstEphemPort},
				{Proto: types.TCP, IP: ctrIP4.IP, Port: 80, HostIP: net.IPv4zero, HostPort: firstEphemPort + 1},
				{Proto: types.TCP, IP: ctrIP6.IP, Port: 80, HostIP: net.IPv6zero, HostPort: firstEphemPort + 1},
			},
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			defer netnsutils.SetupTestOSContext(t)()
			useStubFirewaller(t)

			// Mock the startProxy function used by the code under test.
			proxies := map[proxyCall]bool{} // proxy -> is not stopped
			startProxy := func(pb types.PortBinding, listenSock *os.File) (stop func() error, retErr error) {
				if tc.busyPortIPv4 > 0 && tc.busyPortIPv4 == int(pb.HostPort) && pb.HostIP.To4() != nil {
					return nil, errors.New("busy port")
				}
				c := newProxyCall(pb.Proto.String(), pb.HostIP, int(pb.HostPort), pb.IP, int(pb.Port))
				if _, ok := proxies[c]; ok {
					return nil, fmt.Errorf("duplicate proxy: %#v", c)
				}
				proxies[c] = true
				return func() error {
					if tc.expReleaseErr != "" {
						return errors.New("can't stop now")
					}
					if !proxies[c] {
						return errors.New("already stopped")
					}
					proxies[c] = false
					return nil
				}, nil
			}

			if len(tc.hostAddrs) > 0 {
				dummyLink := &netlink.Bridge{LinkAttrs: netlink.LinkAttrs{Name: "br-dummy"}}
				err := netlink.LinkAdd(dummyLink)
				assert.NilError(t, err)
				for _, addr := range tc.hostAddrs {
					// Add with NODAD so that the address is available immediately.
					err := netlink.AddrAdd(dummyLink,
						&netlink.Addr{IPNet: newIPNet(t, addr), Flags: syscall.IFA_F_NODAD})
					assert.NilError(t, err)
				}
				err = netlink.LinkSetUp(dummyLink)
				assert.NilError(t, err)
			}
			if tc.busyPortIPv4 != 0 {
				tl, err := net.ListenTCP("tcp4", &net.TCPAddr{IP: net.IPv4zero, Port: tc.busyPortIPv4})
				assert.NilError(t, err)
				defer tl.Close()
				ul, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4zero, Port: tc.busyPortIPv4})
				assert.NilError(t, err)
				defer ul.Close()
			}

			var pdc nat.PortDriverClient
			if tc.rootless {
				pdc = newMockPortDriverClient()
			}

			pms := &drvregistry.PortMappers{}
			err := nat.Register(pms, nat.Config{
				RlkClient:   pdc,
				EnableProxy: tc.enableProxy,
				StartProxy:  startProxy,
			})
			assert.NilError(t, err)
			err = routed.Register(pms)
			assert.NilError(t, err)

			n := &bridgeNetwork{
				config: &networkConfiguration{
					BridgeName: "dummybridge",
					EnableIPv4: tc.epAddrV4 != nil,
					EnableIPv6: tc.epAddrV6 != nil,
					GwModeIPv4: tc.gwMode4,
					GwModeIPv6: tc.gwMode6,
				},
				bridge: &bridgeInterface{},
				driver: newDriver(storeutils.NewTempStore(t), pms),
			}
			genericOption := map[string]any{
				netlabel.GenericData: &configuration{
					EnableIPTables:  true,
					EnableIP6Tables: true,
					Hairpin:         tc.hairpin,
				},
			}
			err = n.driver.configure(genericOption)
			assert.NilError(t, err)
			fwn, err := n.newFirewallerNetwork(context.Background())
			assert.NilError(t, err)
			assert.Check(t, fwn != nil, "no firewaller network")
			n.firewallerNetwork = fwn

			expChildIP := func(hostIP net.IP) net.IP {
				if !tc.rootless {
					return hostIP
				}
				if hostIP.To4() == nil {
					return net.ParseIP("::1")
				}
				return net.ParseIP("127.0.0.1")
			}

			portallocator.Get().ReleaseAll()

			// Capture logs by stashing a new logger in the context.
			var sb strings.Builder
			logger := logrus.New()
			logger.Out = &sb
			ctx := log.WithLogger(context.Background(), &log.Entry{Logger: logger})
			t.Cleanup(func() {
				if t.Failed() {
					t.Logf("Daemon logs:\n%s", sb.String())
				}
			})

			ep := &bridgeEndpoint{
				id:     "dummyep",
				nid:    "dummynetwork",
				addr:   tc.epAddrV4,
				addrv6: tc.epAddrV6,
			}
			pbm := portBindingMode{routed: true}
			if ep.addr != nil {
				pbm.ipv4 = true
			}
			if ep.addrv6 != nil || (!tc.noProxy6To4 && ep.addr != nil) {
				pbm.ipv6 = true
			}
			pbs, err := n.addPortMappings(ctx, ep, tc.cfg, tc.defHostIP, pbm)
			if tc.expErr != "" {
				assert.ErrorContains(t, err, tc.expErr)
				return
			}
			assert.NilError(t, err)
			for _, expLog := range tc.expLogs {
				assert.Check(t, is.Contains(sb.String(), expLog))
			}
			assert.Assert(t, is.Len(pbs, len(tc.expPBs)))

			fw := n.driver.firewaller.(*firewaller.StubFirewaller)
			assert.Check(t, is.Equal(fw.Hairpin, !tc.enableProxy))
			assert.Check(t, fw.IPv4)
			assert.Check(t, fw.IPv6)

			fnw := n.firewallerNetwork.(*firewaller.StubFirewallerNetwork)
			assert.Check(t, !fnw.Internal)
			assert.Check(t, !fnw.ICC)
			assert.Check(t, !fnw.Masquerade)

			if n.config.HostIPv4 == nil {
				assert.Check(t, !fnw.Config4.HostIP.IsValid())
			} else {
				assert.Check(t, is.Equal(fnw.Config4.HostIP.String(), n.config.HostIPv4.String()))
			}
			assert.Check(t, is.Equal(fnw.Config4.Routed, tc.gwMode4.routed()))
			assert.Check(t, !fnw.Config4.Unprotected)

			if n.config.HostIPv6 == nil {
				assert.Check(t, !fnw.Config6.HostIP.IsValid())
			} else {
				assert.Check(t, is.Equal(fnw.Config6.HostIP.String(), n.config.HostIPv6.String()))
			}
			assert.Check(t, is.Equal(fnw.Config6.Routed, tc.gwMode6.routed()))
			assert.Check(t, !fnw.Config6.Unprotected)

			assert.Check(t, is.Len(fnw.Ports, len(tc.expPBs)))
			for _, expPB := range tc.expPBs {
				expPBCopy := expPB
				expPBCopy.HostPortEnd = expPB.HostPort
				expPBCopy.HostIP = expChildIP(expPB.HostIP)
				assert.Check(t, fnw.PortExists(expPBCopy),
					"expected port mapping %v (%v)", expPBCopy, expChildIP(expPB.HostIP))
			}

			// Release anything that was allocated.
			err = n.releasePorts(&bridgeEndpoint{portMapping: pbs})
			if tc.expReleaseErr == "" {
				assert.Check(t, err)
			} else {
				assert.Check(t, is.Error(err, tc.expReleaseErr))
			}

			// Check a docker-proxy was started and stopped for each expected port binding.
			if tc.enableProxy {
				expProxies := map[proxyCall]bool{}
				for _, expPB := range tc.expPBs {
					hip := expChildIP(expPB.HostIP)
					is4 := hip.To4() != nil
					if (is4 && tc.gwMode4.routed()) || (!is4 && tc.gwMode6.routed()) {
						continue
					}
					p := newProxyCall(expPB.Proto.String(),
						hip, int(expPB.HostPort),
						expPB.IP, int(expPB.Port))
					expProxies[p] = tc.expReleaseErr != ""
				}
				assert.Check(t, is.DeepEqual(expProxies, proxies))
			}

			// Check the port driver has seen the expected port mappings and no others,
			// and that they have all been closed.
			if pdc != nil {
				pdc := pdc.(*mockPortDriverClient)
				expPorts := map[mockPortDriverPort]bool{}
				for _, expPB := range tc.expPBs {
					if expPB.HostPort == 0 {
						continue
					}
					pdp := mockPortDriverPort{
						proto:    expPB.Proto.String(),
						hostIP:   expPB.HostIP.String(),
						childIP:  expChildIP(expPB.HostIP).String(),
						hostPort: int(expPB.HostPort),
					}
					expPorts[pdp] = false
				}
				assert.Check(t, is.DeepEqual(pdc.openPorts, expPorts))
			}
		})
	}
}

// Type for tracking calls to StartProxy.
type proxyCall struct{ proto, host, container string }

func newProxyCall(proto string,
	hostIP net.IP, hostPort int,
	containerIP net.IP, containerPort int,
) proxyCall {
	return proxyCall{
		proto:     proto,
		host:      fmt.Sprintf("%v:%v", hostIP, hostPort),
		container: fmt.Sprintf("%v:%v", containerIP, containerPort),
	}
}

// Types for tracking calls to the port driver client (mock for RootlessKit client).

type mockPortDriverPort struct {
	proto    string
	hostIP   string
	childIP  string
	hostPort int
}

func (p mockPortDriverPort) String() string {
	return p.hostIP + ":" + strconv.Itoa(p.hostPort) + "/" + p.proto
}

type mockPortDriverClient struct {
	openPorts map[mockPortDriverPort]bool
}

func newMockPortDriverClient() *mockPortDriverClient {
	return &mockPortDriverClient{
		openPorts: map[mockPortDriverPort]bool{},
	}
}

func (c *mockPortDriverClient) ChildHostIP(hostIP netip.Addr) netip.Addr {
	if hostIP.Is6() {
		return netip.IPv6Loopback()
	}
	return netip.MustParseAddr("127.0.0.1")
}

func (c *mockPortDriverClient) AddPort(_ context.Context, proto string, hostIP, childIP netip.Addr, hostPort int) (func() error, error) {
	key := mockPortDriverPort{proto: proto, hostIP: hostIP.String(), childIP: childIP.String(), hostPort: hostPort}
	if _, exists := c.openPorts[key]; exists {
		return nil, fmt.Errorf("mockPortDriverClient: port %s is already open", key)
	}
	c.openPorts[key] = true
	return func() error {
		if !c.openPorts[key] {
			return fmt.Errorf("mockPortDriverClient: port %s is not open", key)
		}
		c.openPorts[key] = false
		return nil
	}, nil
}

type stubPortMapper struct {
	reqs   [][]portmapperapi.PortBindingReq
	mapped []portmapperapi.PortBinding
}

func (pm *stubPortMapper) MapPorts(_ context.Context, reqs []portmapperapi.PortBindingReq, _ portmapperapi.Firewaller) ([]portmapperapi.PortBinding, error) {
	if len(reqs) == 0 {
		return []portmapperapi.PortBinding{}, nil
	}
	pm.reqs = append(pm.reqs, reqs)
	pbs := sliceutil.Map(reqs, func(req portmapperapi.PortBindingReq) portmapperapi.PortBinding {
		return portmapperapi.PortBinding{PortBinding: req.PortBinding}
	})
	pm.mapped = append(pm.mapped, pbs...)
	return pbs, nil
}

func (pm *stubPortMapper) UnmapPorts(_ context.Context, reqs []portmapperapi.PortBinding, _ portmapperapi.Firewaller) error {
	for _, req := range reqs {
		// We're only checking for the PortBinding here, not any other
		// property of [portmapperapi.PortBinding].
		idx := slices.IndexFunc(pm.mapped, func(pb portmapperapi.PortBinding) bool {
			return pb.Equal(req.PortBinding)
		})
		if idx == -1 {
			return fmt.Errorf("stubPortMapper.UnmapPorts: pb doesn't exist %v", req)
		}
		pm.mapped = slices.Delete(pm.mapped, idx, idx+1)
	}
	return nil
}
