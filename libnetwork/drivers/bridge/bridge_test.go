package bridge

import (
	"bytes"
	"fmt"
	"net"
	"regexp"
	"testing"

	"github.com/docker/docker/pkg/iptables"
	"github.com/docker/libnetwork/netutils"
	"github.com/docker/libnetwork/pkg/netlabel"
	"github.com/vishvananda/netlink"
)

func TestCreateFullOptions(t *testing.T) {
	defer netutils.SetupTestNetNS(t)()
	_, d := New()

	config := &Configuration{
		EnableIPForwarding: true,
	}

	netConfig := &NetworkConfiguration{
		BridgeName:     DefaultBridgeName,
		EnableIPv6:     true,
		FixedCIDR:      bridgeNetworks[0],
		EnableIPTables: true,
	}
	_, netConfig.FixedCIDRv6, _ = net.ParseCIDR("2001:db8::/48")
	genericOption := make(map[string]interface{})
	genericOption[netlabel.GenericData] = config

	if err := d.Config(genericOption); err != nil {
		t.Fatalf("Failed to setup driver config: %v", err)
	}

	netOption := make(map[string]interface{})
	netOption[netlabel.GenericData] = netConfig

	err := d.CreateNetwork("dummy", netOption)
	if err != nil {
		t.Fatalf("Failed to create bridge: %v", err)
	}
}

func TestCreate(t *testing.T) {
	defer netutils.SetupTestNetNS(t)()
	_, d := New()

	config := &NetworkConfiguration{BridgeName: DefaultBridgeName}
	genericOption := make(map[string]interface{})
	genericOption[netlabel.GenericData] = config

	if err := d.CreateNetwork("dummy", genericOption); err != nil {
		t.Fatalf("Failed to create bridge: %v", err)
	}
}

func TestCreateFail(t *testing.T) {
	defer netutils.SetupTestNetNS(t)()
	_, d := New()

	config := &NetworkConfiguration{BridgeName: "dummy0"}
	genericOption := make(map[string]interface{})
	genericOption[netlabel.GenericData] = config

	if err := d.CreateNetwork("dummy", genericOption); err == nil {
		t.Fatal("Bridge creation was expected to fail")
	}
}

func TestQueryEndpointInfo(t *testing.T) {
	defer netutils.SetupTestNetNS(t)()

	_, d := New()

	config := &NetworkConfiguration{
		BridgeName:     DefaultBridgeName,
		EnableIPTables: true,
		EnableICC:      false,
	}
	genericOption := make(map[string]interface{})
	genericOption[netlabel.GenericData] = config

	err := d.CreateNetwork("net1", genericOption)
	if err != nil {
		t.Fatalf("Failed to create bridge: %v", err)
	}

	portMappings := getPortMapping()
	epOptions := make(map[string]interface{})
	epOptions[netlabel.PortMap] = portMappings

	_, err = d.CreateEndpoint("net1", "ep1", epOptions)
	if err != nil {
		t.Fatalf("Failed to create an endpoint : %s", err.Error())
	}

	dd := d.(*driver)
	ep, _ := dd.network.endpoints["ep1"]
	data, err := d.EndpointInfo(dd.network.id, ep.id)
	if err != nil {
		t.Fatalf("Failed to ask for endpoint operational data:  %v", err)
	}
	pmd, ok := data[netlabel.PortMap]
	if !ok {
		t.Fatalf("Endpoint operational data does not contain port mapping data")
	}
	pm, ok := pmd.([]netutils.PortBinding)
	if !ok {
		t.Fatalf("Unexpected format for port mapping in endpoint operational data")
	}
	if len(ep.portMapping) != len(pm) {
		t.Fatalf("Incomplete data for port mapping in endpoint operational data")
	}
	for i, pb := range ep.portMapping {
		if !pb.Equal(&pm[i]) {
			t.Fatalf("Unexpected data for port mapping in endpoint operational data")
		}
	}

	// Cleanup as host ports are there
	err = releasePorts(ep)
	if err != nil {
		t.Fatalf("Failed to release mapped ports: %v", err)
	}
}

func TestCreateLinkWithOptions(t *testing.T) {
	defer netutils.SetupTestNetNS(t)()

	_, d := New()

	config := &NetworkConfiguration{BridgeName: DefaultBridgeName}
	netOptions := make(map[string]interface{})
	netOptions[netlabel.GenericData] = config

	err := d.CreateNetwork("net1", netOptions)
	if err != nil {
		t.Fatalf("Failed to create bridge: %v", err)
	}

	mac := net.HardwareAddr([]byte{0x1e, 0x67, 0x66, 0x44, 0x55, 0x66})
	epOptions := make(map[string]interface{})
	epOptions[netlabel.MacAddress] = mac

	sinfo, err := d.CreateEndpoint("net1", "ep", epOptions)
	if err != nil {
		t.Fatalf("Failed to create a link: %s", err.Error())
	}

	ifaceName := sinfo.Interfaces[0].SrcName
	veth, err := netlink.LinkByName(ifaceName)
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(mac, veth.Attrs().HardwareAddr) {
		t.Fatalf("Failed to parse and program endpoint configuration")
	}
}

