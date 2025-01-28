package osl

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/containerd/log"
	"github.com/docker/docker/internal/nlwrap"
	"github.com/docker/docker/libnetwork/internal/l2disco"
	"github.com/docker/docker/libnetwork/ns"
	"github.com/docker/docker/libnetwork/types"
	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netns"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"golang.org/x/sys/unix"
)

const (
	// AdvertiseAddrNMsgsMin defines the minimum number of ARP/NA messages sent when an
	// interface is configured.
	// Zero can be used to disable unsolicited ARP/NA.
	AdvertiseAddrNMsgsMin = 0
	// AdvertiseAddrNMsgsMax defines the maximum number of ARP/NA messages sent when an
	// interface is configured. It's three, to match RFC-5227 Section 1.1
	//	// ("PROBE_NUM=3") and RFC-4861 MAX_NEIGHBOR_ADVERTISEMENT.
	AdvertiseAddrNMsgsMax = 3
	// advertiseAddrNMsgsDefault is the default number of ARP/NA messages sent when
	// an interface is configured.
	advertiseAddrNMsgsDefault = 3

	// AdvertiseAddrIntervalMin defines the minimum interval between ARP/NA messages
	// sent when an interface is configured. The min defined here is nonstandard,
	// RFC-5227 PROBE_MIN and the default for RetransTimer in RFC-4861 are one
	// second. But, faster resends may be useful in a bridge network (where packets
	// are not transmitted on a real network).
	AdvertiseAddrIntervalMin = 100 * time.Millisecond
	// AdvertiseAddrIntervalMax defines the maximum interval between ARP/NA messages
	// sent when an interface is configured. The max of 2s matches RFC-5227
	// PROBE_MAX.
	AdvertiseAddrIntervalMax = 2 * time.Second
	// advertiseAddrIntervalDefault is the default interval between ARP/NA messages
	// sent when and interface is configured.
	// One second matches RFC-5227 PROBE_MIN and the default for RetransTimer in RFC-4861.
	advertiseAddrIntervalDefault = time.Second
)

