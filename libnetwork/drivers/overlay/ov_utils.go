//go:build linux

package overlay

import (
	"context"
	"fmt"
	"net"
	"syscall"

	"github.com/containerd/log"
	"github.com/docker/docker/libnetwork/drivers/overlay/overlayutils"
	"github.com/docker/docker/libnetwork/netutils"
	"github.com/docker/docker/libnetwork/ns"
	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netns"
)

var soTimeout = ns.NetlinkSocketsTimeout

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
	nlh := ns.NlHandle()

	// Generate a name for what will be the host side pipe interface
	name1, err := netutils.GenerateIfaceName(nlh, vethPrefix, vethLen)
	if err != nil {
		return "", "", fmt.Errorf("error generating veth name1: %v", err)
	}

	// Generate a name for what will be the sandbox side pipe interface
	name2, err := netutils.GenerateIfaceName(nlh, vethPrefix, vethLen)
	if err != nil {
		return "", "", fmt.Errorf("error generating veth name2: %v", err)
	}

	// Generate and add the interface pipe host <-> sandbox
	veth := &netlink.Veth{
		LinkAttrs: netlink.LinkAttrs{Name: name1, TxQLen: 0},
		PeerName:  name2,
	}
	if err := nlh.LinkAdd(veth); err != nil {
		return "", "", fmt.Errorf("error creating veth pair: %v", err)
	}

	return name1, name2, nil
}

func createVxlan(name string, vni uint32, mtu int, vtepIPv6 bool) error {
	vxlan := &netlink.Vxlan{
		LinkAttrs: netlink.LinkAttrs{Name: name, MTU: mtu},
		VxlanId:   int(vni),
		Learning:  true,
		Port:      int(overlayutils.VXLANUDPPort()),
		Proxy:     true,
		L3miss:    true,
		L2miss:    true,
	}

	// The kernel restricts the destination VTEP (virtual tunnel endpoint) in
	// VXLAN forwarding database entries to a single address family, defaulting
	// to IPv4 unless either an IPv6 group or default remote destination address
	// is configured when the VXLAN link is created.
	//
	// Set up the VXLAN link for IPv6 destination addresses by setting the VXLAN
	// group address to the IPv6 unspecified address, like iproute2.
	// https://github.com/iproute2/iproute2/commit/97d564b90ccb1e4a3c756d9caae161f55b2b63a2
	// https://patchwork.ozlabs.org/project/netdev/patch/20180917171325.GA2660@localhost.localdomain/
	if vtepIPv6 {
		vxlan.Group = net.IPv6unspecified
	}

	if err := ns.NlHandle().LinkAdd(vxlan); err != nil {
		return fmt.Errorf("error creating vxlan interface: %v", err)
	}

	return nil
}

func deleteInterface(name string) error {
	link, err := ns.NlHandle().LinkByName(name)
	if err != nil {
		return fmt.Errorf("failed to find interface with name %s: %v", name, err)
	}

	if err := ns.NlHandle().LinkDel(link); err != nil {
		return fmt.Errorf("error deleting interface with name %s: %v", name, err)
	}

	return nil
}

func deleteVxlanByVNI(path string, vni uint32) error {
	nlh := ns.NlHandle()
	if path != "" {
		ns, err := netns.GetFromPath(path)
		if err != nil {
			return fmt.Errorf("failed to get ns handle for %s: %v", path, err)
		}
		defer ns.Close()

		nlh, err = netlink.NewHandleAt(ns, syscall.NETLINK_ROUTE)
		if err != nil {
			return fmt.Errorf("failed to get netlink handle for ns %s: %v", path, err)
		}
		defer nlh.Close()
		err = nlh.SetSocketTimeout(soTimeout)
		if err != nil {
			log.G(context.TODO()).Warnf("Failed to set the timeout on the netlink handle sockets for vxlan deletion: %v", err)
		}
	}

	links, err := nlh.LinkList()
	if err != nil {
		return fmt.Errorf("failed to list interfaces while deleting vxlan interface by vni: %v", err)
	}

	for _, l := range links {
		if l.Type() == "vxlan" && (vni == 0 || l.(*netlink.Vxlan).VxlanId == int(vni)) {
			err = nlh.LinkDel(l)
			if err != nil {
				return fmt.Errorf("error deleting vxlan interface with id %d: %v", vni, err)
			}
			return nil
		}
	}

	return fmt.Errorf("could not find a vxlan interface to delete with id %d", vni)
}
