package osl

import (
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/containerd/log"
	"github.com/docker/docker/libnetwork/ns"
	"github.com/docker/docker/libnetwork/types"
	"github.com/pkg/errors"
	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netns"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// newInterface creates a new interface in the given namespace using the
// provided options.
func newInterface(ns *Namespace, srcName, dstPrefix string, options ...IfaceOption) (*Interface, error) {
	i := &Interface{
		srcName: srcName,
		dstName: dstPrefix,
		ns:      ns,
	}
	for _, opt := range options {
		if opt != nil {
			// TODO(thaJeztah): use multi-error instead of returning early.
			if err := opt(i); err != nil {
				return nil, err
			}
		}
	}
	if i.master != "" {
		i.dstMaster = ns.findDst(i.master, true)
		if i.dstMaster == "" {
			return nil, fmt.Errorf("could not find an appropriate master %q for %q", i.master, i.srcName)
		}
	}
	return i, nil
}

// Interface represents the settings and identity of a network device.
// It is used as a return type for Network.Link, and it is common practice
// for the caller to use this information when moving interface SrcName from
// host namespace to DstName in a different net namespace with the appropriate
// network settings.
type Interface struct {
	srcName     string
	dstName     string
	master      string
	dstMaster   string
	mac         net.HardwareAddr
	address     *net.IPNet
	addressIPv6 *net.IPNet
	llAddrs     []*net.IPNet
	routes      []*net.IPNet
	bridge      bool
	sysctls     []string
	ns          *Namespace
}

// SrcName returns the name of the interface in the origin network namespace.
func (i *Interface) SrcName() string {
	return i.srcName
}

// DstName returns the name that will be assigned to the interface once
// moved inside a network namespace. When the caller passes in a DstName,
// it is only expected to pass a prefix. The name will be modified with an
// auto-generated suffix.
func (i *Interface) DstName() string {
	return i.dstName
}

func (i *Interface) DstMaster() string {
	return i.dstMaster
}

// Bridge returns true if the interface is a bridge.
func (i *Interface) Bridge() bool {
	return i.bridge
}

func (i *Interface) MacAddress() net.HardwareAddr {
	return types.GetMacCopy(i.mac)
}

// Address returns the IPv4 address for the interface.
func (i *Interface) Address() *net.IPNet {
	return types.GetIPNetCopy(i.address)
}

// AddressIPv6 returns the IPv6 address for the interface.
func (i *Interface) AddressIPv6() *net.IPNet {
	return types.GetIPNetCopy(i.addressIPv6)
}

// LinkLocalAddresses returns the link-local IP addresses assigned to the
// interface.
func (i *Interface) LinkLocalAddresses() []*net.IPNet {
	return i.llAddrs
}

// Routes returns IP routes for the interface.
func (i *Interface) Routes() []*net.IPNet {
	routes := make([]*net.IPNet, len(i.routes))
	for index, route := range i.routes {
		routes[index] = types.GetIPNetCopy(route)
	}

	return routes
}

// Remove an interface from the sandbox by renaming to original name
// and moving it out of the sandbox.
func (i *Interface) Remove() error {
	nameSpace := i.ns
	return nameSpace.RemoveInterface(i)
}

// Statistics returns the sandbox's side veth interface statistics.
func (i *Interface) Statistics() (*types.InterfaceStatistics, error) {
	l, err := i.ns.nlHandle.LinkByName(i.DstName())
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve the statistics for %s in netns %s: %v", i.DstName(), i.ns.path, err)
	}

	stats := l.Attrs().Statistics
	if stats == nil {
		return nil, fmt.Errorf("no statistics were returned")
	}

	return &types.InterfaceStatistics{
		RxBytes:   stats.RxBytes,
		TxBytes:   stats.TxBytes,
		RxPackets: stats.RxPackets,
		TxPackets: stats.TxPackets,
		RxDropped: stats.RxDropped,
		TxDropped: stats.TxDropped,
	}, nil
}

