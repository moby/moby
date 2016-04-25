package osl

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net"

	"github.com/vishvananda/netlink"
)

// NeighOption is a function option type to set interface options
type NeighOption func(nh *neigh)

type neigh struct {
	dstIP    net.IP
	dstMac   net.HardwareAddr
	linkName string
	linkDst  string
	family   int
}

func (n *neigh) MarshalJSON() ([]byte, error) {
	nMap := make(map[string]interface{})
	if n.dstIP != nil {
		nMap["dstIP"] = n.dstIP.String()
	}
	if n.dstMac != nil {
		nMap["dstMac"] = n.dstMac.String()
	}
	nMap["linkName"] = n.linkName
	nMap["linkDst"] = n.linkDst
	nMap["family"] = n.family
	return json.Marshal(nMap)
}

func (n *neigh) UnmarshalJSON(b []byte) error {
	var nMap map[string]interface{}
	if err := json.Unmarshal(b, &nMap); err != nil {
		return err
	}
	if v, ok := nMap["dstIP"]; ok {
		n.dstIP = net.ParseIP(v.(string))
	}
	if v, ok := nMap["dstMac"]; ok {
		mac, err := net.ParseMAC(v.(string))
		if err != nil {
			return err
		}
		n.dstMac = mac
	}
	n.linkName = nMap["linkName"].(string)
	n.linkDst = nMap["linkDst"].(string)
	n.family = int(nMap["family"].(float64))
	return nil
}

func (n *networkNamespace) findNeighbor(dstIP net.IP, dstMac net.HardwareAddr) *neigh {
	n.Lock()
	defer n.Unlock()

	for _, nh := range n.neighbors {
		if nh.dstIP.Equal(dstIP) && bytes.Equal(nh.dstMac, dstMac) {
			return nh
		}
	}

	return nil
}

func (n *networkNamespace) DeleteNeighbor(dstIP net.IP, dstMac net.HardwareAddr) error {
	nh := n.findNeighbor(dstIP, dstMac)
	if nh == nil {
		return fmt.Errorf("could not find the neighbor entry to delete")
	}

	return nsInvoke(n.nsPath(), func(nsFD int) error { return nil }, func(callerFD int) error {
		var iface netlink.Link

		if nh.linkDst != "" {
			var err error
			iface, err = netlink.LinkByName(nh.linkDst)
			if err != nil {
				return fmt.Errorf("could not find interface with destination name %s: %v",
					nh.linkDst, err)
			}
		}

		nlnh := &netlink.Neigh{
			IP:     dstIP,
			State:  netlink.NUD_PERMANENT,
			Family: nh.family,
		}

		if nlnh.Family > 0 {
			nlnh.HardwareAddr = dstMac
			nlnh.Flags = netlink.NTF_SELF
		}

		if nh.linkDst != "" {
			nlnh.LinkIndex = iface.Attrs().Index
		}

		if err := netlink.NeighDel(nlnh); err != nil {
			return fmt.Errorf("could not delete neighbor entry: %v", err)
		}

		for i, nh := range n.neighbors {
			if nh.dstIP.Equal(dstIP) && bytes.Equal(nh.dstMac, dstMac) {
				n.neighbors = append(n.neighbors[:i], n.neighbors[i+1:]...)
			}
		}

		return nil
	})
}

func (n *networkNamespace) AddNeighbor(dstIP net.IP, dstMac net.HardwareAddr, options ...NeighOption) error {
	nh := n.findNeighbor(dstIP, dstMac)
	if nh != nil {
		// If it exists silently return
		return nil
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

	return nsInvoke(n.nsPath(), func(nsFD int) error { return nil }, func(callerFD int) error {
		var iface netlink.Link

		if nh.linkDst != "" {
			var err error
			iface, err = netlink.LinkByName(nh.linkDst)
			if err != nil {
				return fmt.Errorf("could not find interface with destination name %s: %v",
					nh.linkDst, err)
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

		if err := netlink.NeighSet(nlnh); err != nil {
			return fmt.Errorf("could not add neighbor entry: %v", err)
		}

		n.neighbors = append(n.neighbors, nh)

		return nil
	})
}
