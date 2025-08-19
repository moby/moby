package osl

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/containerd/log"
	"github.com/moby/moby/v2/daemon/libnetwork/internal/l2disco"
	"github.com/moby/moby/v2/daemon/libnetwork/nlwrap"
	"github.com/moby/moby/v2/daemon/libnetwork/ns"
	"github.com/moby/moby/v2/daemon/libnetwork/types"
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
func newInterface(ns *Namespace, srcName, dstPrefix, dstName string, options ...IfaceOption) (*Interface, error) {
	i := &Interface{
		stopCh:                make(chan struct{}),
		srcName:               srcName,
		dstPrefix:             dstPrefix,
		dstName:               dstName,
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
	dstPrefix   string
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

// DstName returns the final interface name in the target network namespace.
// It's generated based on the prefix passed to [Namespace.AddInterface].
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
	return slices.Clone(i.mac)
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
		return nil, errors.New("no statistics were returned")
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

// AddInterface creates an Interface that represents an existing network
// interface (except for bridge interfaces, which are created here).
//
// The network interface will be reconfigured according the options passed, and
// it'll be renamed from srcName into either dstName if it's not empty, or to
// an auto-generated dest name that combines the provided dstPrefix and a
// numeric suffix.
//
// It's safe to call concurrently.
func (n *Namespace) AddInterface(ctx context.Context, srcName, dstPrefix, dstName string, options ...IfaceOption) error {
	ctx, span := otel.Tracer("").Start(ctx, "libnetwork.osl.AddInterface", trace.WithAttributes(
		attribute.String("srcName", srcName),
		attribute.String("dstPrefix", dstPrefix)))
	defer span.End()

	newNs := netns.None()
	if !n.isDefault {
		var err error
		newNs, err = netns.GetFromPath(n.path)
		if err != nil {
			return fmt.Errorf("failed get network namespace %q: %v", n.path, err)
		}
		defer newNs.Close()
	}

	i, iface, err := n.createInterface(ctx, newNs, srcName, dstPrefix, dstName, options...)
	if err != nil {
		return err
	}

	// Configure the interface now this is moved in the proper namespace.
	if err := n.configureInterface(ctx, n.nlHandle, iface, i); err != nil {
		// If configuring the device fails move it back to the host namespace
		// and change the name back to the source name. This allows the caller
		// to properly cleanup the interface. Its important especially for
		// interfaces with global attributes, ex: vni id for vxlan interfaces.
		if nerr := n.nlHandle.LinkSetName(iface, i.SrcName()); nerr != nil {
			log.G(ctx).Errorf("renaming interface (%s->%s) failed, %v after config error %v", i.DstName(), i.SrcName(), nerr, err)
		}
		if nerr := n.nlHandle.LinkSetNsFd(iface, ns.ParseHandlerInt()); nerr != nil {
			log.G(ctx).Errorf("moving interface %s to host ns failed, %v, after config error %v", i.SrcName(), nerr, err)
		}
		return err
	}

	// Up the interface.
	cnt := 0
	for err = n.nlHandle.LinkSetUp(iface); err != nil && cnt < 3; cnt++ {
		ctx, span2 := otel.Tracer("").Start(ctx, "libnetwork.osl.retryingLinkUp", trace.WithAttributes(
			attribute.String("srcName", srcName),
			attribute.String("dstPrefix", dstPrefix)))
		defer span2.End()
		log.G(ctx).Debugf("retrying link setup because of: %v", err)
		time.Sleep(10 * time.Millisecond)
		err = n.nlHandle.LinkSetUp(iface)
	}
	if err != nil {
		return fmt.Errorf("failed to set link up: %v", err)
	}
	log.G(ctx).Debug("link has been set to up")

	// Set the routes on the interface. This can only be done when the interface is up.
	if err := setInterfaceRoutes(ctx, n.nlHandle, iface, i); err != nil {
		return fmt.Errorf("error setting interface %q routes to %q: %v", iface.Attrs().Name, i.Routes(), err)
	}

	// Wait for the interface to be up and running (or a timeout).
	up, err := waitForIfUpped(ctx, newNs, iface.Attrs().Index)
	if err != nil {
		return err
	}

	// If the interface is up, send unsolicited ARP/NA messages if necessary.
	if up {
		waitForBridgePort(ctx, ns.NlHandle(), iface)
		mcastRouteOk := waitForMcastRoute(ctx, iface.Attrs().Index, i, n.nlHandle)
		if err := n.advertiseAddrs(ctx, iface.Attrs().Index, i, n.nlHandle, mcastRouteOk); err != nil {
			return fmt.Errorf("failed to advertise addresses: %w", err)
		}
	}

	return nil
}

// createInterface creates a new Interface, moves the underlying link into the
// target network namespace (if needed), and adds the interface to [Namespace.iFaces].
//
// If dstName is empty, createInterface will generate a unique suffix and
// append it to dstPrefix.
//
// It's safe to call concurrently.
func (n *Namespace) createInterface(ctx context.Context, targetNs netns.NsHandle, srcName, dstPrefix, dstName string, options ...IfaceOption) (*Interface, netlink.Link, error) {
	i, err := newInterface(n, srcName, dstPrefix, dstName, options...)
	if err != nil {
		return nil, nil, err
	}

	// It is not safe to call generateIfaceName and createInterface
	// concurrently, so the Namespace need to be locked until the interface
	// is added to n.iFaces.
	n.mu.Lock()
	defer n.mu.Unlock()

	if n.isDefault {
		i.dstName = i.srcName
	} else if i.dstName == "" {
		i.dstName = n.generateIfaceName(dstPrefix)
	}

	nlhHost := ns.NlHandle()

	// If it is a bridge interface we have to create the bridge inside
	// the namespace so don't try to lookup the interface using srcName
	if i.bridge {
		if err := n.nlHandle.LinkAdd(&netlink.Bridge{
			LinkAttrs: netlink.LinkAttrs{
				Name: i.srcName,
			},
		}); err != nil {
			return nil, nil, fmt.Errorf("failed to create bridge %q: %v", i.srcName, err)
		}
	} else if !i.createdInContainer {
		// Find the network interface identified by the SrcName attribute.
		iface, err := nlhHost.LinkByName(i.srcName)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to get link by name %q: %v", i.srcName, err)
		}

		// Move the network interface to the destination
		// namespace only if the namespace is not a default
		// type
		if !n.isDefault {
			if err := moveLink(ctx, nlhHost, iface, i, targetNs); err != nil {
				return nil, nil, err
			}
		}
	}

	// Find the network interface identified by the SrcName attribute.
	iface, err := n.nlHandle.LinkByName(i.srcName)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get link by name %q: %v", i.srcName, err)
	}

	// Down the interface before configuring
	if err := n.nlHandle.LinkSetDown(iface); err != nil {
		return nil, nil, fmt.Errorf("failed to set link down: %v", err)
	}

	if err := setInterfaceName(ctx, n.nlHandle, iface, i); err != nil {
		return nil, nil, fmt.Errorf("error renaming interface %q to %q: %w", iface.Attrs().Name, i.DstName(), err)
	}

	n.iFaces = append(n.iFaces, i)

	return i, iface, nil
}

