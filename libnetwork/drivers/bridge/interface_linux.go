package bridge

import (
	"context"
	"fmt"
	"net"
	"net/netip"

	"github.com/containerd/log"
	"github.com/docker/docker/errdefs"
	"github.com/docker/docker/libnetwork/internal/netiputil"
	"github.com/vishvananda/netlink"
)

const (
	// DefaultBridgeName is the default name for the bridge interface managed
	// by the driver when unspecified by the caller.
	DefaultBridgeName = "docker0"
)

// Interface models the bridge network device.
type bridgeInterface struct {
	Link        netlink.Link
	bridgeIPv4  *net.IPNet
	bridgeIPv6  *net.IPNet
	gatewayIPv4 net.IP
	gatewayIPv6 net.IP
	nlh         *netlink.Handle
}

// newInterface creates a new bridge interface structure. It attempts to find
// an already existing device identified by the configuration BridgeName field,
// or the default bridge name when unspecified, but doesn't attempt to create
// one when missing
func newInterface(nlh *netlink.Handle, config *networkConfiguration) (*bridgeInterface, error) {
	var err error
	i := &bridgeInterface{nlh: nlh}

	// Initialize the bridge name to the default if unspecified.
	if config.BridgeName == "" {
		config.BridgeName = DefaultBridgeName
	}

	// Attempt to find an existing bridge named with the specified name.
	i.Link, err = nlh.LinkByName(config.BridgeName)
	if err != nil {
		log.G(context.TODO()).Debugf("Did not find any interface with name %s: %v", config.BridgeName, err)
	} else if _, ok := i.Link.(*netlink.Bridge); !ok {
		return nil, fmt.Errorf("existing interface %s is not a bridge", i.Link.Attrs().Name)
	}
	return i, nil
}

// exists indicates if the existing bridge interface exists on the system.
func (i *bridgeInterface) exists() bool {
	return i.Link != nil
}

// addresses returns a bridge's addresses, IPv4 (with family=netlink.FAMILY_V4)
// or IPv6 (family=netlink.FAMILY_V6).
func (i *bridgeInterface) addresses(family int) ([]netlink.Addr, error) {
	if !i.exists() {
		// A nonexistent interface, by definition, cannot have any addresses.
		return nil, nil
	}
	addrs, err := i.nlh.AddrList(i.Link, family)
	if err != nil {
		return nil, fmt.Errorf("Failed to retrieve addresses: %v", err)
	}
	return addrs, nil
}

func (i *bridgeInterface) programIPv6Addresses(config *networkConfiguration) error {
	// Remember the configured addresses.
	i.bridgeIPv6 = config.AddressIPv6
	i.gatewayIPv6 = config.AddressIPv6.IP

	addrPrefix, ok := netiputil.ToPrefix(config.AddressIPv6)
	if !ok {
		return errdefs.System(
			fmt.Errorf("failed to convert bridge IPv6 address '%s' to netip.Prefix",
				config.AddressIPv6.String()))
	}

	// Get the IPv6 addresses currently assigned to the bridge, if any.
	existingAddrs, err := i.addresses(netlink.FAMILY_V6)
	if err != nil {
		return errdefs.System(err)
	}
	// Remove addresses that aren't required.
	for _, existingAddr := range existingAddrs {
		ea, ok := netip.AddrFromSlice(existingAddr.IP)
		if !ok {
			return errdefs.System(
				fmt.Errorf("Failed to convert IPv6 address '%s' to netip.Addr", config.AddressIPv6))
		}
		// Don't delete the kernel-assigned link local address (or fe80::1 - if it was
		// assigned to the bridge by an older version of the daemon that deleted the
		// kernel_ll address, the bridge won't get a new kernel_ll address.) But, do
		// delete unexpected link-local addresses (fe80::/10) that aren't in fe80::/64,
		// those have been IPAM-assigned.
		if p, _ := ea.Prefix(64); p == linkLocalPrefix {
			continue
		}
		// Don't delete multicast addresses as they're never added by the daemon.
		if ea.IsMulticast() {
			continue
		}
		// Ignore the prefix length when comparing addresses, it's informational
		// (RFC-5942 section 4), and removing/re-adding an address that's still valid
		// would disrupt traffic on live-restore.
		if ea != addrPrefix.Addr() {
			err := i.nlh.AddrDel(i.Link, &existingAddr) //#nosec G601 -- Memory aliasing is not an issue in practice as the &existingAddr pointer is not retained by the callee after the AddrDel() call returns.
			if err != nil {
				log.G(context.TODO()).WithFields(log.Fields{
					"error":   err,
					"address": existingAddr.IPNet,
				},
				).Warnf("Failed to remove residual IPv6 address from bridge")
			}
		}
	}
	// Using AddrReplace(), rather than AddrAdd(). When the subnet is changed for an
	// existing bridge in a way that doesn't affect the bridge's assigned address,
	// the old address has not been removed at this point - because that would be
	// service-affecting for a running container.
	//
	// But if, for example, 'fixed-cidr-v6' is changed from '2000:dbe::/64' to
	// '2000:dbe::/80', the default bridge will still be assigned address
	// '2000:dbe::1'. In the output of 'ip a', the prefix length is displayed - and
	// the user is likely to expect to see it updated from '64' to '80'.
	// Unfortunately, 'netlink.AddrReplace()' ('RTM_NEWADDR' with 'NLM_F_REPLACE')
	// doesn't update the prefix length. This is a cosmetic problem, the prefix
	// length of an assigned address is not used to determine whether an address is
	// "on-link" (RFC-5942).
	if err := i.nlh.AddrReplace(i.Link, &netlink.Addr{IPNet: netiputil.ToIPNet(addrPrefix)}); err != nil {
		return errdefs.System(fmt.Errorf("failed to add IPv6 address %s to bridge: %v", i.bridgeIPv6, err))
	}
	return nil
}
