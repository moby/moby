package libnetwork_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"strings"
	"sync"
	"testing"

	"github.com/containerd/containerd/log"
	"github.com/docker/docker/libnetwork"
	"github.com/docker/docker/libnetwork/ipamapi"
	"github.com/docker/docker/libnetwork/netlabel"
	"github.com/docker/docker/libnetwork/options"
	"github.com/docker/docker/libnetwork/osl"
	"github.com/docker/docker/libnetwork/testutils"
	"github.com/docker/docker/libnetwork/types"
	"github.com/docker/docker/pkg/reexec"
	"github.com/pkg/errors"
	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netns"
	"golang.org/x/sync/errgroup"
)

const (
	bridgeNetType = "bridge"
)

func makeTesthostNetwork(t *testing.T, c *libnetwork.Controller) libnetwork.Network {
	t.Helper()
	n, err := createTestNetwork(c, "host", "testhost", options.Generic{}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	return n
}

func TestHost(t *testing.T) {
	defer testutils.SetupTestOSContext(t)()
	controller := newController(t)

	sbx1, err := controller.NewSandbox("host_c1",
		libnetwork.OptionHostname("test1"),
		libnetwork.OptionDomainname("example.com"),
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
		libnetwork.OptionDomainname("example.com"),
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

	network := makeTesthostNetwork(t, controller)
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
		libnetwork.OptionDomainname("example.com"),
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
}

// Testing IPV6 from MAC address
func TestBridgeIpv6FromMac(t *testing.T) {
	defer testutils.SetupTestOSContext(t)()
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
	defer testutils.SetupTestOSContext(t)()
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

	fsbx := &libnetwork.Sandbox{}
	if err = ep1.Join(fsbx); err == nil {
		t.Fatalf("Expected to fail join with invalid Sandbox")
	}
	if _, ok := err.(types.BadRequestError); !ok {
		t.Fatalf("Unexpected error type returned: %T", err)
	}

	sb, err := controller.NewSandbox(containerID,
		libnetwork.OptionHostname("test"),
		libnetwork.OptionDomainname("example.com"),
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
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		err = ep1.Leave(sb)
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
	defer func() {
		err = ep2.Leave(sb)
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
	defer testutils.SetupTestOSContext(t)()
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

	ep2, err := n2.CreateEndpoint("ep2")
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
		libnetwork.OptionDomainname("example.com"),
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
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		err = ep.Leave(cnt)
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
	defer func() {
		err = ep2.Leave(sbox)
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

func TestEnableIPv6(t *testing.T) {
	defer testutils.SetupTestOSContext(t)()
	controller := newController(t)

	tmpResolvConf := []byte("search pommesfrites.fr\nnameserver 12.34.56.78\nnameserver 2001:4860:4860::8888\n")
	expectedResolvConf := []byte("search pommesfrites.fr\nnameserver 127.0.0.11\nnameserver 2001:4860:4860::8888\noptions ndots:0\n")
	// take a copy of resolv.conf for restoring after test completes
	resolvConfSystem, err := os.ReadFile("/etc/resolv.conf")
	if err != nil {
		t.Fatal(err)
	}
	// cleanup
	defer func() {
		if err := os.WriteFile("/etc/resolv.conf", resolvConfSystem, 0644); err != nil {
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

	n, err := createTestNetwork(controller, "bridge", "testnetwork", netOption, nil, ipamV6ConfList)
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

	if err := os.WriteFile("/etc/resolv.conf", tmpResolvConf, 0644); err != nil {
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

	content, err := os.ReadFile(resolvConfPath)
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
	defer testutils.SetupTestOSContext(t)()
	controller := newController(t)

	tmpResolvConf := []byte("search localhost.net\nnameserver 127.0.0.1\nnameserver 2001:4860:4860::8888\n")

	// take a copy of resolv.conf for restoring after test completes
	resolvConfSystem, err := os.ReadFile("/etc/resolv.conf")
	if err != nil {
		t.Fatal(err)
	}
	// cleanup
	defer func() {
		if err := os.WriteFile("/etc/resolv.conf", resolvConfSystem, 0644); err != nil {
			t.Fatal(err)
		}
	}()

	n := makeTesthostNetwork(t, controller)
	ep1, err := n.CreateEndpoint("ep1", libnetwork.CreateOptionDisableResolution())
	if err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile("/etc/resolv.conf", tmpResolvConf, 0644); err != nil {
		t.Fatal(err)
	}

	resolvConfPath := "/tmp/libnetwork_test/resolv.conf"
	defer os.Remove(resolvConfPath)

	sb, err := controller.NewSandbox(containerID,
		libnetwork.OptionUseDefaultSandbox(),
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

	content, err := os.ReadFile(resolvConfPath)
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(content, tmpResolvConf) {
		t.Fatalf("Expected:\n%s\nGot:\n%s", string(tmpResolvConf), string(content))
	}
}

func TestResolvConf(t *testing.T) {
	defer testutils.SetupTestOSContext(t)()
	controller := newController(t)

	tmpResolvConf1 := []byte("search pommesfrites.fr\nnameserver 12.34.56.78\nnameserver 2001:4860:4860::8888\n")
	tmpResolvConf2 := []byte("search pommesfrites.fr\nnameserver 112.34.56.78\nnameserver 2001:4860:4860::8888\n")
	expectedResolvConf1 := []byte("search pommesfrites.fr\nnameserver 127.0.0.11\noptions ndots:0\n")
	tmpResolvConf3 := []byte("search pommesfrites.fr\nnameserver 113.34.56.78\n")

	// take a copy of resolv.conf for restoring after test completes
	resolvConfSystem, err := os.ReadFile("/etc/resolv.conf")
	if err != nil {
		t.Fatal(err)
	}
	// cleanup
	defer func() {
		if err := os.WriteFile("/etc/resolv.conf", resolvConfSystem, 0644); err != nil {
			t.Fatal(err)
		}
	}()

	netOption := options.Generic{
		netlabel.GenericData: options.Generic{
			"BridgeName": "testnetwork",
		},
	}
	n, err := createTestNetwork(controller, "bridge", "testnetwork", netOption, nil, nil)
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

	if err := os.WriteFile("/etc/resolv.conf", tmpResolvConf1, 0644); err != nil {
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

	content, err := os.ReadFile(resolvConfPath)
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(content, expectedResolvConf1) {
		fmt.Printf("\n%v\n%v\n", expectedResolvConf1, content)
		t.Fatalf("Expected:\n%s\nGot:\n%s", string(expectedResolvConf1), string(content))
	}

	err = ep.Leave(sb1)
	if err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile("/etc/resolv.conf", tmpResolvConf2, 0644); err != nil {
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
	if err != nil {
		t.Fatal(err)
	}

	content, err = os.ReadFile(resolvConfPath)
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(content, expectedResolvConf1) {
		t.Fatalf("Expected:\n%s\nGot:\n%s", string(expectedResolvConf1), string(content))
	}

	if err := os.WriteFile(resolvConfPath, tmpResolvConf3, 0644); err != nil {
		t.Fatal(err)
	}

	err = ep.Leave(sb2)
	if err != nil {
		t.Fatal(err)
	}

	err = ep.Join(sb2)
	if err != nil {
		t.Fatal(err)
	}

	content, err = os.ReadFile(resolvConfPath)
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(content, tmpResolvConf3) {
		t.Fatalf("Expected:\n%s\nGot:\n%s", string(tmpResolvConf3), string(content))
	}
}

type parallelTester struct {
	osctx      *testutils.OSContext
	controller *libnetwork.Controller
	net1, net2 libnetwork.Network
	iterCnt    int
}

func (pt parallelTester) Do(t *testing.T, thrNumber int) error {
	var (
		ep  *libnetwork.Endpoint
		sb  *libnetwork.Sandbox
		err error
	)

	teardown, err := pt.osctx.Set()
	if err != nil {
		return err
	}
	defer teardown(t)

	epName := fmt.Sprintf("pep%d", thrNumber)

	if thrNumber == 1 {
		ep, err = pt.net1.EndpointByName(epName)
	} else {
		ep, err = pt.net2.EndpointByName(epName)
	}

	if err != nil {
		return errors.WithStack(err)
	}
	if ep == nil {
		return errors.New("got nil ep with no error")
	}

	cid := fmt.Sprintf("%drace", thrNumber)
	pt.controller.WalkSandboxes(libnetwork.SandboxContainerWalker(&sb, cid))
	if sb == nil {
		return errors.Errorf("got nil sandbox for container: %s", cid)
	}

	for i := 0; i < pt.iterCnt; i++ {
		if err := ep.Join(sb); err != nil {
			if _, ok := err.(types.ForbiddenError); !ok {
				return errors.Wrapf(err, "thread %d", thrNumber)
			}
		}
		if err := ep.Leave(sb); err != nil {
			if _, ok := err.(types.ForbiddenError); !ok {
				return errors.Wrapf(err, "thread %d", thrNumber)
			}
		}
	}

	if err := errors.WithStack(sb.Delete()); err != nil {
		return err
	}
	return errors.WithStack(ep.Delete(false))
}

func TestParallel(t *testing.T) {
	const (
		first      = 1
		last       = 3
		numThreads = last - first + 1
		iterCnt    = 25
	)

	osctx := testutils.SetupTestOSContextEx(t)
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

	sboxes := make([]*libnetwork.Sandbox, numThreads)
	if sboxes[first-1], err = controller.NewSandbox(fmt.Sprintf("%drace", first), libnetwork.OptionUseDefaultSandbox()); err != nil {
		t.Fatal(err)
	}
	for thd := first + 1; thd <= last; thd++ {
		if sboxes[thd-1], err = controller.NewSandbox(fmt.Sprintf("%drace", thd)); err != nil {
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
	defer testutils.SetupTestOSContext(t)()
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

	ep, err := network.CreateEndpoint("testep")
	if err != nil {
		t.Fatal(err)
	}

	sb, err := controller.NewSandbox(containerID, libnetwork.OptionPortMapping(getPortMapping()))
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := sb.Delete(); err != nil {
			t.Fatal(err)
		}
	}()

	err = ep.Join(sb)
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
	defer testutils.SetupTestOSContext(t)()
	controller := newController(t)

	_, err := controller.NewNetwork(bridgeNetType, "testnetworkinternal", "", libnetwork.NetworkOptionIpam(ipamapi.NullIPAM, "", nil, nil, nil))
	if err == nil || err.Error() != "ipv4 pool is empty" {
		t.Fatal("bridge network should complain empty pool")
	}
}
