package overlay

import (
	"fmt"

	"github.com/docker/libnetwork/netutils"
	"github.com/docker/libnetwork/osl"
	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netlink/nl"
)

func validateID(nid, eid string) error {
	if nid == "" {
		return fmt.Errorf("invalid network id")
	}

	if eid == "" {
		return fmt.Errorf("invalid endpoint id")
	}

	return nil
}

func createVethPair() (string, string, error) {
	defer osl.InitOSContext()()

	// Generate a name for what will be the host side pipe interface
	name1, err := netutils.GenerateIfaceName(vethPrefix, vethLen)
	if err != nil {
		return "", "", fmt.Errorf("error generating veth name1: %v", err)
	}

	// Generate a name for what will be the sandbox side pipe interface
	name2, err := netutils.GenerateIfaceName(vethPrefix, vethLen)
	if err != nil {
		return "", "", fmt.Errorf("error generating veth name2: %v", err)
	}

	// Generate and add the interface pipe host <-> sandbox
	veth := &netlink.Veth{
		LinkAttrs: netlink.LinkAttrs{Name: name1, TxQLen: 0},
		PeerName:  name2}
	if err := netlink.LinkAdd(veth); err != nil {
		return "", "", fmt.Errorf("error creating veth pair: %v", err)
	}

	return name1, name2, nil
}

func createVxlan(vni uint32) (string, error) {
	defer osl.InitOSContext()()

	name, err := netutils.GenerateIfaceName("vxlan", 7)
	if err != nil {
		return "", fmt.Errorf("error generating vxlan name: %v", err)
	}

	vxlan := &netlink.Vxlan{
		LinkAttrs: netlink.LinkAttrs{Name: name},
		VxlanId:   int(vni),
		Learning:  true,
		Port:      int(nl.Swap16(vxlanPort)), //network endian order
		Proxy:     true,
		L3miss:    true,
		L2miss:    true,
	}

	if err := netlink.LinkAdd(vxlan); err != nil {
		return "", fmt.Errorf("error creating vxlan interface: %v", err)
	}

	return name, nil
}

func deleteVxlan(name string) error {
	defer osl.InitOSContext()()

	link, err := netlink.LinkByName(name)
	if err != nil {
		return fmt.Errorf("failed to find vxlan interface with name %s: %v", name, err)
	}

	if err := netlink.LinkDel(link); err != nil {
		return fmt.Errorf("error deleting vxlan interface: %v", err)
	}

	return nil
}