func (n *Namespace) generateIfaceName(prefix string) string {
	var suffixes []int
	for _, i := range n.iFaces {
		if s, ok := strings.CutPrefix(i.DstName(), prefix); ok {
			// Ignore non-numerical prefixes and negative suffixes (they're
			// treated as a different prefix).
			if v, err := strconv.Atoi(s); err == nil && v >= 0 && s != "-0" {
				suffixes = append(suffixes, v)
			}
		}
	}

	sort.Ints(suffixes)

	// There are gaps in the numbering; find the first unused number.
	//
	// An alternative implementation could be to look at the highest suffix,
	// and increment it. But, if that incremented number makes the interface
	// name overflow the IFNAMSIZ limit (= 16 chars), the kernel would reject
	// that interface name while there are other unused numbers. So, instead
	// use the lowest suffix available.
	for i := 0; i < len(suffixes); i++ {
		if i != suffixes[i] {
			return prefix + strconv.Itoa(i)
		}
	}

	return prefix + strconv.Itoa(len(suffixes))
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

// waitForBridgePort checks whether link iface is a veth. If it is, and the other
// end of the veth is slaved to a bridge, waits for up to maxWait for the bridge
// port's state to be "forwarding". If STP is enabled on the bridge, it doesn't
// wait. If the port is still not forwarding when this returns, at-least the
// first unsolicited ARP/NA packets may be dropped.
func waitForBridgePort(ctx context.Context, nlh nlwrap.Handle, iface netlink.Link) {
	if iface.Type() != "veth" {
		return
	}
	ctx, span := otel.Tracer("").Start(ctx, "libnetwork.osl.waitForBridgePort")
	defer span.End()
	ctx = log.WithLogger(ctx, log.G(ctx).WithField("veth", iface.Attrs().Name))

	// The parent of a veth is the other end of the veth.
	parentIndex := iface.Attrs().ParentIndex
	if parentIndex <= 0 {
		log.G(ctx).Debug("veth has no parent index")
		return
	}
	parentIface, err := nlh.LinkByIndex(parentIndex)
	if err != nil {
		// The parent isn't in the host's netns, it's probably in a swarm load-balancer
		// sandbox, and we don't know where that is. But, swarm still uses IP-based MAC
		// addresses so the unsolicited ARPs aren't essential. If the first one goes
		// missing because the bridge's port isn't forwarding yet, it's ok.
		log.G(ctx).WithFields(log.Fields{"parentIndex": parentIndex, "error": err}).Debug("No parent interface")
		return
	}
	// If the other end of the veth has a MasterIndex, that's a bridge.
	if parentIface.Attrs().MasterIndex <= 0 {
		log.G(ctx).Debug("veth is not connected to a bridge")
		return
	}
	bridgeIface, err := nlh.LinkByIndex(parentIface.Attrs().MasterIndex)
	if err != nil {
		log.G(ctx).WithFields(log.Fields{
			"parentIndex": parentIndex,
			"masterIndex": parentIface.Attrs().MasterIndex,
			"error":       err,
		}).Warn("No parent bridge link")
		return
	}

	// Ideally, we'd read the port state via netlink. But, vishvananda/netlink needs a
	// patch to include state in its response.
	// - type Protinfo needs a "State uint8"
	// - parseProtinfo() needs "case nl.IFLA_BRPORT_STATE: pi.State = uint8(info.Value[0])"
	/*
		pi, err := nlh.LinkGetProtinfo(parentIface)
		if err != nil {
			return fmt.Errorf("get bridge protinfo: %w", err)
		}
	*/

	// Check that STP is not enabled on the bridge. It won't be enabled on a
	// bridge network's own bridge. But, could be on a user-supplied bridge
	// and, if it is, it won't be forwarding within the timeout here.
	if stpEnabled(ctx, bridgeIface.Attrs().Name) {
		log.G(ctx).Info("STP is enabled, not waiting for port to be forwarding")
		return
	}

	// Read the port state from "/sys/class/net/<bridge>/brif/<veth>/state".
	var portStateFile *os.File
	path := filepath.Join("/sys/class/net", bridgeIface.Attrs().Name, "brif", parentIface.Attrs().Name, "state")
	portStateFile, err = os.Open(path)
	if err != nil {
		// In integration tests where the daemon is running in its own netns, the bridge
		// device isn't visible in "/sys/class/net". So, just wait for hopefully-long-enough
		// for the bridge's port to be ready.
		log.G(ctx).WithField("port", path).Warn("Failed to open port state file, waiting for 20ms")
		time.Sleep(20 * time.Millisecond)
		return
	}
	defer portStateFile.Close()

	// Poll the bridge port's state until it's "forwarding". (By now, it should be. So, poll
	// quickly, and not for long.)
	const pollInterval = 10 * time.Millisecond
	const maxWait = 200 * time.Millisecond
	var stateFileContent [2]byte
	for range int64(maxWait / pollInterval) {
		n, err := portStateFile.ReadAt(stateFileContent[:], 0)
		if err != nil {
			log.G(ctx).WithFields(log.Fields{
				"filename": path,
				"error":    err,
			}).Warn("Failed to read bridge port state")
			return
		}
		if n == 0 {
			log.G(ctx).WithField("filename", path).Warn("Empty bridge port state file")
			return
		}
		// Forwarding is state '3'.
		// https://elixir.bootlin.com/linux/v6.13/source/include/uapi/linux/if_bridge.h#L49-L53
		if stateFileContent[0] != '3' {
			log.G(ctx).WithField("portState", stateFileContent[0]).Debug("waiting for bridge port to be forwarding")
			time.Sleep(pollInterval)
			continue
		}
		log.G(ctx).Debug("Bridge port is forwarding")
		return
	}
	log.G(ctx).WithFields(log.Fields{
		"portState": stateFileContent[0],
		"waitTime":  maxWait,
	}).Warn("Bridge port not forwarding")
}

// stpEnabled returns true if "/sys/class/net/<name>/bridge/stp_state" can be read
// and does not contain "0".
func stpEnabled(ctx context.Context, name string) bool {
	stpStateFilename := filepath.Join("/sys/class/net", name, "bridge/stp_state")
	stpState, err := os.ReadFile(stpStateFilename)
	if err != nil {
		log.G(ctx).WithError(err).Warnf("Failed to read stp_state file %q", stpStateFilename)
		return false
	}
	return len(stpState) > 0 && stpState[0] != '0'
}

// waitForMcastRoute waits for an interface to have a route from ::1 to the IPv6 LL all-nodes
// address (ff02::1), if that route is needed to send a neighbour advertisement for an IPv6
// interface address.
//
// After waiting, or a failure, if there is no route - no error is returned. The NA send may
// fail, but try it anyway.
//
// In CI, the NA send failed with "write ip ::1->ff02::1: sendmsg: network is unreachable".
// That error has not been seen since addition of the check that the veth's parent bridge port
// is forwarding, so that may have been the issue. But, in case it's a timing problem that's
// only less-likely because of delay caused by that check, make sure the route exists.
func waitForMcastRoute(ctx context.Context, ifIndex int, i *Interface, nlh nlwrap.Handle) bool {
	if i.addressIPv6 == nil || i.advertiseAddrNMsgs == 0 {
		return true
	}
	ctx, span := otel.Tracer("").Start(ctx, "libnetwork.osl.waitForMcastRoute")
	defer span.End()

	const pollInterval = 10 * time.Millisecond
	const maxWait = 200 * time.Millisecond
	for range int64(maxWait / pollInterval) {
		routes, err := nlh.RouteGetWithOptions(net.IPv6linklocalallnodes, &netlink.RouteGetOptions{
			IifIndex: ifIndex,
			SrcAddr:  net.IPv6loopback,
		})
		if errors.Is(err, unix.EMSGSIZE) {
			// FIXME(robmry) - if EMSGSIZE is returned (why?), it seems to be persistent.
			//  So, skip the delay and continue to the NA send as it seems to succeed.
			log.G(ctx).Info("Skipping check for route to send NA, EMSGSIZE")
			return true
		}
		if err != nil || len(routes) == 0 {
			log.G(ctx).WithFields(log.Fields{"error": err, "nroutes": len(routes)}).Info("Waiting for route to send NA")
			time.Sleep(pollInterval)
			continue
		}
		return true
	}
	log.G(ctx).WithField("", maxWait).Warn("No route for neighbour advertisement")
	return false
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
func (n *Namespace) advertiseAddrs(ctx context.Context, ifIndex int, i *Interface, nlh nlwrap.Handle, mcastRouteOk bool) error {
	ctx, span := otel.Tracer("").Start(ctx, "libnetwork.osl.advertiseAddrs.initial")
	defer span.End()

	mac := i.MacAddress()
	address4 := i.Address()
	address6 := i.AddressIPv6()
	ctx = log.WithLogger(ctx, log.G(ctx).WithFields(log.Fields{
		"iface":        i.dstName,
		"ifi":          ifIndex,
		"mac":          mac.String(),
		"ip4":          address4,
		"ip6":          address6,
		"mcastRouteOk": mcastRouteOk,
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
			if err := naSender.Send(); err != nil {
				log.G(ctx).WithError(err).Warn("Failed to send unsolicited NA")
				// If there was no multicast route and the network is unreachable, ignore the
				// error - this happens when a macvlan's parent interface is down.
				if mcastRouteOk || !errors.Is(err, unix.ENETUNREACH) {
					errs = append(errs, err)
				}
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
		ctx, span := otel.Tracer("").Start(trace.ContextWithSpanContext(context.WithoutCancel(ctx), trace.SpanContext{}),
			"libnetwork.osl.advertiseAddrs.subsequent",
			trace.WithLinks(trace.LinkFromContext(ctx)))
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

	// Find the network interface identified by the DstName attribute.
	iface, err := n.nlHandle.LinkByName(i.DstName())
	if err != nil {
		return err
	}

	// Down the interface before configuring
	if err := n.nlHandle.LinkSetDown(iface); err != nil {
		return err
	}

	// TODO(aker): Why are we doing this? This would fail if the initial interface set up failed before the "dest interface" was moved into its own namespace; see https://github.com/moby/moby/pull/46315/commits/108595c2fe852a5264b78e96f9e63cda284990a6#r1331253578
	err = n.nlHandle.LinkSetName(iface, i.SrcName())
	if err != nil {
		log.G(context.TODO()).Debugf("LinkSetName failed for interface %s: %v", i.SrcName(), err)
		return err
	}

	// if it is a bridge just delete it.
	if i.Bridge() {
		if err := n.nlHandle.LinkDel(iface); err != nil {
			return fmt.Errorf("failed deleting bridge %q: %v", i.SrcName(), err)
		}
	} else if !n.isDefault {
		// Move the network interface to caller namespace.
		// TODO(aker): What's this really doing? There are no calls to LinkDel in this package: is this code really used? (Interface.Remove() has 3 callers); see https://github.com/moby/moby/pull/46315/commits/108595c2fe852a5264b78e96f9e63cda284990a6#r1331265335
		if err := n.nlHandle.LinkSetNsFd(iface, ns.ParseHandlerInt()); err != nil {
			log.G(context.TODO()).Debugf("LinkSetNsFd failed for interface %s: %v", i.SrcName(), err)
			return err
		}
	}

	n.mu.Lock()
	n.removeInterface(i)
	n.mu.Unlock()

	return nil
}

func (n *Namespace) removeInterface(i *Interface) {
	n.iFaces = slices.DeleteFunc(n.iFaces, func(iface *Interface) bool {
		return iface == i
	})
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
