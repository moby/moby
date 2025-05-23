//go:build linux

package overlay

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"syscall"

	"github.com/containerd/log"
	"github.com/docker/docker/libnetwork/driverapi"
	"github.com/docker/docker/libnetwork/internal/netiputil"
	"github.com/docker/docker/libnetwork/netlabel"
	"github.com/docker/docker/libnetwork/ns"
	"github.com/docker/docker/libnetwork/osl"
	"github.com/docker/docker/libnetwork/types"
	"github.com/gogo/protobuf/proto"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// Join method is invoked when a Sandbox is attached to an endpoint.
func (d *driver) Join(ctx context.Context, nid, eid string, sboxKey string, jinfo driverapi.JoinInfo, epOpts, _ map[string]interface{}) error {
	ctx, span := otel.Tracer("").Start(ctx, "libnetwork.drivers.overlay.Join", trace.WithAttributes(
		attribute.String("nid", nid),
		attribute.String("eid", eid),
		attribute.String("sboxKey", sboxKey)))
	defer span.End()

	if err := validateID(nid, eid); err != nil {
		return err
	}

	n, unlock, err := d.lockNetwork(nid)
	if err != nil {
		return err
	}
	defer unlock()

	ep := n.endpoints[eid]
	if ep == nil {
		return fmt.Errorf("could not find endpoint with id %s", eid)
	}

	if n.secure {
		d.encrMu.Lock()
		nkeys := len(d.keys)
		d.encrMu.Unlock()
		if nkeys == 0 {
			return errors.New("cannot join secure network: encryption keys not present")
		}
	}

	nlh := ns.NlHandle()

	if n.secure && !nlh.SupportsNetlinkFamily(syscall.NETLINK_XFRM) {
		return errors.New("cannot join secure network: required modules to install IPSEC rules are missing on host")
	}

	s := n.getSubnetforIP(ep.addr)
	if s == nil {
		return fmt.Errorf("could not find subnet for endpoint %s", eid)
	}

	if err := n.joinSandbox(s, true); err != nil {
		return fmt.Errorf("network sandbox join failed: %v", err)
	}

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
		return fmt.Errorf("could not find link by name %s: %v", overlayIfName, err)
	}
	err = nlh.LinkSetMTU(veth, mtu)
	if err != nil {
		return err
	}

	if err = n.sbox.AddInterface(ctx, overlayIfName, "veth", "", osl.WithMaster(s.brName)); err != nil {
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
		if err = jinfo.AddStaticRoute(netiputil.ToIPNet(sub.subnetIP), types.NEXTHOP, s.gwIP.Addr().AsSlice()); err != nil {
			log.G(ctx).Errorf("Adding subnet %s static route in network %q failed\n", s.subnetIP, n.id)
		}
	}

	if iNames := jinfo.InterfaceName(); iNames != nil {
		err = iNames.SetNames(containerIfName, "eth", netlabel.GetIfname(epOpts))
		if err != nil {
			return err
		}
	}

	if err := n.peerAdd(eid, ep.addr, ep.mac, netip.Addr{}); err != nil {
		return fmt.Errorf("overlay: failed to add local endpoint to network peer db: %w", err)
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
		log.G(ctx).Errorf("overlay: Failed adding table entry to joininfo: %v", err)
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
	if addr, _ := netip.ParseAddr(peer.TunnelEndpointIP); addr == d.advertiseAddress {
		return
	}

	addr, err := netip.ParsePrefix(peer.EndpointIP)
	if err != nil {
		log.G(context.TODO()).WithError(err).Errorf("Invalid peer IP %s received in event notify", peer.EndpointIP)
		return
	}

	mac, err := net.ParseMAC(peer.EndpointMAC)
	if err != nil {
		log.G(context.TODO()).WithError(err).Errorf("Invalid mac %s received in event notify", peer.EndpointMAC)
		return
	}

	vtep, err := netip.ParseAddr(peer.TunnelEndpointIP)
	if err != nil {
		log.G(context.TODO()).WithError(err).Errorf("Invalid VTEP %s received in event notify", peer.TunnelEndpointIP)
		return
	}

	n, unlock, err := d.lockNetwork(nid)
	if err != nil {
		log.G(context.TODO()).WithFields(log.Fields{
			"error": err,
			"nid":   nid,
		}).Error("overlay: handling peer event")
		return
	}
	defer unlock()

	var opname string
	if etype == driverapi.Delete {
		opname = "delete"
		err = n.peerDelete(eid, addr, mac, vtep)
	} else {
		opname = "add"
		err = n.peerAdd(eid, addr, mac, vtep)
	}
	if err != nil {
		log.G(context.TODO()).WithFields(log.Fields{
			"error": err,
			"nid":   n.id,
			"peer":  peer,
			"op":    opname,
		}).Warn("Peer operation failed")
	}
}

// Leave method is invoked when a Sandbox detaches from an endpoint.
func (d *driver) Leave(nid, eid string) error {
	if err := validateID(nid, eid); err != nil {
		return err
	}

	n, unlock, err := d.lockNetwork(nid)
	if err != nil {
		return err
	}
	defer unlock()

	ep := n.endpoints[eid]

	if ep == nil {
		return types.InternalMaskableErrorf("could not find endpoint with id %s", eid)
	}

	if err := n.peerDelete(eid, ep.addr, ep.mac, netip.Addr{}); err != nil {
		return fmt.Errorf("overlay: failed to delete local endpoint eid:%s from network peer db: %w", eid, err)
	}

	n.leaveSandbox()

	return nil
}
