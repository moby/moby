package osl

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net"
	"os"

	"github.com/containerd/log"
	"github.com/vishvananda/netlink"
)

// NeighborSearchError indicates that the neighbor is already present
type NeighborSearchError struct {
	ip      net.IP
	mac     net.HardwareAddr
	present bool
}

func (n NeighborSearchError) Error() string {
	return fmt.Sprintf("Search neighbor failed for IP %v, mac %v, present in db:%t", n.ip, n.mac, n.present)
}

type neigh struct {
	dstIP    net.IP
	dstMac   net.HardwareAddr
	linkName string
	linkDst  string
	family   int
}

func (n *Namespace) findNeighbor(dstIP net.IP, dstMac net.HardwareAddr) *neigh {
	n.mu.Lock()
	defer n.mu.Unlock()

	for _, nh := range n.neighbors {
		if nh.dstIP.Equal(dstIP) && bytes.Equal(nh.dstMac, dstMac) {
			return nh
		}
	}

	return nil
}

// DeleteNeighbor deletes neighbor entry from the sandbox.
func (n *Namespace) DeleteNeighbor(dstIP net.IP, dstMac net.HardwareAddr) error {
	nh := n.findNeighbor(dstIP, dstMac)
	if nh == nil {
		return NeighborSearchError{dstIP, dstMac, false}
	}

	n.mu.Lock()
	nlh := n.nlHandle
	n.mu.Unlock()

	var linkIndex int
	if nh.linkDst != "" {
		iface, err := nlh.LinkByName(nh.linkDst)
		if err != nil {
			return fmt.Errorf("could not find interface with destination name %s: %v", nh.linkDst, err)
		}
		linkIndex = iface.Attrs().Index
	}

	nlnh := &netlink.Neigh{
		LinkIndex: linkIndex,
		IP:        dstIP,
		State:     netlink.NUD_PERMANENT,
		Family:    nh.family,
	}

	if nh.family > 0 {
		nlnh.HardwareAddr = dstMac
		nlnh.Flags = netlink.NTF_SELF
	}

	// If the kernel deletion fails for the neighbor entry still remove it
	// from the namespace cache, otherwise kernel update can fail if the
	// neighbor moves back to the same host again.
	if err := nlh.NeighDel(nlnh); err != nil && !errors.Is(err, os.ErrNotExist) {
		log.G(context.TODO()).Warnf("Deleting neighbor IP %s, mac %s failed, %v", dstIP, dstMac, err)
	}

	// Delete the dynamic entry in the bridge
	if nh.family > 0 {
		if err := nlh.NeighDel(&netlink.Neigh{
			LinkIndex:    linkIndex,
			IP:           dstIP,
			Family:       nh.family,
			HardwareAddr: dstMac,
			Flags:        netlink.NTF_MASTER,
		}); err != nil && !errors.Is(err, os.ErrNotExist) {
			log.G(context.TODO()).WithError(err).Warn("error while deleting neighbor entry")
		}
	}

	n.mu.Lock()
	for i, neighbor := range n.neighbors {
		if neighbor.dstIP.Equal(dstIP) && bytes.Equal(neighbor.dstMac, dstMac) {
			n.neighbors = append(n.neighbors[:i], n.neighbors[i+1:]...)
			break
		}
	}
	n.mu.Unlock()
	log.G(context.TODO()).Debugf("Neighbor entry deleted for IP %v, mac %v", dstIP, dstMac)

	return nil
}

// AddNeighbor adds a neighbor entry into the sandbox.
func (n *Namespace) AddNeighbor(dstIP net.IP, dstMac net.HardwareAddr, force bool, options ...NeighOption) error {
	var (
		iface                  netlink.Link
		err                    error
		neighborAlreadyPresent bool
	)

	// If the namespace already has the neighbor entry but the AddNeighbor is called
	// because of a miss notification (force flag) program the kernel anyway.
	nh := n.findNeighbor(dstIP, dstMac)
	if nh != nil {
		neighborAlreadyPresent = true
		log.G(context.TODO()).Warnf("Neighbor entry already present for IP %v, mac %v neighbor:%+v forceUpdate:%t", dstIP, dstMac, nh, force)
		if !force {
			return NeighborSearchError{dstIP, dstMac, true}
		}
	}

	nh = &neigh{
		dstIP:  dstIP,
		dstMac: dstMac,
	}

	nh.processNeighOptions(options...)

	if nh.linkName != "" {
		nh.linkDst = n.findDst(nh.linkName, false)
		if nh.linkDst == "" {
			return fmt.Errorf("could not find the interface with name %s", nh.linkName)
		}
	}

	n.mu.Lock()
	nlh := n.nlHandle
	n.mu.Unlock()

	if nh.linkDst != "" {
		iface, err = nlh.LinkByName(nh.linkDst)
		if err != nil {
			return fmt.Errorf("could not find interface with destination name %s: %v", nh.linkDst, err)
		}
	}

	nlnh := &netlink.Neigh{
		IP:           dstIP,
		HardwareAddr: dstMac,
		State:        netlink.NUD_PERMANENT,
		Family:       nh.family,
	}

	if nlnh.Family > 0 {
		nlnh.Flags = netlink.NTF_SELF
	}

	if nh.linkDst != "" {
		nlnh.LinkIndex = iface.Attrs().Index
	}

	if err := nlh.NeighSet(nlnh); err != nil {
		return fmt.Errorf("could not add neighbor entry:%+v error:%v", nlnh, err)
	}

	if neighborAlreadyPresent {
		return nil
	}

	n.mu.Lock()
	n.neighbors = append(n.neighbors, nh)
	n.mu.Unlock()
	log.G(context.TODO()).Debugf("Neighbor entry added for IP:%v, mac:%v on ifc:%s", dstIP, dstMac, nh.linkName)

	return nil
}
