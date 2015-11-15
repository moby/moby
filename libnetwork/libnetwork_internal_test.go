package libnetwork

import (
	"encoding/json"
	"fmt"
	"net"
	"testing"

	"github.com/docker/libnetwork/datastore"
	"github.com/docker/libnetwork/driverapi"
	"github.com/docker/libnetwork/ipamapi"
	"github.com/docker/libnetwork/netlabel"
	"github.com/docker/libnetwork/testutils"
	"github.com/docker/libnetwork/types"
)

func TestDriverRegistration(t *testing.T) {
	bridgeNetType := "bridge"
	c, err := New()
	if err != nil {
		t.Fatal(err)
	}
	defer c.Stop()
	err = c.(*controller).RegisterDriver(bridgeNetType, nil, driverapi.Capability{})
	if err == nil {
		t.Fatalf("Expecting the RegisterDriver to fail for %s", bridgeNetType)
	}
	if _, ok := err.(driverapi.ErrActiveRegistration); !ok {
		t.Fatalf("Failed for unexpected reason: %v", err)
	}
	err = c.(*controller).RegisterDriver("test-dummy", nil, driverapi.Capability{})
	if err != nil {
		t.Fatalf("Test failed with an error %v", err)
	}
}

func TestNetworkMarshalling(t *testing.T) {
	n := &network{
		name:        "Miao",
		id:          "abccba",
		ipamType:    "default",
		addrSpace:   "viola",
		networkType: "bridge",
		enableIPv6:  true,
		persist:     true,
		ipamV4Config: []*IpamConf{
			&IpamConf{
				PreferredPool: "10.2.0.0/16",
				SubPool:       "10.2.0.0/24",
				Options: map[string]string{
					netlabel.MacAddress: "a:b:c:d:e:f",
				},
				Gateway:      "",
				AuxAddresses: nil,
			},
			&IpamConf{
				PreferredPool: "10.2.0.0/16",
				SubPool:       "10.2.1.0/24",
				Options:       nil,
				Gateway:       "10.2.1.254",
			},
		},
		ipamV6Config: []*IpamConf{
			&IpamConf{
				PreferredPool: "abcd::/64",
				SubPool:       "abcd:abcd:abcd:abcd:abcd::/80",
				Gateway:       "abcd::29/64",
				AuxAddresses:  nil,
			},
		},
		ipamV4Info: []*IpamInfo{
			&IpamInfo{
				PoolID: "ipoolverde123",
				Meta: map[string]string{
					netlabel.Gateway: "10.2.1.255/16",
				},
				IPAMData: driverapi.IPAMData{
					AddressSpace: "viola",
					Pool: &net.IPNet{
						IP:   net.IP{10, 2, 0, 0},
						Mask: net.IPMask{255, 255, 255, 0},
					},
					Gateway:      nil,
					AuxAddresses: nil,
				},
			},
			&IpamInfo{
				PoolID: "ipoolblue345",
				Meta: map[string]string{
					netlabel.Gateway: "10.2.1.255/16",
				},
				IPAMData: driverapi.IPAMData{
					AddressSpace: "viola",
					Pool: &net.IPNet{
						IP:   net.IP{10, 2, 1, 0},
						Mask: net.IPMask{255, 255, 255, 0},
					},
					Gateway: &net.IPNet{IP: net.IP{10, 2, 1, 254}, Mask: net.IPMask{255, 255, 255, 0}},
					AuxAddresses: map[string]*net.IPNet{
						"ip3": &net.IPNet{IP: net.IP{10, 2, 1, 3}, Mask: net.IPMask{255, 255, 255, 0}},
						"ip5": &net.IPNet{IP: net.IP{10, 2, 1, 55}, Mask: net.IPMask{255, 255, 255, 0}},
					},
				},
			},
			&IpamInfo{
				PoolID: "weirdinfo",
				IPAMData: driverapi.IPAMData{
					Gateway: &net.IPNet{
						IP:   net.IP{11, 2, 1, 255},
						Mask: net.IPMask{255, 0, 0, 0},
					},
				},
			},
		},
		ipamV6Info: []*IpamInfo{
			&IpamInfo{
				PoolID: "ipoolv6",
				IPAMData: driverapi.IPAMData{
					AddressSpace: "viola",
					Pool: &net.IPNet{
						IP:   net.IP{0xab, 0xcd, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
						Mask: net.IPMask{255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 0, 0, 0, 0, 0, 0},
					},
					Gateway: &net.IPNet{
						IP:   net.IP{0xab, 0xcd, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 29},
						Mask: net.IPMask{255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 0, 0, 0, 0, 0, 0},
					},
					AuxAddresses: nil,
				},
			},
		},
	}

	b, err := json.Marshal(n)
	if err != nil {
		t.Fatal(err)
	}

	nn := &network{}
	err = json.Unmarshal(b, nn)
	if err != nil {
		t.Fatal(err)
	}

	if n.name != nn.name || n.id != nn.id || n.networkType != nn.networkType || n.ipamType != nn.ipamType ||
		n.addrSpace != nn.addrSpace || n.enableIPv6 != nn.enableIPv6 ||
		n.persist != nn.persist || !compareIpamConfList(n.ipamV4Config, nn.ipamV4Config) ||
		!compareIpamInfoList(n.ipamV4Info, nn.ipamV4Info) || !compareIpamConfList(n.ipamV6Config, nn.ipamV6Config) ||
		!compareIpamInfoList(n.ipamV6Info, nn.ipamV6Info) {
		t.Fatalf("JSON marsh/unmarsh failed."+
			"\nOriginal:\n%#v\nDecoded:\n%#v"+
			"\nOriginal ipamV4Conf: %#v\n\nDecoded ipamV4Conf: %#v"+
			"\nOriginal ipamV4Info: %s\n\nDecoded ipamV4Info: %s"+
			"\nOriginal ipamV6Conf: %#v\n\nDecoded ipamV6Conf: %#v"+
			"\nOriginal ipamV6Info: %s\n\nDecoded ipamV6Info: %s",
			n, nn, printIpamConf(n.ipamV4Config), printIpamConf(nn.ipamV4Config),
			printIpamInfo(n.ipamV4Info), printIpamInfo(nn.ipamV4Info),
			printIpamConf(n.ipamV6Config), printIpamConf(nn.ipamV6Config),
			printIpamInfo(n.ipamV6Info), printIpamInfo(nn.ipamV6Info))
	}
}

func printIpamConf(list []*IpamConf) string {
	s := fmt.Sprintf("\n[]*IpamConfig{")
	for _, i := range list {
		s = fmt.Sprintf("%s %v,", s, i)
	}
	s = fmt.Sprintf("%s}", s)
	return s
}

func printIpamInfo(list []*IpamInfo) string {
	s := fmt.Sprintf("\n[]*IpamInfo{")
	for _, i := range list {
		s = fmt.Sprintf("%s\n{\n%s\n}", s, i)
	}
	s = fmt.Sprintf("%s\n}", s)
	return s
}

func TestEndpointMarshalling(t *testing.T) {
	ip, nw6, err := net.ParseCIDR("2001:db8:4003::122/64")
	if err != nil {
		t.Fatal(err)
	}
	nw6.IP = ip

	e := &endpoint{
		name:      "Bau",
		id:        "efghijklmno",
		sandboxID: "ambarabaciccicocco",
		anonymous: true,
		iface: &endpointInterface{
			mac: []byte{11, 12, 13, 14, 15, 16},
			addr: &net.IPNet{
				IP:   net.IP{10, 0, 1, 23},
				Mask: net.IPMask{255, 255, 255, 0},
			},
			addrv6:    nw6,
			srcName:   "veth12ab1314",
			dstPrefix: "eth",
			v4PoolID:  "poolpool",
			v6PoolID:  "poolv6",
		},
	}

	b, err := json.Marshal(e)
	if err != nil {
		t.Fatal(err)
	}

	ee := &endpoint{}
	err = json.Unmarshal(b, ee)
	if err != nil {
		t.Fatal(err)
	}

	if e.name != ee.name || e.id != ee.id || e.sandboxID != ee.sandboxID || !compareEndpointInterface(e.iface, ee.iface) || e.anonymous != ee.anonymous {
		t.Fatalf("JSON marsh/unmarsh failed.\nOriginal:\n%#v\nDecoded:\n%#v\nOriginal iface: %#v\nDecodediface:\n%#v", e, ee, e.iface, ee.iface)
	}
}

func compareEndpointInterface(a, b *endpointInterface) bool {
	if a == b {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return a.srcName == b.srcName && a.dstPrefix == b.dstPrefix && a.v4PoolID == b.v4PoolID && a.v6PoolID == b.v6PoolID &&
		types.CompareIPNet(a.addr, b.addr) && types.CompareIPNet(a.addrv6, b.addrv6)
}

func compareIpamConfList(listA, listB []*IpamConf) bool {
	var a, b *IpamConf
	if len(listA) != len(listB) {
		return false
	}
	for i := 0; i < len(listA); i++ {
		a = listA[i]
		b = listB[i]
		if a.PreferredPool != b.PreferredPool ||
			a.SubPool != b.SubPool ||
			!compareStringMaps(a.Options, b.Options) ||
			a.Gateway != b.Gateway || !compareStringMaps(a.AuxAddresses, b.AuxAddresses) {
			return false
		}
	}
	return true
}

func compareIpamInfoList(listA, listB []*IpamInfo) bool {
	var a, b *IpamInfo
	if len(listA) != len(listB) {
		return false
	}
	for i := 0; i < len(listA); i++ {
		a = listA[i]
		b = listB[i]
		if a.PoolID != b.PoolID || !compareStringMaps(a.Meta, b.Meta) ||
			!types.CompareIPNet(a.Gateway, b.Gateway) ||
			a.AddressSpace != b.AddressSpace ||
			!types.CompareIPNet(a.Pool, b.Pool) ||
			!compareAddresses(a.AuxAddresses, b.AuxAddresses) {
			return false
		}
	}
	return true
}

func compareStringMaps(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	if len(a) > 0 {
		for k := range a {
			if a[k] != b[k] {
				return false
			}
		}
	}
	return true
}

func compareAddresses(a, b map[string]*net.IPNet) bool {
	if len(a) != len(b) {
		return false
	}
	if len(a) > 0 {
		for k := range a {
			if !types.CompareIPNet(a[k], b[k]) {
				return false
			}
		}
	}
	return true
}

func TestAuxAddresses(t *testing.T) {
	c, err := New()
	if err != nil {
		t.Fatal(err)
	}
	defer c.Stop()

	n := &network{ipamType: ipamapi.DefaultIPAM, networkType: "bridge", ctrlr: c.(*controller)}

	input := []struct {
		masterPool   string
		subPool      string
		auxAddresses map[string]string
		good         bool
	}{
		{"192.168.0.0/16", "", map[string]string{"goodOne": "192.168.2.2"}, true},
		{"192.168.0.0/16", "", map[string]string{"badOne": "192.169.2.3"}, false},
		{"192.168.0.0/16", "192.168.1.0/24", map[string]string{"goodOne": "192.168.1.2"}, true},
		{"192.168.0.0/16", "192.168.1.0/24", map[string]string{"stillGood": "192.168.2.4"}, true},
		{"192.168.0.0/16", "192.168.1.0/24", map[string]string{"badOne": "192.169.2.4"}, false},
	}

	for _, i := range input {

		n.ipamV4Config = []*IpamConf{&IpamConf{PreferredPool: i.masterPool, SubPool: i.subPool, AuxAddresses: i.auxAddresses}}

		err = n.ipamAllocate()

		if i.good != (err == nil) {
			t.Fatalf("Unexpected result for %v: %v", i, err)
		}

		n.ipamRelease()
	}
}

func TestIpamReleaseOnNetDriverFailures(t *testing.T) {
	if !testutils.IsRunningInContainer() {
		defer testutils.SetupTestOSContext(t)()
	}

	cfgOptions, err := OptionBoltdbWithRandomDBFile()
	c, err := New(cfgOptions...)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Stop()

	cc := c.(*controller)
	bd := badDriver{failNetworkCreation: true}
	cc.drivers[badDriverName] = &driverData{driver: &bd, capability: driverapi.Capability{DataScope: datastore.LocalScope}}

	// Test whether ipam state release is invoked  on network create failure from net driver
	// by checking whether subsequent network creation requesting same gateway IP succeeds
	ipamOpt := NetworkOptionIpam(ipamapi.DefaultIPAM, "", []*IpamConf{&IpamConf{PreferredPool: "10.34.0.0/16", Gateway: "10.34.255.254"}}, nil)
	if _, err := c.NewNetwork(badDriverName, "badnet1", ipamOpt); err == nil {
		t.Fatalf("bad network driver should have failed network creation")
	}

	gnw, err := c.NewNetwork("bridge", "goodnet1", ipamOpt)
	if err != nil {
		t.Fatal(err)
	}
	gnw.Delete()

	// Now check whether ipam release works on endpoint creation failure
	bd.failNetworkCreation = false
	bnw, err := c.NewNetwork(badDriverName, "badnet2", ipamOpt)
	if err != nil {
		t.Fatal(err)
	}
	defer bnw.Delete()

	if _, err := bnw.CreateEndpoint("ep0"); err == nil {
		t.Fatalf("bad network driver should have failed endpoint creation")
	}

	// Now create good bridge network with different gateway
	ipamOpt2 := NetworkOptionIpam(ipamapi.DefaultIPAM, "", []*IpamConf{&IpamConf{PreferredPool: "10.34.0.0/16", Gateway: "10.34.255.253"}}, nil)
	gnw, err = c.NewNetwork("bridge", "goodnet2", ipamOpt2)
	if err != nil {
		t.Fatal(err)
	}
	defer gnw.Delete()

	ep, err := gnw.CreateEndpoint("ep1")
	if err != nil {
		t.Fatal(err)
	}
	defer ep.Delete()

	expectedIP, _ := types.ParseCIDR("10.34.0.1/16")
	if !types.CompareIPNet(ep.Info().Iface().Address(), expectedIP) {
		t.Fatalf("Ipam release must have failed, endpoint has unexpected address: %v", ep.Info().Iface().Address())
	}
}

var badDriverName = "bad network driver"

type badDriver struct {
	failNetworkCreation bool
}

func (b *badDriver) CreateNetwork(nid string, options map[string]interface{}, ipV4Data, ipV6Data []driverapi.IPAMData) error {
	if b.failNetworkCreation {
		return fmt.Errorf("I will not create any network")
	}
	return nil
}
func (b *badDriver) DeleteNetwork(nid string) error {
	return nil
}
func (b *badDriver) CreateEndpoint(nid, eid string, ifInfo driverapi.InterfaceInfo, options map[string]interface{}) error {
	return fmt.Errorf("I will not create any endpoint")
}
func (b *badDriver) DeleteEndpoint(nid, eid string) error {
	return nil
}
func (b *badDriver) EndpointOperInfo(nid, eid string) (map[string]interface{}, error) {
	return nil, nil
}
func (b *badDriver) Join(nid, eid string, sboxKey string, jinfo driverapi.JoinInfo, options map[string]interface{}) error {
	return fmt.Errorf("I will not allow any join")
}
func (b *badDriver) Leave(nid, eid string) error {
	return nil
}
func (b *badDriver) DiscoverNew(dType driverapi.DiscoveryType, data interface{}) error {
	return nil
}
func (b *badDriver) DiscoverDelete(dType driverapi.DiscoveryType, data interface{}) error {
	return nil
}
func (b *badDriver) Type() string {
	return badDriverName
}
