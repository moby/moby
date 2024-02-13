//go:build linux

package overlay

import (
	"context"
	"fmt"
	"net"
	"syscall"

	"github.com/containerd/log"
	"github.com/docker/docker/libnetwork/driverapi"
	"github.com/docker/docker/libnetwork/ns"
	"github.com/docker/docker/libnetwork/osl"
	"github.com/docker/docker/libnetwork/types"
	"github.com/gogo/protobuf/proto"
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

	if n.secure && len(d.keys) == 0 {
		return fmt.Errorf("cannot join secure network: encryption keys not present")
	}

	nlh := ns.NlHandle()

	if n.secure && !nlh.SupportsNetlinkFamily(syscall.NETLINK_XFRM) {
		return fmt.Errorf("cannot join secure network: required modules to install IPSEC rules are missing on host")
	}

	s := n.getSubnetforIP(ep.addr)
	if s == nil {
		return fmt.Errorf("could not find subnet for endpoint %s", eid)
	}

	if err := n.joinSandbox(s, true); err != nil {
		return fmt.Errorf("network sandbox join failed: %v", err)
	}

	sbox := n.sandbox()

	overlayIfName, containerIfName, err := createVethPair()
	if err != nil {
		return err
	}

	ep.ifName = containerIfName

	// Set the container interface and its peer MTU to 1450 to allow
	// for 50 bytes vxlan encap (inner eth header(14) + outer IP(20) +
	// outer UDP(8) + vxlan header(8))
	mtu := n.maxMTU()

	veth, err := nlh.LinkByName(overlayIfName)
	if err != nil {
		return fmt.Errorf("cound not find link by name %s: %v", overlayIfName, err)
	}
	err = nlh.LinkSetMTU(veth, mtu)
	if err != nil {
		return err
	}

	if err = sbox.AddInterface(overlayIfName, "veth", osl.WithMaster(s.brName)); err != nil {
		return fmt.Errorf("could not add veth pair inside the network sandbox: %v", err)
	}

	veth, err = nlh.LinkByName(containerIfName)
	if err != nil {
		return fmt.Errorf("could not find link by name %s: %v", containerIfName, err)
	}
	err = nlh.LinkSetMTU(veth, mtu)
	if err != nil {
		return err
	}

	if err = nlh.LinkSetHardwareAddr(veth, ep.mac); err != nil {
		return fmt.Errorf("could not set mac address (%v) to the container interface: %v", ep.mac, err)
	}

	for _, sub := range n.subnets {
		if sub == s {
			continue
		}
		if err = jinfo.AddStaticRoute(sub.subnetIP, types.NEXTHOP, s.gwIP.IP); err != nil {
			log.G(context.TODO()).Errorf("Adding subnet %s static route in network %q failed\n", s.subnetIP, n.id)
		}
	}

	if iNames := jinfo.InterfaceName(); iNames != nil {
		err = iNames.SetNames(containerIfName, "eth")
		if err != nil {
			return err
		}
	}

	d.peerAdd(nid, eid, ep.addr.IP, ep.addr.Mask, ep.mac, d.advertiseAddress, true)

	if err = d.checkEncryption(nid, nil, true, true); err != nil {
		log.G(context.TODO()).Warn(err)
	}

	buf, err := proto.Marshal(&PeerRecord{
		EndpointIP:       ep.addr.String(),
		EndpointMAC:      ep.mac.String(),
		TunnelEndpointIP: d.advertiseAddress.String(),
	})
	if err != nil {
		return err
	}

	if err := jinfo.AddTableEntry(ovPeerTable, eid, buf); err != nil {
		log.G(context.TODO()).Errorf("overlay: Failed adding table entry to joininfo: %v", err)
	}

	return nil
}

func (d *driver) DecodeTableEntry(tablename string, key string, value []byte) (string, map[string]string) {
	if tablename != ovPeerTable {
		log.G(context.TODO()).Errorf("DecodeTableEntry: unexpected table name %s", tablename)
		return "", nil
	}

	var peer PeerRecord
	if err := proto.Unmarshal(value, &peer); err != nil {
		log.G(context.TODO()).Errorf("DecodeTableEntry: failed to unmarshal peer record for key %s: %v", key, err)
		return "", nil
	}

	return key, map[string]string{
		"Host IP": peer.TunnelEndpointIP,
	}
}

func (d *driver) EventNotify(etype driverapi.EventType, nid, tableName, key string, value []byte) {
	if tableName != ovPeerTable {
		log.G(context.TODO()).Errorf("Unexpected table notification for table %s received", tableName)
		return
	}

	eid := key

	var peer PeerRecord
	if err := proto.Unmarshal(value, &peer); err != nil {
		log.G(context.TODO()).Errorf("Failed to unmarshal peer record: %v", err)
		return
	}

	// Ignore local peers. We already know about them and they
	// should not be added to vxlan fdb.
	if net.ParseIP(peer.TunnelEndpointIP).Equal(d.advertiseAddress) {
		return
	}

	addr, err := types.ParseCIDR(peer.EndpointIP)
	if err != nil {
		log.G(context.TODO()).Errorf("Invalid peer IP %s received in event notify", peer.EndpointIP)
		return
	}

	mac, err := net.ParseMAC(peer.EndpointMAC)
	if err != nil {
		log.G(context.TODO()).Errorf("Invalid mac %s received in event notify", peer.EndpointMAC)
		return
	}

	vtep := net.ParseIP(peer.TunnelEndpointIP)
	if vtep == nil {
		log.G(context.TODO()).Errorf("Invalid VTEP %s received in event notify", peer.TunnelEndpointIP)
		return
	}

	if etype == driverapi.Delete {
		d.peerDelete(nid, eid, addr.IP, addr.Mask, mac, vtep, false)
		return
	}

	d.peerAdd(nid, eid, addr.IP, addr.Mask, mac, vtep, false)
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

	d.peerDelete(nid, eid, ep.addr.IP, ep.addr.Mask, ep.mac, d.advertiseAddress, true)

	n.leaveSandbox()

	return nil
}
