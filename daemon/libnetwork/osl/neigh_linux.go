package osl

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"strings"

	"github.com/containerd/log"
	"github.com/vishvananda/netlink"
)

// NeighborSearchError indicates that the neighbor is missing or already present.
type NeighborSearchError struct {
	IP       net.IP
	MAC      net.HardwareAddr
	LinkName string
	Present  bool
}

func (n NeighborSearchError) Error() string {
	var b strings.Builder
	b.WriteString("neighbor entry ")
	if n.Present {
		b.WriteString("already exists ")
	} else {
		b.WriteString("not found ")
	}
	b.WriteString("for IP ")
	b.WriteString(n.IP.String())
	b.WriteString(", mac ")
	b.WriteString(n.MAC.String())
	if n.LinkName != "" {
		b.WriteString(", link ")
		b.WriteString(n.LinkName)
	}
	return b.String()
}

// DeleteNeighbor deletes a neighbor entry from the sandbox.
//
// To delete an entry inserted by [AddNeighbor] the caller must provide the same
// parameters used to add it.
func (n *Namespace) DeleteNeighbor(dstIP net.IP, dstMac net.HardwareAddr, options ...NeighOption) error {
	nlnh, linkName, err := n.nlNeigh(dstIP, dstMac, options...)
	if err != nil {
		return err
	}

	if err := n.nlHandle.NeighDel(nlnh); err != nil {
		log.G(context.TODO()).WithFields(log.Fields{
			"ip":    dstIP,
			"mac":   dstMac,
			"ifc":   linkName,
			"error": err,
		}).Warn("error deleting neighbor entry")
		if errors.Is(err, os.ErrNotExist) {
			return NeighborSearchError{IP: dstIP, MAC: dstMac, LinkName: linkName}
		}
		return fmt.Errorf("could not delete neighbor %+v: %w", nlnh, err)
	}

	// Delete the dynamic entry in the bridge
	if nlnh.Family > 0 {
		nlnh.Flags = netlink.NTF_MASTER
		if err := n.nlHandle.NeighDel(nlnh); err != nil && !errors.Is(err, os.ErrNotExist) {
			log.G(context.TODO()).WithFields(log.Fields{
				"ip":    dstIP,
				"mac":   dstMac,
				"ifc":   linkName,
				"error": err,
			}).Warn("error deleting dynamic neighbor entry")
		}
	}

	log.G(context.TODO()).WithFields(log.Fields{
		"ip":  dstIP,
		"mac": dstMac,
		"ifc": linkName,
	}).Debug("Neighbor entry deleted")

	return nil
}

// AddNeighbor adds a neighbor entry into the sandbox.
//
// An FDB entry (family AF_BRIDGE) replaces any existing entry for the same
// MAC. Any other entry is only added when absent, and [NeighborSearchError]
// is returned when it is already present.
func (n *Namespace) AddNeighbor(dstIP net.IP, dstMac net.HardwareAddr, options ...NeighOption) error {
	nlnh, linkName, err := n.nlNeigh(dstIP, dstMac, options...)
	if err != nil {
		return err
	}

	if nlnh.Family > 0 {
		// The VXLAN device learns FDB entries from inbound traffic, so the
		// kernel may already hold a dynamic entry for this MAC by the time
		// the permanent one is (re)added, e.g. after a peer node failed and
		// rejoined the cluster while its containers kept sending. Replace
		// it rather than fail: the learned entry ages out, the permanent
		// one must not.
		err = n.nlHandle.NeighSet(nlnh)
	} else {
		err = n.nlHandle.NeighAdd(nlnh)
	}
	if err != nil {
		if errors.Is(err, os.ErrExist) {
			log.G(context.TODO()).WithFields(log.Fields{
				"ip":    dstIP,
				"mac":   dstMac,
				"ifc":   linkName,
				"neigh": fmt.Sprintf("%+v", nlnh),
			}).Warn("Neighbor entry already present")
			return NeighborSearchError{IP: dstIP, MAC: dstMac, LinkName: linkName, Present: true}
		} else {
			return fmt.Errorf("could not add neighbor entry %+v: %w", nlnh, err)
		}
	}

	log.G(context.TODO()).WithFields(log.Fields{
		"ip":  dstIP,
		"mac": dstMac,
		"ifc": linkName,
	}).Debug("Neighbor entry added")

	return nil
}

type neigh struct {
	linkName string
	family   int
}

func (n *Namespace) nlNeigh(dstIP net.IP, dstMac net.HardwareAddr, options ...NeighOption) (*netlink.Neigh, string, error) {
	var nh neigh
	nh.processNeighOptions(options...)

	nlnh := &netlink.Neigh{
		IP:           dstIP,
		HardwareAddr: dstMac,
		State:        netlink.NUD_PERMANENT,
		Family:       nh.family,
	}

	if nlnh.Family > 0 {
		nlnh.Flags = netlink.NTF_SELF
	}

	if nh.linkName != "" {
		linkDst := n.findDst(nh.linkName, false)
		if linkDst == "" {
			return nil, nh.linkName, fmt.Errorf("could not find the interface with name %s", nh.linkName)
		}
		iface, err := n.nlHandle.LinkByName(linkDst)
		if err != nil {
			return nil, nh.linkName, fmt.Errorf("could not find interface with destination name %s: %w", linkDst, err)
		}
		nlnh.LinkIndex = iface.Attrs().Index
	}

	return nlnh, nh.linkName, nil
}
