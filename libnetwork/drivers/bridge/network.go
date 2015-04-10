package bridge

import (
	"errors"
	"net"
	"strings"

	"github.com/docker/libcontainer/utils"
	"github.com/docker/libnetwork"
	"github.com/vishvananda/netlink"
)

// ErrEndpointExists is returned if more than one endpoint is added to the network
var ErrEndpointExists = errors.New("Endpoint already exists (Only one endpoint allowed)")

type bridgeNetwork struct {
	NetworkName string
	bridge      *bridgeInterface
	EndPoint    *libnetwork.Interface
}

func (b *bridgeNetwork) Name() string {
	return b.NetworkName
}

func (b *bridgeNetwork) Type() string {
	return networkType
}

func (b *bridgeNetwork) Link(name string) ([]*libnetwork.Interface, error) {

	var ipv6Addr net.IPNet

	if b.EndPoint != nil {
		return nil, ErrEndpointExists
	}

	name1, err := generateIfaceName()
	if err != nil {
		return nil, err
	}

	name2, err := generateIfaceName()
	if err != nil {
		return nil, err
	}

	veth := &netlink.Veth{
		LinkAttrs: netlink.LinkAttrs{Name: name1, TxQLen: 0},
		PeerName:  name2}
	if err := netlink.LinkAdd(veth); err != nil {
		return nil, err
	}

	host, err := netlink.LinkByName(name1)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err != nil {
			netlink.LinkDel(host)
		}
	}()

	container, err := netlink.LinkByName(name2)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err != nil {
			netlink.LinkDel(container)
		}
	}()

	if err = netlink.LinkSetMaster(host,
		&netlink.Bridge{LinkAttrs: netlink.LinkAttrs{Name: b.bridge.Config.BridgeName}}); err != nil {
		return nil, err
	}

	ip4, err := ipAllocator.RequestIP(b.bridge.bridgeIPv4, nil)
	if err != nil {
		return nil, err
	}
	ipv4Addr := net.IPNet{IP: ip4, Mask: b.bridge.bridgeIPv4.Mask}

	if b.bridge.Config.EnableIPv6 {
		ip6, err := ipAllocator.RequestIP(b.bridge.bridgeIPv6, nil)
		if err != nil {
			return nil, err
		}
		ipv6Addr = net.IPNet{IP: ip6, Mask: b.bridge.bridgeIPv6.Mask}
	}

	var interfaces []*libnetwork.Interface
	intf := &libnetwork.Interface{}
	intf.SrcName = name2
	intf.DstName = "eth0"
	intf.Address = ipv4Addr.String()
	intf.Gateway = b.bridge.bridgeIPv4.IP.String()
	if b.bridge.Config.EnableIPv6 {
		intf.AddressIPv6 = ipv6Addr.String()
		intf.GatewayIPv6 = b.bridge.bridgeIPv6.IP.String()
	}

	b.EndPoint = intf
	interfaces = append(interfaces, intf)
	return interfaces, nil
}

func generateIfaceName() (string, error) {
	for i := 0; i < 10; i++ {
		name, err := utils.GenerateRandomName("veth", 7)
		if err != nil {
			continue
		}
		if _, err := net.InterfaceByName(name); err != nil {
			if strings.Contains(err.Error(), "no such") {
				return name, nil
			}
			return "", err
		}
	}
	return "", errors.New("Failed to find name for new interface")
}

func (b *bridgeNetwork) Delete() error {
	return netlink.LinkDel(b.bridge.Link)
}
