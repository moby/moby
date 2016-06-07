package overlay

import (
	"fmt"

	"github.com/docker/libnetwork/netutils"
	"github.com/docker/libnetwork/osl"
	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netns"
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

func createVxlan(name string, vni uint32) error {
	defer osl.InitOSContext()()

	vxlan := &netlink.Vxlan{
		LinkAttrs: netlink.LinkAttrs{Name: name},
		VxlanId:   int(vni),
		Learning:  true,
		Port:      vxlanPort,
		Proxy:     true,
		L3miss:    true,
		L2miss:    true,
	}

	if err := netlink.LinkAdd(vxlan); err != nil {
		return fmt.Errorf("error creating vxlan interface: %v", err)
	}

	return nil
}

func deleteInterface(name string) error {
	defer osl.InitOSContext()()

	link, err := netlink.LinkByName(name)
	if err != nil {
		return fmt.Errorf("failed to find interface with name %s: %v", name, err)
	}

	if err := netlink.LinkDel(link); err != nil {
		return fmt.Errorf("error deleting interface with name %s: %v", name, err)
	}

	return nil
}

func deleteVxlanByVNI(path string, vni uint32) error {
	defer osl.InitOSContext()()

	var nlh *netlink.Handle
	if path == "" {
		var err error
		nlh, err = netlink.NewHandle()
		if err != nil {
			return fmt.Errorf("failed to get netlink handle for current ns: %v", err)
		}
	} else {
		var (
			err error
			ns  netns.NsHandle
		)

		ns, err = netns.GetFromPath(path)
		if err != nil {
			return fmt.Errorf("failed to get ns handle for %s: %v", path, err)
		}
		defer ns.Close()

		nlh, err = netlink.NewHandleAt(ns)
		if err != nil {
			return fmt.Errorf("failed to get netlink handle for ns %s: %v", path, err)
		}
	}
	defer nlh.Delete()

	links, err := nlh.LinkList()
	if err != nil {
		return fmt.Errorf("failed to list interfaces while deleting vxlan interface by vni: %v", err)
	}

	for _, l := range links {
		if l.Type() == "vxlan" && (vni == 0 || l.(*netlink.Vxlan).VxlanId == int(vni)) {
			err = netlink.LinkDel(l)
			if err != nil {
				return fmt.Errorf("error deleting vxlan interface with id %d: %v", vni, err)
			}

			return nil
		}
	}

	return fmt.Errorf("could not find a vxlan interface to delete with id %d", vni)
}