func (n *Namespace) findDst(srcName string, isBridge bool) string {
	n.mu.Lock()
	defer n.mu.Unlock()

	for _, i := range n.iFaces {
		// The master should match the srcname of the interface and the
		// master interface should be of type bridge, if searching for a bridge type
		if i.SrcName() == srcName && (!isBridge || i.Bridge()) {
			return i.DstName()
		}
	}

	return ""
}

func moveLink(ctx context.Context, nlhHost *netlink.Handle, iface netlink.Link, i *Interface, path string) error {
	ctx, span := otel.Tracer("").Start(ctx, "libnetwork.osl.moveLink", trace.WithAttributes(
		attribute.String("ifaceName", i.DstName())))
	defer span.End()

	newNs, err := netns.GetFromPath(path)
	if err != nil {
		return fmt.Errorf("failed get network namespace %q: %v", path, err)
	}
	defer newNs.Close()
	if err := nlhHost.LinkSetNsFd(iface, int(newNs)); err != nil {
		return fmt.Errorf("failed to set namespace on link %q: %v", i.srcName, err)
	}
	return nil
}

// AddInterface adds an existing Interface to the sandbox. The operation will rename
// from the Interface SrcName to DstName as it moves, and reconfigure the
// interface according to the specified settings. The caller is expected
// to only provide a prefix for DstName. The AddInterface api will auto-generate
// an appropriate suffix for the DstName to disambiguate.
func (n *Namespace) AddInterface(ctx context.Context, srcName, dstPrefix string, options ...IfaceOption) error {
	ctx, span := otel.Tracer("").Start(ctx, "libnetwork.osl.AddInterface", trace.WithAttributes(
		attribute.String("srcName", srcName),
		attribute.String("dstPrefix", dstPrefix)))
	defer span.End()

	i, err := newInterface(n, srcName, dstPrefix, options...)
	if err != nil {
		return err
	}

	n.mu.Lock()
	if n.isDefault {
		i.dstName = i.srcName
	} else {
		i.dstName = fmt.Sprintf("%s%d", dstPrefix, n.nextIfIndex[dstPrefix])
		n.nextIfIndex[dstPrefix]++
	}

	path := n.path
	isDefault := n.isDefault
	nlh := n.nlHandle
	nlhHost := ns.NlHandle()
	n.mu.Unlock()

	// If it is a bridge interface we have to create the bridge inside
	// the namespace so don't try to lookup the interface using srcName
	if i.bridge {
		if err := nlh.LinkAdd(&netlink.Bridge{
			LinkAttrs: netlink.LinkAttrs{
				Name: i.srcName,
			},
		}); err != nil {
			return fmt.Errorf("failed to create bridge %q: %v", i.srcName, err)
		}
	} else {
		// Find the network interface identified by the SrcName attribute.
		iface, err := nlhHost.LinkByName(i.srcName)
		if err != nil {
			return fmt.Errorf("failed to get link by name %q: %v", i.srcName, err)
		}

		// Move the network interface to the destination
		// namespace only if the namespace is not a default
		// type
		if !isDefault {
			if err := moveLink(ctx, nlhHost, iface, i, path); err != nil {
				return err
			}
		}
	}

	// Find the network interface identified by the SrcName attribute.
	iface, err := nlh.LinkByName(i.srcName)
	if err != nil {
		return fmt.Errorf("failed to get link by name %q: %v", i.srcName, err)
	}

	// Down the interface before configuring
	if err := nlh.LinkSetDown(iface); err != nil {
		return fmt.Errorf("failed to set link down: %v", err)
	}

	// Configure the interface now this is moved in the proper namespace.
	if err := n.configureInterface(ctx, nlh, iface, i); err != nil {
		// If configuring the device fails move it back to the host namespace
		// and change the name back to the source name. This allows the caller
		// to properly cleanup the interface. Its important especially for
		// interfaces with global attributes, ex: vni id for vxlan interfaces.
		if nerr := nlh.LinkSetName(iface, i.SrcName()); nerr != nil {
			log.G(ctx).Errorf("renaming interface (%s->%s) failed, %v after config error %v", i.DstName(), i.SrcName(), nerr, err)
		}
		if nerr := nlh.LinkSetNsFd(iface, ns.ParseHandlerInt()); nerr != nil {
			log.G(ctx).Errorf("moving interface %s to host ns failed, %v, after config error %v", i.SrcName(), nerr, err)
		}
		return err
	}

	// Up the interface.
	cnt := 0
	for err = nlh.LinkSetUp(iface); err != nil && cnt < 3; cnt++ {
		ctx, span2 := otel.Tracer("").Start(ctx, "libnetwork.osl.retryingLinkUp", trace.WithAttributes(
			attribute.String("srcName", srcName),
			attribute.String("dstPrefix", dstPrefix)))
		defer span2.End()
		log.G(ctx).Debugf("retrying link setup because of: %v", err)
		time.Sleep(10 * time.Millisecond)
		err = nlh.LinkSetUp(iface)
	}
	if err != nil {
		return fmt.Errorf("failed to set link up: %v", err)
	}

	// Set the routes on the interface. This can only be done when the interface is up.
	if err := setInterfaceRoutes(ctx, nlh, iface, i); err != nil {
		return fmt.Errorf("error setting interface %q routes to %q: %v", iface.Attrs().Name, i.Routes(), err)
	}

	n.mu.Lock()
	n.iFaces = append(n.iFaces, i)
	n.mu.Unlock()

	return nil
}

