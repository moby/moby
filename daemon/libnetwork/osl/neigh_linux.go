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
			return NeighborSearchError{dstIP, dstMac, linkName, false}
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
func (n *Namespace) AddNeighbor(dstIP net.IP, dstMac net.HardwareAddr, options ...NeighOption) error {
	nlnh, linkName, err := n.nlNeigh(dstIP, dstMac, options...)
	if err != nil {
		return err
	}

	if err := n.nlHandle.NeighAdd(nlnh); err != nil {
		if errors.Is(err, os.ErrExist) {
			log.G(context.TODO()).WithFields(log.Fields{
				"ip":    dstIP,
				"mac":   dstMac,
				"ifc":   linkName,
				"neigh": fmt.Sprintf("%+v", nlnh),
			}).Warn("Neighbor entry already present")
			return NeighborSearchError{dstIP, dstMac, linkName, true}
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

// SetNeighbor adds a neighbor entry into the sandbox or replaces an existing one.
func (n *Namespace) SetNeighbor(dstIP net.IP, dstMac net.HardwareAddr, options ...NeighOption) error {
	nlnh, linkName, err := n.nlNeigh(dstIP, dstMac, options...)
	if err != nil {
		return err
	}

	if err := n.nlHandle.NeighSet(nlnh); err != nil {
		return fmt.Errorf("could not set neighbor entry %+v: %w", nlnh, err)
	}

	log.G(context.TODO()).WithFields(log.Fields{
		"ip":  dstIP,
		"mac": dstMac,
		"ifc": linkName,
	}).Debug("Neighbor entry set")

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