func getExposedPorts() []netutils.TransportPort {
	return []netutils.TransportPort{
		netutils.TransportPort{Proto: netutils.TCP, Port: uint16(5000)},
		netutils.TransportPort{Proto: netutils.UDP, Port: uint16(400)},
		netutils.TransportPort{Proto: netutils.TCP, Port: uint16(600)},
	}
}

func getPortMapping() []netutils.PortBinding {
	return []netutils.PortBinding{
		netutils.PortBinding{Proto: netutils.TCP, Port: uint16(230), HostPort: uint16(23000)},
		netutils.PortBinding{Proto: netutils.UDP, Port: uint16(200), HostPort: uint16(22000)},
		netutils.PortBinding{Proto: netutils.TCP, Port: uint16(120), HostPort: uint16(12000)},
	}
}

func TestLinkContainers(t *testing.T) {
	defer netutils.SetupTestNetNS(t)()

	_, d := New()

	config := &NetworkConfiguration{
		BridgeName:     DefaultBridgeName,
		EnableIPTables: true,
		EnableICC:      false,
	}
	genericOption := make(map[string]interface{})
	genericOption[netlabel.GenericData] = config

	err := d.CreateNetwork("net1", genericOption)
	if err != nil {
		t.Fatalf("Failed to create bridge: %v", err)
	}

	exposedPorts := getExposedPorts()
	epOptions := make(map[string]interface{})
	epOptions[netlabel.ExposedPorts] = exposedPorts

	sinfo, err := d.CreateEndpoint("net1", "ep1", epOptions)
	if err != nil {
		t.Fatalf("Failed to create an endpoint : %s", err.Error())
	}

	addr1 := sinfo.Interfaces[0].Address
	if addr1 == nil {
		t.Fatalf("No Ipv4 address assigned to the endpoint:  ep1")
	}

	sinfo, err = d.CreateEndpoint("net1", "ep2", nil)
	if err != nil {
		t.Fatalf("Failed to create an endpoint : %s", err.Error())
	}

	addr2 := sinfo.Interfaces[0].Address
	if addr2 == nil {
		t.Fatalf("No Ipv4 address assigned to the endpoint:  ep2")
	}

	ce := []string{"ep1"}
	cConfig := &ContainerConfiguration{ChildEndpoints: ce}
	genericOption = make(map[string]interface{})
	genericOption[netlabel.GenericData] = cConfig

	_, err = d.Join("net1", "ep2", "", genericOption)
	if err != nil {
		t.Fatalf("Failed to link ep1 and ep2")
	}

	out, err := iptables.Raw("-L", DockerChain)
	for _, pm := range exposedPorts {
		regex := fmt.Sprintf("%s dpt:%d", pm.Proto.String(), pm.Port)
		re := regexp.MustCompile(regex)
		matches := re.FindAllString(string(out[:]), -1)
		if len(matches) != 1 {
			t.Fatalf("IP Tables programming failed %s", string(out[:]))
		}

		regex = fmt.Sprintf("%s spt:%d", pm.Proto.String(), pm.Port)
		matched, _ := regexp.MatchString(regex, string(out[:]))
		if !matched {
			t.Fatalf("IP Tables programming failed %s", string(out[:]))
		}
	}

	err = d.Leave("net1", "ep2", genericOption)
	if err != nil {
		t.Fatalf("Failed to unlink ep1 and ep2")
	}

	out, err = iptables.Raw("-L", DockerChain)
	for _, pm := range exposedPorts {
		regex := fmt.Sprintf("%s dpt:%d", pm.Proto.String(), pm.Port)
		re := regexp.MustCompile(regex)
		matches := re.FindAllString(string(out[:]), -1)
		if len(matches) != 0 {
			t.Fatalf("Leave should have deleted relevant IPTables rules  %s", string(out[:]))
		}

		regex = fmt.Sprintf("%s spt:%d", pm.Proto.String(), pm.Port)
		matched, _ := regexp.MatchString(regex, string(out[:]))
		if matched {
			t.Fatalf("Leave should have deleted relevant IPTables rules  %s", string(out[:]))
		}
	}

	// Error condition test with an invalid endpoint-id "ep4"
	ce = []string{"ep1", "ep4"}
	cConfig = &ContainerConfiguration{ChildEndpoints: ce}
	genericOption = make(map[string]interface{})
	genericOption[netlabel.GenericData] = cConfig

	_, err = d.Join("net1", "ep2", "", genericOption)
	if err != nil {
		out, err = iptables.Raw("-L", DockerChain)
		for _, pm := range exposedPorts {
			regex := fmt.Sprintf("%s dpt:%d", pm.Proto.String(), pm.Port)
			re := regexp.MustCompile(regex)
			matches := re.FindAllString(string(out[:]), -1)
			if len(matches) != 0 {
				t.Fatalf("Error handling should rollback relevant IPTables rules  %s", string(out[:]))
			}

			regex = fmt.Sprintf("%s spt:%d", pm.Proto.String(), pm.Port)
			matched, _ := regexp.MatchString(regex, string(out[:]))
			if matched {
				t.Fatalf("Error handling should rollback relevant IPTables rules  %s", string(out[:]))
			}
		}
	} else {
		t.Fatalf("Expected Join to fail given link conditions are not satisfied")
	}
}