// RemoveInterface removes an interface from the namespace by renaming to
// original name and moving it out of the sandbox.
func (n *Namespace) RemoveInterface(i *Interface) error {
	n.mu.Lock()
	isDefault := n.isDefault
	nlh := n.nlHandle
	n.mu.Unlock()

	// Find the network interface identified by the DstName attribute.
	iface, err := nlh.LinkByName(i.DstName())
	if err != nil {
		return err
	}

	// Down the interface before configuring
	if err := nlh.LinkSetDown(iface); err != nil {
		return err
	}

	// TODO(aker): Why are we doing this? This would fail if the initial interface set up failed before the "dest interface" was moved into its own namespace; see https://github.com/moby/moby/pull/46315/commits/108595c2fe852a5264b78e96f9e63cda284990a6#r1331253578
	err = nlh.LinkSetName(iface, i.SrcName())
	if err != nil {
		log.G(context.TODO()).Debugf("LinkSetName failed for interface %s: %v", i.SrcName(), err)
		return err
	}

	// if it is a bridge just delete it.
	if i.Bridge() {
		if err := nlh.LinkDel(iface); err != nil {
			return fmt.Errorf("failed deleting bridge %q: %v", i.SrcName(), err)
		}
	} else if !isDefault {
		// Move the network interface to caller namespace.
		// TODO(aker): What's this really doing? There are no calls to LinkDel in this package: is this code really used? (Interface.Remove() has 3 callers); see https://github.com/moby/moby/pull/46315/commits/108595c2fe852a5264b78e96f9e63cda284990a6#r1331265335
		if err := nlh.LinkSetNsFd(iface, ns.ParseHandlerInt()); err != nil {
			log.G(context.TODO()).Debugf("LinkSetNsFd failed for interface %s: %v", i.SrcName(), err)
			return err
		}
	}

	n.mu.Lock()
	for index, intf := range i.ns.iFaces {
		if intf == i {
			i.ns.iFaces = append(i.ns.iFaces[:index], i.ns.iFaces[index+1:]...)
			break
		}
	}
	n.mu.Unlock()

	return nil
}

