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

func getRequiredIPv6Addrs(config *networkConfiguration) (requiredAddrs map[netip.Addr]netip.Prefix, err error) {
	requiredAddrs = make(map[netip.Addr]netip.Prefix)

	// Always give the bridge 'fe80::1' - every interface is required to have an
	// address in 'fe80::/64'. Linux may assign an address, but we'll replace it with
	// 'fe80::1'. Then, if the configured prefix is 'fe80::/64', the IPAM pool
	// assigned address will not be a second address in the LL subnet.
	ra, ok := netiputil.ToPrefix(bridgeIPv6)
	if !ok {
		err = fmt.Errorf("Failed to convert Link-Local IPv6 address to netip.Prefix")
		return nil, err
	}
	requiredAddrs[ra.Addr()] = ra

	ra, ok = netiputil.ToPrefix(config.AddressIPv6)
	if !ok {
		err = fmt.Errorf("failed to convert bridge IPv6 address '%s' to netip.Prefix", config.AddressIPv6.String())
		return nil, err
	}
	requiredAddrs[ra.Addr()] = ra

	return requiredAddrs, nil
}

func (i *bridgeInterface) programIPv6Addresses(config *networkConfiguration) error {
	// Get the IPv6 addresses currently assigned to the bridge, if any.
	existingAddrs, err := i.addresses(netlink.FAMILY_V6)
	if err != nil {
		return errdefs.System(err)
	}

	// Get the list of required IPv6 addresses for this bridge.
	var requiredAddrs map[netip.Addr]netip.Prefix
	requiredAddrs, err = getRequiredIPv6Addrs(config)
	if err != nil {
		return errdefs.System(err)
	}
	i.bridgeIPv6 = config.AddressIPv6
	i.gatewayIPv6 = config.AddressIPv6.IP

	// Remove addresses that aren't required.
	for _, existingAddr := range existingAddrs {
		ea, ok := netip.AddrFromSlice(existingAddr.IP)
		if !ok {
			return errdefs.System(fmt.Errorf("Failed to convert IPv6 address '%s' to netip.Addr", config.AddressIPv6))
		}
		// Ignore the prefix length when comparing addresses, it's informational
		// (RFC-5942 section 4), and removing/re-adding an address that's still valid
		// would disrupt traffic on live-restore.
		if _, required := requiredAddrs[ea]; !required {
			err := i.nlh.AddrDel(i.Link, &existingAddr) //#nosec G601 -- Memory aliasing is not an issue in practice as the &existingAddr pointer is not retained by the callee after the AddrDel() call returns.
			if err != nil {
				log.G(context.TODO()).WithFields(log.Fields{"error": err, "address": existingAddr.IPNet}).Warnf("Failed to remove residual IPv6 address from bridge")
			}
		}
	}
	// Add or update required addresses.
	for _, addrPrefix := range requiredAddrs {
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
	}
	return nil
}
