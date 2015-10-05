package overlay

import (
	"fmt"
	"net"

	"github.com/docker/libnetwork/driverapi"
	"github.com/vishvananda/netlink"
)

// Join method is invoked when a Sandbox is attached to an endpoint.
func (d *driver) Join(nid, eid string, sboxKey string, jinfo driverapi.JoinInfo, options map[string]interface{}) error {
	if err := validateID(nid, eid); err != nil {
		return err
	}

	n := d.network(nid)
	if n == nil {
		return fmt.Errorf("could not find network with id %s", nid)
	}

	ep := n.endpoint(eid)
	if ep == nil {
		return fmt.Errorf("could not find endpoint with id %s", eid)
	}

	if err := n.joinSandbox(); err != nil {
		return fmt.Errorf("network sandbox join failed: %v",
			err)
	}

	sbox := n.sandbox()

	name1, name2, err := createVethPair()
	if err != nil {
		return err
	}

	// Set the container interface and its peer MTU to 1450 to allow
	// for 50 bytes vxlan encap (inner eth header(14) + outer IP(20) +
	// outer UDP(8) + vxlan header(8))
	veth, err := netlink.LinkByName(name1)
	if err != nil {
		return fmt.Errorf("cound not find link by name %s: %v", name1, err)
	}
	err = netlink.LinkSetMTU(veth, vxlanVethMTU)
	if err != nil {
		return err
	}

	if err := sbox.AddInterface(name1, "veth",
		sbox.InterfaceOptions().Master("bridge1")); err != nil {
		return fmt.Errorf("could not add veth pair inside the network sandbox: %v", err)
	}

	veth, err = netlink.LinkByName(name2)
	if err != nil {
		return fmt.Errorf("could not find link by name %s: %v", name2, err)
	}
	err = netlink.LinkSetMTU(veth, vxlanVethMTU)
	if err != nil {
		return err
	}

	if err := netlink.LinkSetHardwareAddr(veth, ep.mac); err != nil {
		return fmt.Errorf("could not set mac address to the container interface: %v", err)
	}

	if iNames := jinfo.InterfaceName(); iNames != nil {
		err = iNames.SetNames(name2, "eth")
		if err != nil {
			return err
		}
	}

	d.peerDbAdd(nid, eid, ep.addr.IP, ep.mac,
		net.ParseIP(d.bindAddress), true)
	d.pushLocalEndpointEvent("join", nid, eid)

	return nil
}

// Leave method is invoked when a Sandbox detaches from an endpoint.
func (d *driver) Leave(nid, eid string) error {
	if err := validateID(nid, eid); err != nil {
		return err
	}

	n := d.network(nid)
	if n == nil {
		return fmt.Errorf("could not find network with id %s", nid)
	}

	d.notifyCh <- ovNotify{
		action: "leave",
		nid:    nid,
		eid:    eid,
	}

	n.leaveSandbox()

	return nil
}