func (n *Namespace) configureInterface(ctx context.Context, nlh *netlink.Handle, iface netlink.Link, i *Interface) error {
	ctx, span := otel.Tracer("").Start(ctx, "libnetwork.osl.configureInterface", trace.WithAttributes(
		attribute.String("ifaceName", iface.Attrs().Name)))
	defer span.End()

	ifaceName := iface.Attrs().Name
	ifaceConfigurators := []struct {
		Fn         func(context.Context, *netlink.Handle, netlink.Link, *Interface) error
		ErrMessage string
	}{
		{setInterfaceName, fmt.Sprintf("error renaming interface %q to %q", ifaceName, i.DstName())},
		{setInterfaceMAC, fmt.Sprintf("error setting interface %q MAC to %q", ifaceName, i.MacAddress())},
		{setInterfaceIP, fmt.Sprintf("error setting interface %q IP to %v", ifaceName, i.Address())},
		{setInterfaceIPv6, fmt.Sprintf("error setting interface %q IPv6 to %v", ifaceName, i.AddressIPv6())},
		{setInterfaceMaster, fmt.Sprintf("error setting interface %q master to %q", ifaceName, i.DstMaster())},
		{setInterfaceLinkLocalIPs, fmt.Sprintf("error setting interface %q link local IPs to %v", ifaceName, i.LinkLocalAddresses())},
	}

	for _, config := range ifaceConfigurators {
		if err := config.Fn(ctx, nlh, iface, i); err != nil {
			return fmt.Errorf("%s: %v", config.ErrMessage, err)
		}
	}

	if err := n.setSysctls(ctx, i.dstName, i.sysctls); err != nil {
		return err
	}

	return nil
}

func setInterfaceMaster(ctx context.Context, nlh *netlink.Handle, iface netlink.Link, i *Interface) error {
	if i.DstMaster() == "" {
		return nil
	}

	ctx, span := otel.Tracer("").Start(ctx, "libnetwork.osl.setInterfaceMaster", trace.WithAttributes(
		attribute.String("i.SrcName", i.SrcName()),
		attribute.String("i.DstName", i.DstName())))
	defer span.End()

	return nlh.LinkSetMaster(iface, &netlink.Bridge{
		LinkAttrs: netlink.LinkAttrs{Name: i.DstMaster()},
	})
}

func setInterfaceMAC(ctx context.Context, nlh *netlink.Handle, iface netlink.Link, i *Interface) error {
	if i.MacAddress() == nil {
		return nil
	}

	ctx, span := otel.Tracer("").Start(ctx, "libnetwork.osl.setInterfaceMAC", trace.WithAttributes(
		attribute.String("i.SrcName", i.SrcName()),
		attribute.String("i.DstName", i.DstName())))
	defer span.End()

	return nlh.LinkSetHardwareAddr(iface, i.MacAddress())
}

func setInterfaceIP(ctx context.Context, nlh *netlink.Handle, iface netlink.Link, i *Interface) error {
	if i.Address() == nil {
		return nil
	}

	ctx, span := otel.Tracer("").Start(ctx, "libnetwork.osl.setInterfaceIP", trace.WithAttributes(
		attribute.String("i.SrcName", i.SrcName()),
		attribute.String("i.DstName", i.DstName())))
	defer span.End()

	if err := checkRouteConflict(nlh, i.Address(), netlink.FAMILY_V4); err != nil {
		return err
	}
	ipAddr := &netlink.Addr{IPNet: i.Address(), Label: ""}
	return nlh.AddrAdd(iface, ipAddr)
}

func setInterfaceIPv6(ctx context.Context, nlh *netlink.Handle, iface netlink.Link, i *Interface) error {
	addr := i.AddressIPv6()
	ctx, span := otel.Tracer("").Start(ctx, "libnetwork.osl.setInterfaceIPv6", trace.WithAttributes(
		attribute.String("i.SrcName", i.SrcName()),
		attribute.String("i.DstName", i.DstName()),
		attribute.String("i.AddressIPv6", addr.String())))
	defer span.End()

	// IPv6 must be enabled on the interface if and only if the network is
	// IPv6-enabled. For an interface on an IPv4-only network, if IPv6 isn't
	// disabled, the interface will be put into IPv6 multicast groups making
	// it unexpectedly susceptible to NDP cache poisoning, route injection, etc.
	// (At present, there will always be a pre-configured IPv6 address if the
	// network is IPv6-enabled.)
	if err := setIPv6(i.ns.path, i.DstName(), addr != nil); err != nil {
		return fmt.Errorf("failed to configure ipv6: %v", err)
	}
	if addr == nil {
		return nil
	}
	if err := checkRouteConflict(nlh, addr, netlink.FAMILY_V6); err != nil {
		return err
	}
	nlAddr := &netlink.Addr{IPNet: addr, Label: "", Flags: syscall.IFA_F_NODAD}
	return nlh.AddrAdd(iface, nlAddr)
}

