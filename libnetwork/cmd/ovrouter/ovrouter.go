package main

import (
	"fmt"
	"net"
	"os"
	"os/signal"

	"github.com/docker/docker/pkg/reexec"
	"github.com/docker/libnetwork/driverapi"
	"github.com/docker/libnetwork/drivers/overlay"
	"github.com/docker/libnetwork/netlabel"
	"github.com/docker/libnetwork/types"
	"github.com/vishvananda/netlink"
)

type router struct {
	d driverapi.Driver
}

type endpoint struct {
	addr net.IPNet
	mac  net.HardwareAddr
	name string
	id   int
}

func (r *router) RegisterDriver(name string, driver driverapi.Driver, c driverapi.Capability) error {
	r.d = driver
	return nil
}

func (ep *endpoint) Interfaces() []driverapi.InterfaceInfo {
	return nil
}

func (ep *endpoint) AddInterface(ID int, mac net.HardwareAddr, ipv4 net.IPNet, ipv6 net.IPNet) error {
	ep.id = ID
	ep.addr = ipv4
	ep.mac = mac
	return nil
}

func (ep *endpoint) InterfaceNames() []driverapi.InterfaceNameInfo {
	return []driverapi.InterfaceNameInfo{ep}

}

func (ep *endpoint) SetNames(srcName, dstPrefix string) error {
	ep.name = srcName
	return nil
}

func (ep *endpoint) ID() int {
	return ep.id
}

func (ep *endpoint) SetGateway(net.IP) error {
	return nil
}

func (ep *endpoint) SetGatewayIPv6(net.IP) error {
	return nil
}

func (ep *endpoint) AddStaticRoute(destination *net.IPNet, routeType int,
	nextHop net.IP, interfaceID int) error {
	return nil
}

func (ep *endpoint) SetHostsPath(string) error {
	return nil
}

func (ep *endpoint) SetResolvConfPath(string) error {
	return nil
}

func main() {
	if reexec.Init() {
		return
	}

	r := &router{}
	if err := overlay.Init(r); err != nil {
		fmt.Printf("Failed to initialize overlay driver: %v\n", err)
		os.Exit(1)
	}

	opt := make(map[string]interface{})
	if len(os.Args) > 1 {
		opt[netlabel.OverlayBindInterface] = os.Args[1]
	}
	if len(os.Args) > 2 {
		opt[netlabel.OverlayNeighborIP] = os.Args[2]
	}
	if len(os.Args) > 3 {
		opt[netlabel.KVProvider] = os.Args[3]
	}
	if len(os.Args) > 4 {
		opt[netlabel.KVProviderURL] = os.Args[4]
	}

	r.d.Config(opt)

	if err := r.d.CreateNetwork(types.UUID("testnetwork"),
		map[string]interface{}{}); err != nil {
		fmt.Printf("Failed to create network in the driver: %v\n", err)
		os.Exit(1)
	}

	ep := &endpoint{}
	if err := r.d.CreateEndpoint(types.UUID("testnetwork"), types.UUID("testep"),
		ep, map[string]interface{}{}); err != nil {
		fmt.Printf("Failed to create endpoint in the driver: %v\n", err)
		os.Exit(1)
	}

	if err := r.d.Join(types.UUID("testnetwork"), types.UUID("testep"),
		"", ep, map[string]interface{}{}); err != nil {
		fmt.Printf("Failed to join an endpoint in the driver: %v\n", err)
		os.Exit(1)
	}

	link, err := netlink.LinkByName(ep.name)
	if err != nil {
		fmt.Printf("Failed to find the container interface with name %s: %v\n",
			ep.name, err)
		os.Exit(1)
	}

	ipAddr := &netlink.Addr{IPNet: &ep.addr, Label: ""}
	if err := netlink.AddrAdd(link, ipAddr); err != nil {
		fmt.Printf("Failed to add address to the interface: %v\n", err)
		os.Exit(1)
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, os.Kill)

	for {
		select {
		case <-sigCh:
			r.d.Leave(types.UUID("testnetwork"), types.UUID("testep"))
			overlay.Fini(r.d)
			os.Exit(0)
		}
	}
}
