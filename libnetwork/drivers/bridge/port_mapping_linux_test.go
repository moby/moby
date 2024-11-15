package bridge

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"os"
	"strconv"
	"strings"
	"syscall"
	"testing"

	"github.com/containerd/log"
	"github.com/docker/docker/internal/testutils/netnsutils"
	"github.com/docker/docker/libnetwork/iptables"
	"github.com/docker/docker/libnetwork/netlabel"
	"github.com/docker/docker/libnetwork/ns"
	"github.com/docker/docker/libnetwork/portallocator"
	"github.com/docker/docker/libnetwork/types"
	"github.com/sirupsen/logrus"
	"github.com/vishvananda/netlink"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestPortMappingConfig(t *testing.T) {
	defer netnsutils.SetupTestOSContext(t)()
	d := newDriver()

	config := &configuration{
		EnableIPTables: true,
	}
	genericOption := make(map[string]interface{})
	genericOption[netlabel.GenericData] = config

	if err := d.configure(genericOption); err != nil {
		t.Fatalf("Failed to setup driver config: %v", err)
	}

	binding1 := types.PortBinding{Proto: types.SCTP, Port: uint16(300), HostPort: uint16(65000)}
	binding2 := types.PortBinding{Proto: types.UDP, Port: uint16(400), HostPort: uint16(54000)}
	binding3 := types.PortBinding{Proto: types.TCP, Port: uint16(500), HostPort: uint16(65000)}
	portBindings := []types.PortBinding{binding1, binding2, binding3}

	sbOptions := make(map[string]interface{})
	sbOptions[netlabel.PortMap] = portBindings

	netOptions := map[string]interface{}{
		netlabel.GenericData: &networkConfiguration{
			BridgeName: DefaultBridgeName,
			EnableIPv4: true,
		},
	}

	ipdList4 := getIPv4Data(t)
	err := d.CreateNetwork("dummy", netOptions, nil, ipdList4, getIPv6Data(t))
	if err != nil {
		t.Fatalf("Failed to create bridge: %v", err)
	}

	te := newTestEndpoint(ipdList4[0].Pool, 11)
	err = d.CreateEndpoint(context.Background(), "dummy", "ep1", te.Interface(), nil)
	if err != nil {
		t.Fatalf("Failed to create the endpoint: %s", err.Error())
	}

	if err = d.Join(context.Background(), "dummy", "ep1", "sbox", te, sbOptions); err != nil {
		t.Fatalf("Failed to join the endpoint: %v", err)
	}

	if err = d.ProgramExternalConnectivity(context.Background(), "dummy", "ep1", sbOptions); err != nil {
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

	err = d.RevokeExternalConnectivity("dummy", "ep1")
	if err != nil {
		t.Fatal(err)
	}
}

func TestPortMappingV6Config(t *testing.T) {
	defer netnsutils.SetupTestOSContext(t)()
	if err := loopbackUp(); err != nil {
		t.Fatalf("Could not bring loopback iface up: %v", err)
	}

	d := newDriver()

	config := &configuration{
		EnableIPTables:  true,
		EnableIP6Tables: true,
	}
	genericOption := make(map[string]interface{})
	genericOption[netlabel.GenericData] = config

	if err := d.configure(genericOption); err != nil {
		t.Fatalf("Failed to setup driver config: %v", err)
	}

	portBindings := []types.PortBinding{
		{Proto: types.UDP, Port: uint16(400), HostPort: uint16(54000)},
		{Proto: types.TCP, Port: uint16(500), HostPort: uint16(65000)},
		{Proto: types.SCTP, Port: uint16(500), HostPort: uint16(65000)},
	}

	sbOptions := make(map[string]interface{})
	sbOptions[netlabel.PortMap] = portBindings
	netConfig := &networkConfiguration{
		BridgeName: DefaultBridgeName,
		EnableIPv6: true,
	}
	netOptions := make(map[string]interface{})
	netOptions[netlabel.GenericData] = netConfig

	ipdList4 := getIPv4Data(t)
	err := d.CreateNetwork("dummy", netOptions, nil, ipdList4, getIPv6Data(t))
	if err != nil {
		t.Fatalf("Failed to create bridge: %v", err)
	}

	te := newTestEndpoint(ipdList4[0].Pool, 11)
	err = d.CreateEndpoint(context.Background(), "dummy", "ep1", te.Interface(), nil)
	if err != nil {
		t.Fatalf("Failed to create the endpoint: %s", err.Error())
	}

	if err = d.Join(context.Background(), "dummy", "ep1", "sbox", te, sbOptions); err != nil {
		t.Fatalf("Failed to join the endpoint: %v", err)
	}

	if err = d.ProgramExternalConnectivity(context.Background(), "dummy", "ep1", sbOptions); err != nil {
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

	err = d.RevokeExternalConnectivity("dummy", "ep1")
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

func TestCmpPortBindings(t *testing.T) {
	pb := types.PortBinding{
		Proto:       types.TCP,
		IP:          net.ParseIP("172.17.0.2"),
		Port:        80,
		HostIP:      net.ParseIP("192.168.1.2"),
		HostPort:    8080,
		HostPortEnd: 8080,
	}
	var pbA, pbB types.PortBinding

	assert.Check(t, cmpPortBinding(pb, pb) == 0)

	pbA, pbB = pb, pb
	pbA.Port = 22
	assert.Check(t, cmpPortBinding(pbA, pbB) < 0)
	assert.Check(t, cmpPortBinding(pbB, pbA) > 0)

	pbA, pbB = pb, pb
	pbB.Proto = types.UDP
	assert.Check(t, cmpPortBinding(pbA, pbB) < 0)
	assert.Check(t, cmpPortBinding(pbB, pbA) > 0)

	pbA, pbB = pb, pb
	pbA.Port = 22
	pbA.Proto = types.UDP
	assert.Check(t, cmpPortBinding(pbA, pbB) < 0)
	assert.Check(t, cmpPortBinding(pbB, pbA) > 0)

	pbA, pbB = pb, pb
	pbB.HostPort = 8081
	assert.Check(t, cmpPortBinding(pbA, pbB) < 0)
	assert.Check(t, cmpPortBinding(pbB, pbA) > 0)

	pbA, pbB = pb, pb
	pbB.HostPort, pbB.HostPortEnd = 0, 0
	assert.Check(t, cmpPortBinding(pbA, pbB) < 0)
	assert.Check(t, cmpPortBinding(pbB, pbA) > 0)

	pbA, pbB = pb, pb
	pbB.HostPortEnd = 8081
	assert.Check(t, cmpPortBinding(pbA, pbB) < 0)
	assert.Check(t, cmpPortBinding(pbB, pbA) > 0)

	pbA, pbB = pb, pb
	pbA.HostPortEnd = 8080
	pbB.HostPortEnd = 8081
	assert.Check(t, cmpPortBinding(pbA, pbB) < 0)
	assert.Check(t, cmpPortBinding(pbB, pbA) > 0)
}

func TestBindHostPortsError(t *testing.T) {
	cfg := []portBindingReq{
		{
			PortBinding: types.PortBinding{
				Proto:       types.TCP,
				Port:        80,
				HostPort:    8080,
				HostPortEnd: 8080,
			},
		},
		{
			PortBinding: types.PortBinding{
				Proto:       types.TCP,
				Port:        80,
				HostPort:    8080,
				HostPortEnd: 8081,
			},
		},
	}
	pbs, err := bindHostPorts(context.Background(), cfg, "")
	assert.Check(t, is.Error(err, "port binding mismatch 80/tcp:8080-8080, 80/tcp:8080-8081"))
	assert.Check(t, pbs == nil)
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
		cfg          []types.PortBinding
		defHostIP    net.IP
		proxyPath    string
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
			cfg: []types.PortBinding{
				{Proto: types.TCP, Port: 22},
				{Proto: types.TCP, Port: 80},
			},
			proxyPath: "/dummy/path/to/proxy",
			expPBs: []types.PortBinding{
				{Proto: types.TCP, IP: ctrIP4.IP, Port: 22, HostIP: net.IPv4zero, HostPort: firstEphemPort},
				{Proto: types.TCP, IP: ctrIP6.IP, Port: 22, HostIP: net.IPv6zero, HostPort: firstEphemPort},
				{Proto: types.TCP, IP: ctrIP4.IP, Port: 80, HostIP: net.IPv4zero, HostPort: firstEphemPort + 1},
				{Proto: types.TCP, IP: ctrIP6.IP, Port: 80, HostIP: net.IPv6zero, HostPort: firstEphemPort + 1},
			},
		},
		{
			name:      "specific host port",
			epAddrV4:  ctrIP4,
			epAddrV6:  ctrIP6,
			cfg:       []types.PortBinding{{Proto: types.TCP, Port: 80, HostPort: 8080}},
			proxyPath: "/dummy/path/to/proxy",
			expPBs: []types.PortBinding{
				{Proto: types.TCP, IP: ctrIP4.IP, Port: 80, HostIP: net.IPv4zero, HostPort: 8080, HostPortEnd: 8080},
				{Proto: types.TCP, IP: ctrIP6.IP, Port: 80, HostIP: net.IPv6zero, HostPort: 8080, HostPortEnd: 8080},
			},
		},
		{
			name:      "nat explicitly enabled",
			epAddrV4:  ctrIP4,
			epAddrV6:  ctrIP6,
			cfg:       []types.PortBinding{{Proto: types.TCP, Port: 80, HostPort: 8080}},
			gwMode4:   gwModeNAT,
			gwMode6:   gwModeNAT,
			proxyPath: "/dummy/path/to/proxy",
			expPBs: []types.PortBinding{
				{Proto: types.TCP, IP: ctrIP4.IP, Port: 80, HostIP: net.IPv4zero, HostPort: 8080, HostPortEnd: 8080},
				{Proto: types.TCP, IP: ctrIP6.IP, Port: 80, HostIP: net.IPv6zero, HostPort: 8080, HostPortEnd: 8080},
			},
		},
		{
			name:         "specific host port in-use",
			epAddrV4:     ctrIP4,
			epAddrV6:     ctrIP6,
			cfg:          []types.PortBinding{{Proto: types.TCP, Port: 80, HostPort: 8080}},
			proxyPath:    "/dummy/path/to/proxy",
			busyPortIPv4: 8080,
			expErr:       "failed to bind host port for 0.0.0.0:8080:172.19.0.2:80/tcp: address already in use",
		},
		{
			name:      "ipv4 mapped container address with specific host port",
			epAddrV4:  ctrIP4Mapped,
			epAddrV6:  ctrIP6,
			cfg:       []types.PortBinding{{Proto: types.TCP, Port: 80, HostPort: 8080}},
			proxyPath: "/dummy/path/to/proxy",
			expPBs: []types.PortBinding{
				{Proto: types.TCP, IP: ctrIP4.IP, Port: 80, HostIP: net.IPv4zero, HostPort: 8080, HostPortEnd: 8080},
				{Proto: types.TCP, IP: ctrIP6.IP, Port: 80, HostIP: net.IPv6zero, HostPort: 8080, HostPortEnd: 8080},
			},
		},
		{
			name:      "ipv4 mapped host address with specific host port",
			epAddrV4:  ctrIP4,
			epAddrV6:  ctrIP6,
			cfg:       []types.PortBinding{{Proto: types.TCP, Port: 80, HostIP: newIPNet(t, "::ffff:127.0.0.1/128").IP, HostPort: 8080}},
			proxyPath: "/dummy/path/to/proxy",
			expPBs: []types.PortBinding{
				{Proto: types.TCP, IP: ctrIP4.IP, Port: 80, HostIP: newIPNet(t, "127.0.0.1/32").IP, HostPort: 8080, HostPortEnd: 8080},
			},
		},
		{
			name:         "host port range with first port in-use",
			epAddrV4:     ctrIP4,
			epAddrV6:     ctrIP6,
			cfg:          []types.PortBinding{{Proto: types.TCP, Port: 80, HostPort: 8080, HostPortEnd: 8081}},
			proxyPath:    "/dummy/path/to/proxy",
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
			cfg: []types.PortBinding{
				{Proto: types.TCP, Port: 80, HostIP: net.IPv4zero, HostPort: 8080, HostPortEnd: 8081},
				{Proto: types.TCP, Port: 80, HostIP: net.IPv6zero, HostPort: 8080, HostPortEnd: 8081},
			},
			proxyPath:    "/dummy/path/to/proxy",
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
			cfg: []types.PortBinding{
				{Proto: types.TCP, Port: 80, HostPort: 8080, HostPortEnd: 8083},
				{Proto: types.TCP, Port: 81, HostPort: 8080, HostPortEnd: 8083},
				{Proto: types.TCP, Port: 82, HostPort: 8080, HostPortEnd: 8083},
				{Proto: types.UDP, Port: 80, HostPort: 8080, HostPortEnd: 8083},
				{Proto: types.UDP, Port: 81, HostPort: 8080, HostPortEnd: 8083},
				{Proto: types.UDP, Port: 82, HostPort: 8080, HostPortEnd: 8083},
			},
			proxyPath:    "/dummy/path/to/proxy",
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
			cfg: []types.PortBinding{
				{Proto: types.TCP, Port: 80, HostPort: 8080, HostPortEnd: 8082},
				{Proto: types.TCP, Port: 81, HostPort: 8080, HostPortEnd: 8082},
				{Proto: types.TCP, Port: 82, HostPort: 8080, HostPortEnd: 8082},
			},
			proxyPath:    "/dummy/path/to/proxy",
			busyPortIPv4: 8081,
			expErr:       "failed to bind host port 8081 for 0.0.0.0:8080-8082:172.19.0.2:82/tcp",
		},
		{
			name:     "map host ipv6 to ipv4 container with proxy",
			epAddrV4: ctrIP4,
			cfg: []types.PortBinding{
				{Proto: types.TCP, HostIP: net.IPv4zero, Port: 80},
				{Proto: types.TCP, HostIP: net.IPv6zero, Port: 80},
			},
			proxyPath: "/dummy/path/to/proxy",
			expPBs: []types.PortBinding{
				{Proto: types.TCP, IP: ctrIP4.IP, Port: 80, HostIP: net.IPv4zero, HostPort: firstEphemPort},
				{Proto: types.TCP, IP: ctrIP4.IP, Port: 80, HostIP: net.IPv6zero, HostPort: firstEphemPort},
			},
		},
		{
			name:     "map to ipv4 container with proxy but noProxy6To4",
			epAddrV4: ctrIP4,
			cfg: []types.PortBinding{
				{Proto: types.TCP, Port: 80},
			},
			proxyPath:   "/dummy/path/to/proxy",
			noProxy6To4: true,
			expPBs: []types.PortBinding{
				{Proto: types.TCP, IP: ctrIP4.IP, Port: 80, HostIP: net.IPv4zero, HostPort: firstEphemPort},
			},
		},
		{
			name:     "map host ipv6 to ipv4 container without proxy",
			epAddrV4: ctrIP4,
			cfg: []types.PortBinding{
				{Proto: types.TCP, HostIP: net.IPv4zero, Port: 80},
				{Proto: types.TCP, HostIP: net.IPv6zero, Port: 80}, // silently ignored
			},
			expPBs: []types.PortBinding{
				{Proto: types.TCP, IP: ctrIP4.IP, Port: 80, HostIP: net.IPv4zero, HostPort: firstEphemPort},
			},
		},
		{
			name:      "default host ip is nonzero v4",
			epAddrV4:  ctrIP4,
			epAddrV6:  ctrIP6,
			cfg:       []types.PortBinding{{Proto: types.TCP, Port: 80}},
			proxyPath: "/dummy/path/to/proxy",
			defHostIP: newIPNet(t, "127.0.0.1/8").IP,
			expPBs: []types.PortBinding{
				{Proto: types.TCP, IP: ctrIP4.IP, Port: 80, HostIP: newIPNet(t, "127.0.0.1/8").IP, HostPort: firstEphemPort},
			},
		},
		{
			name:      "default host ip is nonzero IPv4-mapped IPv6",
			epAddrV4:  ctrIP4,
			epAddrV6:  ctrIP6,
			cfg:       []types.PortBinding{{Proto: types.TCP, Port: 80}},
			proxyPath: "/dummy/path/to/proxy",
			defHostIP: newIPNet(t, "::ffff:127.0.0.1/72").IP,
			expPBs: []types.PortBinding{
				{Proto: types.TCP, IP: ctrIP4.IP, Port: 80, HostIP: newIPNet(t, "127.0.0.1/8").IP, HostPort: firstEphemPort},
			},
		},
		{
			name:      "default host ip is v6",
			epAddrV4:  ctrIP4,
			epAddrV6:  ctrIP6,
			cfg:       []types.PortBinding{{Proto: types.TCP, Port: 80}},
			proxyPath: "/dummy/path/to/proxy",
			defHostIP: net.IPv6zero,
			expPBs: []types.PortBinding{
				{Proto: types.TCP, IP: ctrIP6.IP, Port: 80, HostIP: net.IPv6zero, HostPort: firstEphemPort},
			},
		},
		{
			name:      "default host ip is nonzero v6",
			epAddrV4:  ctrIP4,
			epAddrV6:  ctrIP6,
			cfg:       []types.PortBinding{{Proto: types.TCP, Port: 80}},
			proxyPath: "/dummy/path/to/proxy",
			defHostIP: newIPNet(t, "::1/128").IP,
			expPBs: []types.PortBinding{
				{Proto: types.TCP, IP: ctrIP6.IP, Port: 80, HostIP: newIPNet(t, "::1/128").IP, HostPort: firstEphemPort},
			},
		},
		{
			name:     "error releasing bindings",
			epAddrV4: ctrIP4,
			epAddrV6: ctrIP6,
			cfg: []types.PortBinding{
				{Proto: types.TCP, Port: 80, HostPort: 8080},
				{Proto: types.TCP, Port: 22, HostPort: 2222},
			},
			proxyPath: "/dummy/path/to/proxy",
			expPBs: []types.PortBinding{
				{Proto: types.TCP, IP: ctrIP4.IP, Port: 22, HostIP: net.IPv4zero, HostPort: 2222},
				{Proto: types.TCP, IP: ctrIP6.IP, Port: 22, HostIP: net.IPv6zero, HostPort: 2222},
				{Proto: types.TCP, IP: ctrIP4.IP, Port: 80, HostIP: net.IPv4zero, HostPort: 8080},
				{Proto: types.TCP, IP: ctrIP6.IP, Port: 80, HostIP: net.IPv6zero, HostPort: 8080},
			},
			expReleaseErr: "failed to stop userland proxy for port mapping 0.0.0.0:2222:172.19.0.2:22/tcp: can't stop now\n" +
				"failed to stop userland proxy for port mapping [::]:2222:[fdf8:b88e:bb5c:3483::2]:22/tcp: can't stop now\n" +
				"failed to stop userland proxy for port mapping 0.0.0.0:8080:172.19.0.2:80/tcp: can't stop now\n" +
				"failed to stop userland proxy for port mapping [::]:8080:[fdf8:b88e:bb5c:3483::2]:80/tcp: can't stop now",
		},
		{
			name:     "disable nat6",
			epAddrV4: ctrIP4,
			epAddrV6: ctrIP6,
			cfg: []types.PortBinding{
				{Proto: types.TCP, Port: 22},
				{Proto: types.TCP, Port: 80},
			},
			proxyPath: "/dummy/path/to/proxy",
			gwMode6:   gwModeRouted,
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
			cfg: []types.PortBinding{
				{Proto: types.TCP, Port: 22},
				{Proto: types.TCP, Port: 80},
			},
			proxyPath: "/dummy/path/to/proxy",
			gwMode6:   gwModeRouted,
			defHostIP: net.IPv6loopback,
			expPBs: []types.PortBinding{
				{Proto: types.TCP, IP: ctrIP6.IP, Port: 22, HostIP: net.IPv6zero},
				{Proto: types.TCP, IP: ctrIP6.IP, Port: 80, HostIP: net.IPv6zero},
			},
		},
		{
			name:     "disable nat4",
			epAddrV4: ctrIP4,
			epAddrV6: ctrIP6,
			cfg: []types.PortBinding{
				{Proto: types.TCP, Port: 22},
				{Proto: types.TCP, Port: 80},
			},
			proxyPath: "/dummy/path/to/proxy",
			gwMode4:   gwModeRouted,
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
			cfg: []types.PortBinding{
				{Proto: types.TCP, Port: 22},
				{Proto: types.TCP, Port: 80},
			},
			proxyPath: "/dummy/path/to/proxy",
			gwMode4:   gwModeRouted,
			gwMode6:   gwModeRouted,
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
			cfg: []types.PortBinding{
				{Proto: types.TCP, Port: 22, HostIP: net.IPv6loopback},
			},
			expLogs: []string{"Cannot map from IPv6 to an IPv4-only container because the userland proxy is disabled"},
		},
		{
			name:      "ipv6 default mapping to ipv4 container no proxy",
			epAddrV4:  ctrIP4,
			defHostIP: net.IPv6loopback,
			cfg: []types.PortBinding{
				{Proto: types.TCP, Port: 22},
			},
			expLogs: []string{"Cannot map from default host binding address to an IPv4-only container because the userland proxy is disabled"},
		},
		{
			name:      "routed mode specific address",
			epAddrV4:  ctrIP4,
			epAddrV6:  ctrIP6,
			gwMode4:   gwModeRouted,
			gwMode6:   gwModeRouted,
			proxyPath: "/dummy/path/to/proxy",
			cfg: []types.PortBinding{
				{Proto: types.TCP, Port: 22, HostIP: newIPNet(t, "127.0.0.1/8").IP},
				{Proto: types.TCP, Port: 22, HostIP: net.IPv6loopback},
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
			name:      "routed4 nat6 with ipv4 default binding",
			epAddrV4:  ctrIP4,
			epAddrV6:  ctrIP6,
			gwMode4:   gwModeRouted,
			defHostIP: newIPNet(t, "127.0.0.1/8").IP,
			proxyPath: "/dummy/path/to/proxy",
			cfg: []types.PortBinding{
				{Proto: types.TCP, Port: 22},
			},
			expPBs: []types.PortBinding{
				{Proto: types.TCP, IP: ctrIP4.IP, Port: 22, HostIP: net.IPv4zero},
			},
		},
		{
			name:      "routed4 nat6 with ipv6 default binding",
			epAddrV4:  ctrIP4,
			epAddrV6:  ctrIP6,
			gwMode4:   gwModeRouted,
			defHostIP: net.IPv6loopback,
			proxyPath: "/dummy/path/to/proxy",
			cfg: []types.PortBinding{
				{Proto: types.TCP, Port: 22},
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
			cfg: []types.PortBinding{
				{Proto: types.TCP, Port: 22, HostPort: 2222},
			},
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
			cfg: []types.PortBinding{
				// These two should both get the same host port.
				{Proto: types.TCP, Port: 80, HostIP: newIPNet(t, "fd0c:9167:5b11::2/64").IP},
				{Proto: types.TCP, Port: 80, HostIP: newIPNet(t, "192.168.1.2/24").IP},
				// These three should all get the same host port.
				{Proto: types.TCP, Port: 22, HostIP: newIPNet(t, "fd0c:9167:5b11::2/64").IP},
				{Proto: types.TCP, Port: 22, HostIP: newIPNet(t, "fd0c:9167:5b11::3/64").IP},
				{Proto: types.TCP, Port: 22, HostIP: newIPNet(t, "192.168.1.2/24").IP},
				// These two should get different host ports, and the exact-port should be allocated
				// before the range.
				{Proto: types.TCP, Port: 12345, HostPort: 12345, HostPortEnd: 12346},
				{Proto: types.TCP, Port: 12345, HostPort: 12345},
			},
			proxyPath: "/dummy/path/to/proxy",
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
			cfg: []types.PortBinding{
				{Proto: types.TCP, Port: 22},
				{Proto: types.TCP, Port: 80},
			},
			proxyPath: "/dummy/path/to/proxy",
			rootless:  true,
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
			cfg: []types.PortBinding{
				{Proto: types.TCP, Port: 22},
				{Proto: types.TCP, Port: 80},
			},
			rootless: true,
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

			// Mock the startProxy function used by the code under test.
			origStartProxy := startProxy
			defer func() { startProxy = origStartProxy }()
			proxies := map[proxyCall]bool{} // proxy -> is not stopped
			startProxy = func(pb types.PortBinding,
				proxyPath string,
				listenSock *os.File,
			) (stop func() error, retErr error) {
				if tc.busyPortIPv4 > 0 && tc.busyPortIPv4 == int(pb.HostPort) && pb.HostIP.To4() != nil {
					return nil, errors.New("busy port")
				}
				c := newProxyCall(pb.Proto.String(), pb.HostIP, int(pb.HostPort), pb.IP, int(pb.Port), proxyPath)
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

			// Mock the RootlessKit port driver.
			origNewPortDriverClient := newPortDriverClient
			defer func() { newPortDriverClient = origNewPortDriverClient }()
			newPortDriverClient = func(ctx context.Context) (portDriverClient, error) {
				return newMockPortDriverClient(ctx)
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

			n := &bridgeNetwork{
				config: &networkConfiguration{
					BridgeName: "dummybridge",
					EnableIPv4: tc.epAddrV4 != nil,
					EnableIPv6: tc.epAddrV6 != nil,
					GwModeIPv4: tc.gwMode4,
					GwModeIPv6: tc.gwMode6,
				},
				driver: newDriver(),
			}
			genericOption := map[string]interface{}{
				netlabel.GenericData: &configuration{
					EnableIPTables:      true,
					EnableIP6Tables:     true,
					EnableUserlandProxy: tc.proxyPath != "",
					UserlandProxyPath:   tc.proxyPath,
					Rootless:            tc.rootless,
				},
			}
			err := n.driver.configure(genericOption)
			assert.NilError(t, err)

			assert.Check(t, is.Equal(n.driver.portDriverClient == nil, !tc.rootless))
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

			pbs, err := n.addPortMappings(ctx, tc.epAddrV4, tc.epAddrV6, tc.cfg, tc.defHostIP, tc.noProxy6To4)
			if tc.expErr != "" {
				assert.ErrorContains(t, err, tc.expErr)
				return
			}
			assert.NilError(t, err)
			for _, expLog := range tc.expLogs {
				assert.Check(t, is.Contains(sb.String(), expLog))
			}
			assert.Assert(t, is.Len(pbs, len(tc.expPBs)))

			// Check the iptables rules.
			for _, expPB := range tc.expPBs {
				var disableNAT bool
				var addrM, addrD, addrH string
				var ipv iptables.IPVersion
				hip := expChildIP(expPB.HostIP)
				if expPB.IP.To4() == nil {
					disableNAT = tc.gwMode6.natDisabled()
					ipv = iptables.IPv6
					addrM = ctrIP6.IP.String() + "/128"
					addrD = "[" + ctrIP6.IP.String() + "]"
					addrH = hip.String() + "/128"
				} else {
					disableNAT = tc.gwMode4.natDisabled()
					ipv = iptables.IPv4
					addrM = ctrIP4.IP.String() + "/32"
					addrD = ctrIP4.IP.String()
					addrH = hip.String() + "/32"
				}
				if hip.IsUnspecified() {
					addrH = "0/0"
				}

				// Check the MASQUERADE rule.
				masqRule := fmt.Sprintf("-s %s -d %s -p %s -m %s --dport %d -j MASQUERADE",
					addrM, addrM, expPB.Proto, expPB.Proto, expPB.Port)
				ir := iptRule{ipv: ipv, table: iptables.Nat, chain: "POSTROUTING", args: strings.Split(masqRule, " ")}
				if disableNAT || tc.proxyPath != "" {
					assert.Check(t, !ir.Exists(), fmt.Sprintf("unexpected rule %s", ir))
				} else {
					assert.Check(t, ir.Exists(), fmt.Sprintf("expected rule %s", ir))
				}

				// Check the DNAT rule.
				dnatRule := ""
				if ipv == iptables.IPv6 && !tc.gwMode6.natDisabled() {
					dnatRule += "! -s fe80::/10 "
				}
				if tc.proxyPath != "" {
					// No docker-proxy, so expect "hairpinMode".
					dnatRule += "! -i dummybridge "
				}
				dnatRule += fmt.Sprintf("-d %s -p %s -m %s --dport %d -j DNAT --to-destination %s:%d",
					addrH, expPB.Proto, expPB.Proto, expPB.HostPort, addrD, expPB.Port)
				ir = iptRule{ipv: ipv, table: iptables.Nat, chain: "DOCKER", args: strings.Split(dnatRule, " ")}
				if disableNAT {
					assert.Check(t, !ir.Exists(), fmt.Sprintf("unexpected rule %s", ir))
				} else {
					assert.Check(t, ir.Exists(), fmt.Sprintf("expected rule %s", ir))
				}

				// Check that the container's port is open.
				filterRule := fmt.Sprintf("-d %s ! -i dummybridge -o dummybridge -p %s -m %s --dport %d -j ACCEPT",
					addrM, expPB.Proto, expPB.Proto, expPB.Port)
				ir = iptRule{ipv: ipv, table: iptables.Filter, chain: "DOCKER", args: strings.Split(filterRule, " ")}
				assert.Check(t, ir.Exists(), fmt.Sprintf("expected rule %s", ir))
			}

			// Release anything that was allocated.
			err = n.releasePorts(&bridgeEndpoint{portMapping: pbs})
			if tc.expReleaseErr == "" {
				assert.Check(t, err)
			} else {
				assert.Check(t, is.Error(err, tc.expReleaseErr))
			}

			// Check a docker-proxy was started and stopped for each expected port binding.
			if tc.proxyPath != "" {
				expProxies := map[proxyCall]bool{}
				for _, expPB := range tc.expPBs {
					hip := expChildIP(expPB.HostIP)
					is4 := hip.To4() != nil
					if (is4 && tc.gwMode4.natDisabled()) || (!is4 && tc.gwMode6.natDisabled()) {
						continue
					}
					p := newProxyCall(expPB.Proto.String(),
						hip, int(expPB.HostPort),
						expPB.IP, int(expPB.Port), tc.proxyPath)
					expProxies[p] = tc.expReleaseErr != ""
				}
				assert.Check(t, is.DeepEqual(expProxies, proxies))
			}

			// Check the port driver has seen the expected port mappings and no others,
			// and that they have all been closed.
			if n.driver.portDriverClient != nil {
				pdc := n.driver.portDriverClient.(*mockPortDriverClient)
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
type proxyCall struct{ proto, host, container, proxyPath string }

func newProxyCall(proto string,
	hostIP net.IP, hostPort int,
	containerIP net.IP, containerPort int,
	proxyPath string,
) proxyCall {
	return proxyCall{
		proto:     proto,
		host:      fmt.Sprintf("%v:%v", hostIP, hostPort),
		container: fmt.Sprintf("%v:%v", containerIP, containerPort),
		proxyPath: proxyPath,
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

func newMockPortDriverClient(_ context.Context) (*mockPortDriverClient, error) {
	return &mockPortDriverClient{
		openPorts: map[mockPortDriverPort]bool{},
	}, nil
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