func setInterfaceLinkLocalIPs(ctx context.Context, nlh *netlink.Handle, iface netlink.Link, i *Interface) error {
	ctx, span := otel.Tracer("").Start(ctx, "libnetwork.osl.setInterfaceLinkLocalIPs", trace.WithAttributes(
		attribute.String("i.SrcName", i.SrcName()),
		attribute.String("i.DstName", i.DstName())))
	defer span.End()

	for _, llIP := range i.LinkLocalAddresses() {
		ipAddr := &netlink.Addr{IPNet: llIP}
		if err := nlh.AddrAdd(iface, ipAddr); err != nil {
			return err
		}
	}
	return nil
}

func (n *Namespace) setSysctls(ctx context.Context, ifName string, sysctls []string) error {
	ctx, span := otel.Tracer("").Start(ctx, "libnetwork.osl.setSysctls", trace.WithAttributes(
		attribute.String("ifName", ifName)))
	defer span.End()

	for _, sc := range sysctls {
		k, v, found := strings.Cut(sc, "=")
		if !found {
			return fmt.Errorf("expected sysctl '%s' to have format name=value", sc)
		}
		sk := strings.Split(k, ".")
		if len(sk) != 5 {
			return fmt.Errorf("expected sysctl '%s' to have format net.X.Y.IFNAME.Z", sc)
		}

		sysPath := filepath.Join(append([]string{"/proc/sys", sk[0], sk[1], sk[2], ifName}, sk[4:]...)...)
		var errF error
		f := func() {
			if fi, err := os.Stat(sysPath); err != nil || !fi.Mode().IsRegular() {
				errF = fmt.Errorf("%s is not a sysctl file", sysPath)
			} else if curVal, err := os.ReadFile(sysPath); err != nil {
				errF = errors.Wrapf(err, "unable to read '%s'", sysPath)
			} else if strings.TrimSpace(string(curVal)) == v {
				// The value is already correct, don't try to write the file in case
				// "/proc/sys/net" is a read-only filesystem.
			} else if err := os.WriteFile(sysPath, []byte(v), 0o644); err != nil {
				errF = errors.Wrapf(err, "unable to write to '%s'", sysPath)
			}
		}

		if err := n.InvokeFunc(f); err != nil {
			return errors.Wrapf(err, "failed to run sysctl setter in network namespace")
		}
		if errF != nil {
			return errF
		}
	}
	return nil
}

func setInterfaceName(ctx context.Context, nlh *netlink.Handle, iface netlink.Link, i *Interface) error {
	ctx, span := otel.Tracer("").Start(ctx, "libnetwork.osl.setInterfaceName", trace.WithAttributes(
		attribute.String("ifaceName", iface.Attrs().Name)))
	defer span.End()

	return nlh.LinkSetName(iface, i.DstName())
}

func setInterfaceRoutes(ctx context.Context, nlh *netlink.Handle, iface netlink.Link, i *Interface) error {
	ctx, span := otel.Tracer("").Start(ctx, "libnetwork.osl.setInterfaceRoutes", trace.WithAttributes(
		attribute.String("i.SrcName", i.SrcName()),
		attribute.String("i.DstName", i.DstName())))
	defer span.End()

	for _, route := range i.Routes() {
		err := nlh.RouteAdd(&netlink.Route{
			Scope:     netlink.SCOPE_LINK,
			LinkIndex: iface.Attrs().Index,
			Dst:       route,
		})
		if err != nil {
			return err
		}
	}
	return nil
}

func checkRouteConflict(nlh *netlink.Handle, address *net.IPNet, family int) error {
	routes, err := nlh.RouteList(nil, family)
	if err != nil {
		return err
	}
	for _, route := range routes {
		if route.Dst != nil && !route.Dst.IP.IsUnspecified() {
			if route.Dst.Contains(address.IP) || address.Contains(route.Dst.IP) {
				return fmt.Errorf("cannot program address %v in sandbox interface because it conflicts with existing route %s",
					address, route)
			}
		}
	}
	return nil
}
