package overlay

import (
	"fmt"
	"net"
	"strings"

	log "github.com/Sirupsen/logrus"
	"github.com/docker/libnetwork/driverapi"
	"github.com/docker/libnetwork/types"
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

	s := n.getSubnetforIP(ep.addr)
	if s == nil {
		return fmt.Errorf("could not find subnet for endpoint %s", eid)
	}

	if err := n.obtainVxlanID(s); err != nil {
		return fmt.Errorf("couldn't get vxlan id for %q: %v", s.subnetIP.String(), err)
	}

	if err := n.joinSandbox(); err != nil {
		return fmt.Errorf("network sandbox join failed: %v", err)
	}

	if err := n.joinSubnetSandbox(s); err != nil {
		return fmt.Errorf("subnet sandbox join failed for %q: %v", s.subnetIP.String(), err)
	}

	// joinSubnetSandbox gets called when an endpoint comes up on a new subnet in the
	// overlay network. Hence the Endpoint count should be updated outside joinSubnetSandbox
	n.incEndpointCount()

	sbox := n.sandbox()

	overlayIfName, containerIfName, err := createVethPair()
	if err != nil {
		return err
	}

	ep.ifName = containerIfName

	// Set the container interface and its peer MTU to 1450 to allow
	// for 50 bytes vxlan encap (inner eth header(14) + outer IP(20) +
	// outer UDP(8) + vxlan header(8))
	veth, err := netlink.LinkByName(overlayIfName)
	if err != nil {
		return fmt.Errorf("cound not find link by name %s: %v", overlayIfName, err)
	}
	err = netlink.LinkSetMTU(veth, vxlanVethMTU)
	if err != nil {
		return err
	}

	if err := sbox.AddInterface(overlayIfName, "veth",
		sbox.InterfaceOptions().Master(s.brName)); err != nil {
		return fmt.Errorf("could not add veth pair inside the network sandbox: %v", err)
	}

	veth, err = netlink.LinkByName(containerIfName)
	if err != nil {
		return fmt.Errorf("could not find link by name %s: %v", containerIfName, err)
	}
	err = netlink.LinkSetMTU(veth, vxlanVethMTU)
	if err != nil {
		return err
	}

	if err := netlink.LinkSetHardwareAddr(veth, ep.mac); err != nil {
		return fmt.Errorf("could not set mac address (%v) to the container interface: %v", ep.mac, err)
	}

	for _, sub := range n.subnets {
		if sub == s {
			continue
		}
		if err := jinfo.AddStaticRoute(sub.subnetIP, types.NEXTHOP, s.gwIP.IP); err != nil {
			log.Errorf("Adding subnet %s static route in network %q failed\n", s.subnetIP, n.id)
		}
	}

	if iNames := jinfo.InterfaceName(); iNames != nil {
		err = iNames.SetNames(containerIfName, "eth")
		if err != nil {
			return err
		}
	}

	d.peerDbAdd(nid, eid, ep.addr.IP, ep.addr.Mask, ep.mac,
		net.ParseIP(d.bindAddress), true)

	if err := jinfo.AddTableEntry(ovPeerTable, eid, []byte(fmt.Sprintf("%s,%s,%s", ep.addr, ep.mac, d.bindAddress))); err != nil {
		log.Errorf("overlay: Failed adding table entry to joininfo: %v", err)
	}

	d.pushLocalEndpointEvent("join", nid, eid)

	return nil
}

func (d *driver) EventNotify(etype driverapi.EventType, nid, tableName, key string, value []byte) {
	if tableName != ovPeerTable {
		log.Errorf("Unexpected table notification for table %s received", tableName)
		return
	}

	eid := key
	values := strings.Split(string(value), ",")
	if len(values) < 3 {
		log.Errorf("Invalid value %s received through event notify", string(value))
		return
	}

	addr, err := types.ParseCIDR(values[0])
	if err != nil {
		log.Errorf("Invalid peer IP %s received in event notify", values[0])
		return
	}

	mac, err := net.ParseMAC(values[1])
	if err != nil {
		log.Errorf("Invalid mac %s received in event notify", values[1])
		return
	}

	vtep := net.ParseIP(values[2])
	if vtep == nil {
		log.Errorf("Invalid VTEP %s received in event notify", values[2])
		return
	}

	if etype == driverapi.Delete {
		d.peerDelete(nid, eid, addr.IP, addr.Mask, mac, vtep, true)
		return
	}

	d.peerAdd(nid, eid, addr.IP, addr.Mask, mac, vtep, true)
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

	ep := n.endpoint(eid)

	if ep == nil {
		return types.InternalMaskableErrorf("could not find endpoint with id %s", eid)
	}

	if d.notifyCh != nil {
		d.notifyCh <- ovNotify{
			action: "leave",
			nw:     n,
			ep:     ep,
		}
	}

	n.leaveSandbox()

	return nil
}