// newInterface creates a new interface in the given namespace using the
// provided options.
func newInterface(ns *Namespace, srcName, dstPrefix string, options ...IfaceOption) (*Interface, error) {
	i := &Interface{
		stopCh:                make(chan struct{}),
		srcName:               srcName,
		dstName:               dstPrefix,
		advertiseAddrNMsgs:    advertiseAddrNMsgsDefault,
		advertiseAddrInterval: advertiseAddrIntervalDefault,
		ns:                    ns,
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
	stopCh      chan struct{} // stopCh is closed before the interface is deleted.
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
	// advertiseAddrNMsgs is the number of unsolicited ARP/NA messages that will be sent to
	// advertise the interface's addresses. No messages will be sent if this is zero.
	advertiseAddrNMsgs int
	// advertiseAddrInterval is the interval between unsolicited ARP/NA messages sent to
	// advertise the interface's addresses.
	advertiseAddrInterval time.Duration
	createdInContainer    bool
	ns                    *Namespace
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

func moveLink(ctx context.Context, nlhHost nlwrap.Handle, iface netlink.Link, i *Interface, nsh netns.NsHandle) error {
	ctx, span := otel.Tracer("").Start(ctx, "libnetwork.osl.moveLink", trace.WithAttributes(
		attribute.String("ifaceName", i.DstName())))
	defer span.End()

	if err := nlhHost.LinkSetNsFd(iface, int(nsh)); err != nil {
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

	newNs := netns.None()
	if !isDefault {
		newNs, err = netns.GetFromPath(path)
		if err != nil {
			return fmt.Errorf("failed get network namespace %q: %v", path, err)
		}
		defer newNs.Close()
	}

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
	} else if !i.createdInContainer {
		// Find the network interface identified by the SrcName attribute.
		iface, err := nlhHost.LinkByName(i.srcName)
		if err != nil {
			return fmt.Errorf("failed to get link by name %q: %v", i.srcName, err)
		}

		// Move the network interface to the destination
		// namespace only if the namespace is not a default
		// type
		if !isDefault {
			if err := moveLink(ctx, nlhHost, iface, i, newNs); err != nil {
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
	log.G(ctx).Debug("link has been set to up")

	// Set the routes on the interface. This can only be done when the interface is up.
	if err := setInterfaceRoutes(ctx, nlh, iface, i); err != nil {
		return fmt.Errorf("error setting interface %q routes to %q: %v", iface.Attrs().Name, i.Routes(), err)
	}

	// Wait for the interface to be up and running (or a timeout).
	up, err := waitForIfUpped(ctx, newNs, iface.Attrs().Index)
	if err != nil {
		return err
	}

	// If the interface is up, send unsolicited ARP/NA messages if necessary.
	if up {
		if err := n.advertiseAddrs(ctx, iface.Attrs().Index, i, nlh); err != nil {
			return fmt.Errorf("failed to advertise addresses: %w", err)
		}
	}

	n.mu.Lock()
	n.iFaces = append(n.iFaces, i)
	n.mu.Unlock()

	return nil
}

func waitForIfUpped(ctx context.Context, ns netns.NsHandle, ifIndex int) (bool, error) {
	ctx, span := otel.Tracer("").Start(context.WithoutCancel(ctx), "libnetwork.osl.waitforIfUpped")
	defer span.End()

	update := make(chan netlink.LinkUpdate, 100)
	upped := make(chan struct{})
	opts := netlink.LinkSubscribeOptions{
		ListExisting: true, // in case the link is already up
		ErrorCallback: func(err error) {
			select {
			case <-upped:
				// Ignore errors sent after the upped channel is closed, the netlink
				// package sends an EAGAIN after it closes its netlink socket when it
				// sees this channel is closed. (No message is ever sent on upped.)
				return
			default:
			}
			log.G(ctx).WithFields(log.Fields{
				"ifi":   ifIndex,
				"error": err,
			}).Info("netlink error while waiting for interface up")
		},
	}
	if ns.IsOpen() {
		opts.Namespace = &ns
	}
	if err := nlwrap.LinkSubscribeWithOptions(update, upped, opts); err != nil {
		return false, fmt.Errorf("failed to subscribe to link updates: %w", err)
	}

	// When done (interface upped, or timeout), stop the LinkSubscribe and drain
	// the result channel. If the result channel isn't closed after a timeout,
	// log a warning to note the goroutine leak.
	defer func() {
		close(upped)
		drainTimerC := time.After(3 * time.Second)
		for {
			select {
			case _, ok := <-update:
				if !ok {
					return
				}
			case <-drainTimerC:
				log.G(ctx).Warn("timeout while waiting for LinkSubscribe to terminate")
			}
		}
	}()

	timerC := time.After(5 * time.Second)
	for {
		select {
		case <-timerC:
			log.G(ctx).Warnf("timeout in waitForIfUpped")
			return false, nil
		case u, ok := <-update:
			if !ok {
				// The netlink package failed to read from its netlink socket. It will
				// already have called the ErrorCallback, so the issue has been logged.
				return false, nil
			}
			if u.Attrs().Index != ifIndex {
				continue
			}
			log.G(ctx).WithFields(log.Fields{
				"iface": u.Attrs().Name,
				"ifi":   u.Attrs().Index,
				"flags": deviceFlags(u.Flags),
			}).Debug("link update")
			if u.Flags&unix.IFF_UP == unix.IFF_UP {
				return true, nil
			}
		}
	}
}

// advertiseAddrs triggers send unsolicited ARP and Neighbour Advertisement
// messages, so that caches are updated with the MAC address currently associated
// with the interface's IP addresses.
//
// IP addresses are recycled quickly when endpoints are dropped on network
// disconnect or container stop. A new MAC address may have been generated, so
// this is necessary to avoid packets sent to the old MAC address getting dropped
// until the ARP/Neighbour cache entries expire.
//
// Note that the kernel's arp_notify sysctl setting is not respected.
func (n *Namespace) advertiseAddrs(ctx context.Context, ifIndex int, i *Interface, nlh nlwrap.Handle) error {
	mac := i.MacAddress()
	address4 := i.Address()
	address6 := i.AddressIPv6()
	ctx = log.WithLogger(ctx, log.G(ctx).WithFields(log.Fields{
		"iface": i.dstName,
		"ifi":   ifIndex,
		"mac":   mac.String(),
		"ip4":   address4,
		"ip6":   address6,
	}))

	if address4 == nil && address6 == nil {
		// Nothing to do - for example, a bridge with no configured addresses.
		log.G(ctx).Debug("No IP addresses to advertise")
		return nil
	}
	if mac == nil {
		// Nothing to do - for example, a layer-3 ipvlan.
		log.G(ctx).Debug("No MAC address to advertise")
		return nil
	}
	if i.advertiseAddrNMsgs == 0 {
		log.G(ctx).Debug("Unsolicited ARP/NA is disabled")
		return nil
	}

	arpSender, naSender := n.prepAdvertiseAddrs(ctx, i, ifIndex)
	if arpSender == nil && naSender == nil {
		return nil
	}
	cleanup := func() {
		if arpSender != nil {
			arpSender.Close()
		}
		if naSender != nil {
			naSender.Close()
		}
	}
	stillSending := false
	defer func() {
		if !stillSending {
			cleanup()
		}
	}()

	send := func(ctx context.Context) error {
		link, err := nlh.LinkByIndex(ifIndex)
		if err != nil {
			return fmt.Errorf("failed to refresh link attributes: %w", err)
		}
		if curMAC := link.Attrs().HardwareAddr; !bytes.Equal(curMAC, mac) {
			log.G(ctx).WithFields(log.Fields{"newMAC": curMAC.String()}).Warn("MAC address changed")
			return fmt.Errorf("MAC address changed, got %s, expected %s", curMAC, mac.String())
		}
		log.G(ctx).Debug("Sending unsolicited ARP/NA")
		var errs []error
		if arpSender != nil {
			if err := arpSender.Send(); err != nil {
				log.G(ctx).WithError(err).Warn("Failed to send unsolicited ARP")
				errs = append(errs, err)
			}
		}
		if naSender != nil {
			// FIXME(robmry) - retry if this fails, but still return the error.
			// In CI, the send has failed a couple of times with "write ip ::1->ff02::1: sendmsg: network is unreachable".
			// Can't repro locally, so - try find out whether a retry helps and it's something racing, or it's a
			// persistent problem.
			for c := range 3 {
				if c > 0 {
					time.Sleep(50 * time.Millisecond)
				}

				routes, rgErr := nlh.RouteGetWithOptions(net.IPv6linklocalallnodes, &netlink.RouteGetOptions{
					IifIndex: ifIndex,
					SrcAddr:  net.IPv6loopback,
				})
				var routeStr string
				if rgErr != nil {
					routeStr = fmt.Sprintf("RouteGet->'%s'", rgErr.Error())
				} else if len(routes) != 1 {
					routeStr = fmt.Sprintf("RouteGet->%d routes", len(routes))
				} else {
					routeStr = fmt.Sprintf("RouteGet->'%s'", routes[0].String())
				}

				if err := naSender.Send(); err != nil {
					log.G(ctx).WithError(err).Warn("Failed to send unsolicited NA")
					errs = append(errs, fmt.Errorf("%s: %w", routeStr, err))
					continue
				}
				if c > 0 {
					errs = append(errs, fmt.Errorf("success"))
				}
				break
			}
		}
		return errors.Join(errs...)
	}

	// Send an initial message. If it fails, skip the resends.
	if err := send(ctx); err != nil {
		return err
	}
	if i.advertiseAddrNMsgs == 1 {
		return nil
	}
	// Don't clean up on return from this function, there are more ARPs/NAs to send.
	stillSending = true

	// Send the rest in the background.
	go func() {
		defer cleanup()
		ctx, span := otel.Tracer("").Start(context.WithoutCancel(ctx), "libnetwork.osl.advertiseAddrs")
		defer span.End()
		ticker := time.NewTicker(i.advertiseAddrInterval)
		defer ticker.Stop()
		for c := range i.advertiseAddrNMsgs - 1 {
			select {
			case <-i.stopCh:
				log.G(ctx).Debug("Unsolicited ARP/NA sends cancelled")
				return
			case <-ticker.C:
				if send(log.WithLogger(ctx, log.G(ctx).WithFields(log.Fields{"n": c + 1}))) != nil {
					return
				}
			}
		}
	}()

	return nil
}

func (n *Namespace) prepAdvertiseAddrs(ctx context.Context, i *Interface, ifIndex int) (*l2disco.UnsolARP, *l2disco.UnsolNA) {
	var ua *l2disco.UnsolARP
	var un *l2disco.UnsolNA
	if err := n.InvokeFunc(func() {
		if address4 := i.Address(); address4 != nil {
			var err error
			ua, err = l2disco.NewUnsolARP(ctx, address4.IP, i.MacAddress(), ifIndex)
			if err != nil {
				log.G(ctx).WithError(err).Warn("Failed to prepare unsolicited ARP")
			}
		}
		if address6 := i.AddressIPv6(); address6 != nil {
			var err error
			un, err = l2disco.NewUnsolNA(ctx, address6.IP, i.MacAddress(), ifIndex)
			if err != nil {
				log.G(ctx).WithError(err).Warn("Failed to prepare unsolicited NA")
			}
		}
	}); err != nil {
		log.G(ctx).WithError(err).Warn("Failed to prepare unsolicited ARP/NA messages")
		return nil, nil
	}
	return ua, un
}

// RemoveInterface removes an interface from the namespace by renaming to
// original name and moving it out of the sandbox.
func (n *Namespace) RemoveInterface(i *Interface) error {
	close(i.stopCh)
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

func (n *Namespace) configureInterface(ctx context.Context, nlh nlwrap.Handle, iface netlink.Link, i *Interface) error {
	ctx, span := otel.Tracer("").Start(ctx, "libnetwork.osl.configureInterface", trace.WithAttributes(
		attribute.String("ifaceName", iface.Attrs().Name)))
	defer span.End()

	ifaceName := iface.Attrs().Name
	ifaceConfigurators := []struct {
		Fn         func(context.Context, nlwrap.Handle, netlink.Link, *Interface) error
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

func setInterfaceMaster(ctx context.Context, nlh nlwrap.Handle, iface netlink.Link, i *Interface) error {
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

func setInterfaceMAC(ctx context.Context, nlh nlwrap.Handle, iface netlink.Link, i *Interface) error {
	if i.MacAddress() == nil {
		return nil
	}

	ctx, span := otel.Tracer("").Start(ctx, "libnetwork.osl.setInterfaceMAC", trace.WithAttributes(
		attribute.String("i.SrcName", i.SrcName()),
		attribute.String("i.DstName", i.DstName())))
	defer span.End()

	return nlh.LinkSetHardwareAddr(iface, i.MacAddress())
}

func setInterfaceIP(ctx context.Context, nlh nlwrap.Handle, iface netlink.Link, i *Interface) error {
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

func setInterfaceIPv6(ctx context.Context, nlh nlwrap.Handle, iface netlink.Link, i *Interface) error {
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

func setInterfaceLinkLocalIPs(ctx context.Context, nlh nlwrap.Handle, iface netlink.Link, i *Interface) error {
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
				errF = fmt.Errorf("unable to read '%s': %w", sysPath, err)
			} else if strings.TrimSpace(string(curVal)) == v {
				// The value is already correct, don't try to write the file in case
				// "/proc/sys/net" is a read-only filesystem.
			} else if err := os.WriteFile(sysPath, []byte(v), 0o644); err != nil {
				errF = fmt.Errorf("unable to write to '%s': %w", sysPath, err)
			}
		}

		if err := n.InvokeFunc(f); err != nil {
			return fmt.Errorf("failed to run sysctl setter in network namespace: %w", err)
		}
		if errF != nil {
			return errF
		}
	}
	return nil
}

func setInterfaceName(ctx context.Context, nlh nlwrap.Handle, iface netlink.Link, i *Interface) error {
	ctx, span := otel.Tracer("").Start(ctx, "libnetwork.osl.setInterfaceName", trace.WithAttributes(
		attribute.String("ifaceName", iface.Attrs().Name)))
	defer span.End()

	return nlh.LinkSetName(iface, i.DstName())
}

func setInterfaceRoutes(ctx context.Context, nlh nlwrap.Handle, iface netlink.Link, i *Interface) error {
	ctx, span := otel.Tracer("").Start(ctx, "libnetwork.osl.setInterfaceRoutes", trace.WithAttributes(
		attribute.String("i.SrcName", i.SrcName()),
		attribute.String("i.DstName", i.DstName())))
	defer span.End()

	for _, route := range i.Routes() {
		if route.IP.IsUnspecified() {
			// Don't set up a default route now, it'll be set later if this interface is
			// selected as the default gateway.
			continue
		}
		if err := nlh.RouteAdd(&netlink.Route{
			Scope:     netlink.SCOPE_LINK,
			LinkIndex: iface.Attrs().Index,
			Dst:       route,
		}); err != nil {
			return err
		}
	}
	return nil
}

func checkRouteConflict(nlh nlwrap.Handle, address *net.IPNet, family int) error {
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