func TestValidateConfig(t *testing.T) {

	// Test mtu
	c := NetworkConfiguration{Mtu: -2}
	err := c.Validate()
	if err == nil {
		t.Fatalf("Failed to detect invalid MTU number")
	}

	c.Mtu = 9000
	err = c.Validate()
	if err != nil {
		t.Fatalf("unexpected validation error on MTU number")
	}

	// Bridge network
	_, network, _ := net.ParseCIDR("172.28.0.0/16")

	// Test FixedCIDR
	_, containerSubnet, _ := net.ParseCIDR("172.27.0.0/16")
	c = NetworkConfiguration{
		AddressIPv4: network,
		FixedCIDR:   containerSubnet,
	}

	err = c.Validate()
	if err == nil {
		t.Fatalf("Failed to detect invalid FixedCIDR network")
	}

	_, containerSubnet, _ = net.ParseCIDR("172.28.0.0/16")
	c.FixedCIDR = containerSubnet
	err = c.Validate()
	if err != nil {
		t.Fatalf("Unexpected validation error on FixedCIDR network")
	}

	_, containerSubnet, _ = net.ParseCIDR("172.28.0.0/15")
	c.FixedCIDR = containerSubnet
	err = c.Validate()
	if err == nil {
		t.Fatalf("Failed to detect invalid FixedCIDR network")
	}

	_, containerSubnet, _ = net.ParseCIDR("172.28.0.0/17")
	c.FixedCIDR = containerSubnet
	err = c.Validate()
	if err != nil {
		t.Fatalf("Unexpected validation error on FixedCIDR network")
	}

	// Test v4 gw
	c.DefaultGatewayIPv4 = net.ParseIP("172.27.30.234")
	err = c.Validate()
	if err == nil {
		t.Fatalf("Failed to detect invalid default gateway")
	}

	c.DefaultGatewayIPv4 = net.ParseIP("172.28.30.234")
	err = c.Validate()
	if err != nil {
		t.Fatalf("Unexpected validation error on default gateway")
	}

	// Test v6 gw
	_, containerSubnet, _ = net.ParseCIDR("2001:1234:ae:b004::/64")
	c = NetworkConfiguration{
		EnableIPv6:         true,
		FixedCIDRv6:        containerSubnet,
		DefaultGatewayIPv6: net.ParseIP("2001:1234:ac:b004::bad:a55"),
	}
	err = c.Validate()
	if err == nil {
		t.Fatalf("Failed to detect invalid v6 default gateway")
	}

	c.DefaultGatewayIPv6 = net.ParseIP("2001:1234:ae:b004::bad:a55")
	err = c.Validate()
	if err != nil {
		t.Fatalf("Unexpected validation error on v6 default gateway")
	}

	c.FixedCIDRv6 = nil
	err = c.Validate()
	if err == nil {
		t.Fatalf("Failed to detect invalid v6 default gateway")
	}
}

func TestSetDefaultGw(t *testing.T) {
	defer netutils.SetupTestNetNS(t)()
	_, d := New()

	_, subnetv6, _ := net.ParseCIDR("2001:db8:ea9:9abc:b0c4::/80")
	gw4 := bridgeNetworks[0].IP.To4()
	gw4[3] = 254
	gw6 := net.ParseIP("2001:db8:ea9:9abc:b0c4::254")

	config := &NetworkConfiguration{
		BridgeName:         DefaultBridgeName,
		EnableIPv6:         true,
		FixedCIDRv6:        subnetv6,
		DefaultGatewayIPv4: gw4,
		DefaultGatewayIPv6: gw6,
	}

	genericOption := make(map[string]interface{})
	genericOption[netlabel.GenericData] = config

	err := d.CreateNetwork("dummy", genericOption)
	if err != nil {
		t.Fatalf("Failed to create bridge: %v", err)
	}

	sinfo, err := d.CreateEndpoint("dummy", "ep", nil)
	if err != nil {
		t.Fatalf("Failed to create endpoint: %v", err)
	}

	if !gw4.Equal(sinfo.Gateway) {
		t.Fatalf("Failed to configure default gateway. Expected %v. Found %v", gw4, sinfo.Gateway)
	}

	if !gw6.Equal(sinfo.GatewayIPv6) {
		t.Fatalf("Failed to configure default gateway. Expected %v. Found %v", gw6, sinfo.GatewayIPv6)
	}
}
