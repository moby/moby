// FIXME(thaJeztah): remove once we are a module; the go:build directive prevents go from downgrading language version to go1.16:
//go:build go1.23

package bridge

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"os"
	"slices"
	"strconv"
	"syscall"

	"github.com/containerd/log"
	"github.com/docker/docker/daemon/libnetwork/drivers/bridge/internal/firewaller"
	"github.com/docker/docker/daemon/libnetwork/drivers/bridge/internal/rlkclient"
	"github.com/docker/docker/daemon/libnetwork/netutils"
	"github.com/docker/docker/daemon/libnetwork/portallocator"
	"github.com/docker/docker/daemon/libnetwork/portmapper"
	"github.com/docker/docker/daemon/libnetwork/portmapperapi"
	"github.com/docker/docker/daemon/libnetwork/types"
)

// Allow unit tests to supply a dummy StartProxy.
var startProxy = portmapper.StartProxy

// addPortMappings takes cfg, the configuration for port mappings, selects host
// ports when ranges are given, binds host ports to check they're available and
// reserve them, starts docker-proxy if required, and sets up iptables
// NAT/forwarding rules as necessary. If anything goes wrong, it will undo any
// work it's done and return an error. Otherwise, the returned slice of
// PortBinding has an entry per address family (if cfg describes a mapping for
// 'any' host address, it's expanded into mappings for IPv4 and IPv6, because
// that's how the mapping is presented in 'inspect'). HostPort and HostPortEnd in
// each returned PortBinding are set to the selected and reserved port.
func (n *bridgeNetwork) addPortMappings(
	ctx context.Context,
	ep *bridgeEndpoint,
	cfg []portmapperapi.PortBindingReq,
	defHostIP net.IP,
	pbmReq portBindingMode,
) (_ []portmapperapi.PortBinding, retErr error) {
	if len(defHostIP) == 0 {
		defHostIP = net.IPv4zero
	} else if addr4 := defHostIP.To4(); addr4 != nil {
		// Unmap the address if it's IPv4-mapped IPv6.
		defHostIP = addr4
	}

	bindings := make([]portmapperapi.PortBinding, 0, len(cfg)*2)
	defer func() {
		if retErr != nil {
			if err := releasePortBindings(bindings, n.firewallerNetwork); err != nil {
				log.G(ctx).Warnf("Release port bindings: %s", err.Error())
			}
		}
	}()

	bindingReqs := n.sortAndNormPBs(ctx, ep, cfg, defHostIP, pbmReq)

	proxyPath := n.userlandProxyPath()
	pdc := n.getPortDriverClient()

	// toBind accumulates port bindings that should be allocated the same host port
	// (if required by NAT config). If the host address is unspecified, and defHostIP
	// is 0.0.0.0, one iteration of the loop may generate bindings for v4 and v6. If
	// a host address is specified, it'll either be IPv4 or IPv6, and only one
	// binding will be added per iteration. Config for bindings that only differ in
	// host IP are sorted next to each other, the loop continues until toBind has
	// collected them all, for both v4 and v6. The addresses may be 0.0.0.0 and [::],
	// or multiple addresses of both address families. Once there are no more
	// bindings to collect, they're applied and toBind is reset.
	var toBind []portmapperapi.PortBindingReq
	for i, c := range bindingReqs {
		toBind = append(toBind, c)
		if i < len(bindingReqs)-1 && c.DisableNAT == bindingReqs[i+1].DisableNAT && needSamePort(c, bindingReqs[i+1]) {
			// This port binding matches the next, apart from host IP. So, continue
			// collecting bindings, then allocate the same host port for all addresses.
			continue
		}

		var newB []portmapperapi.PortBinding
		var err error
		if c.DisableNAT {
			newB, err = setupForwardedPorts(ctx, toBind, n.firewallerNetwork)
		} else {
			newB, err = bindHostPorts(ctx, toBind, proxyPath, pdc, n.firewallerNetwork)
		}
		if err != nil {
			return nil, err
		}
		bindings = append(bindings, newB...)

		// Reset toBind now the ports are bound.
		toBind = toBind[:0]
	}

	// Start userland proxy processes.
	if proxyPath != "" {
		for i := range bindings {
			if bindings[i].BoundSocket == nil || bindings[i].RootlesskitUnsupported || bindings[i].StopProxy != nil {
				continue
			}
			var err error
			bindings[i].StopProxy, err = startProxy(
				bindings[i].ChildPortBinding(), proxyPath, bindings[i].BoundSocket,
			)
			if err != nil {
				return nil, fmt.Errorf("failed to start userland proxy for port mapping %s: %w",
					bindings[i].PortBinding, err)
			}
			if err := bindings[i].BoundSocket.Close(); err != nil {
				log.G(ctx).WithFields(log.Fields{
					"error":   err,
					"mapping": bindings[i].PortBinding,
				}).Warnf("failed to close proxy socket")
			}
			bindings[i].BoundSocket = nil
		}
	}

	return bindings, nil
}

// sortAndNormPBs transforms cfg into a list of portBindingReq, with all fields
// normalized:
//
//   - HostPortEnd=HostPort (rather than 0) if the host port isn't a range
//   - HostIP is set to the default host IP if not specified, and the binding is
//     NATed
//   - DisableNAT is set if the binding is routed, and HostIP is cleared
//
// When no HostIP is specified, and the default HostIP is 0.0.0.0, a duplicate
// IPv6 port binding is created with the same port and protocol, but with
// HostIP set to [::].
//
// Finally, port bindings are sorted into the ordering defined by cmpPortBindingReqs
// in order to form groups of port bindings that should be processed in one go.
func (n *bridgeNetwork) sortAndNormPBs(
	ctx context.Context,
	ep *bridgeEndpoint,
	cfg []portmapperapi.PortBindingReq,
	defHostIP net.IP,
	pbmReq portBindingMode,
) []portmapperapi.PortBindingReq {
	var containerIPv4, containerIPv6 net.IP
	if ep.addr != nil {
		containerIPv4 = ep.addr.IP
	}
	if ep.addrv6 != nil {
		containerIPv6 = ep.addrv6.IP
	}

	proxyPath := n.userlandProxyPath()
	pdc := n.getPortDriverClient()
	disableNAT4, disableNAT6 := n.getNATDisabled()

	add4 := !ep.portBindingState.ipv4 && pbmReq.ipv4
	add6 := !ep.portBindingState.ipv6 && pbmReq.ipv6

	reqs := make([]portmapperapi.PortBindingReq, 0, len(cfg))
	for _, c := range cfg {
		if c.HostPortEnd == 0 {
			c.HostPortEnd = c.HostPort
		}

		if add4 {
			if bindingIPv4, ok := configurePortBindingIPv4(ctx, pdc, disableNAT4, c, containerIPv4, defHostIP); ok {
				reqs = append(reqs, bindingIPv4)
			}
		}

		// If the container has no IPv6 address, allow proxying host IPv6 traffic to it
		// by setting up the binding with the IPv4 interface if the userland proxy is enabled
		// This change was added to keep backward compatibility
		containerIP := containerIPv6
		if containerIPv6 == nil && pbmReq.ipv4 && add6 {
			if proxyPath == "" {
				// There's no way to map from host-IPv6 to container-IPv4 with the userland proxy
				// disabled.
				// If that is required, don't treat it as an error because, as networks are
				// connected/disconnected, the container's gateway endpoint might change to a
				// network where this config makes more sense.
				if len(c.HostIP) > 0 && c.HostIP.To4() == nil {
					log.G(ctx).WithFields(log.Fields{"mapping": c}).Info(
						"Cannot map from IPv6 to an IPv4-only container because the userland proxy is disabled")
				}
				if len(c.HostIP) == 0 && defHostIP.To4() == nil {
					log.G(ctx).WithFields(log.Fields{
						"mapping": c,
						"default": defHostIP,
					}).Info("Cannot map from default host binding address to an IPv4-only container because the userland proxy is disabled")
				}
			} else {
				containerIP = containerIPv4
			}
		}
		if add6 {
			if bindingIPv6, ok := configurePortBindingIPv6(ctx, pdc, disableNAT6, c, containerIP, defHostIP); ok {
				reqs = append(reqs, bindingIPv6)
			}
		}
	}
	slices.SortFunc(reqs, cmpPortBindingReqs)
	return reqs
}

// cmpPortBindingReqs defines an ordering over PortBinding such that bindings that differ
// only in host IP are adjacent (those bindings should be allocated the same port).
//
// Port bindings are first sorted by their mapper, then:
//   - exact host ports are placed before ranges (in case exact ports fall within
//     ranges, giving a better chance of allocating the exact ports), then
//   - same container port are adjacent (lowest ports first), then
//   - same protocols are adjacent (tcp < udp < sctp), then
//   - same host ports or ranges are adjacent, then
//   - ordered by container IP (then host IP, if set).
func cmpPortBindingReqs(a, b portmapperapi.PortBindingReq) int {
	if a.DisableNAT != b.DisableNAT {
		if a.DisableNAT {
			return 1 // NAT disabled bindings come last
		}
		return -1
	}
	// Exact host port < host port range.
	aIsRange := a.HostPort == 0 || a.HostPort != a.HostPortEnd
	bIsRange := b.HostPort == 0 || b.HostPort != b.HostPortEnd
	if aIsRange != bIsRange {
		if aIsRange {
			return 1
		}
		return -1
	}
	if a.Port != b.Port {
		return int(a.Port) - int(b.Port)
	}
	if a.Proto != b.Proto {
		return int(a.Proto) - int(b.Proto)
	}
	if a.HostPort != b.HostPort {
		return int(a.HostPort) - int(b.HostPort)
	}
	if a.HostPortEnd != b.HostPortEnd {
		return int(a.HostPortEnd) - int(b.HostPortEnd)
	}
	aHostIP, _ := netip.AddrFromSlice(a.HostIP)
	bHostIP, _ := netip.AddrFromSlice(b.HostIP)
	if c := aHostIP.Unmap().Compare(bHostIP.Unmap()); c != 0 {
		return c
	}
	aIP, _ := netip.AddrFromSlice(a.IP)
	bIP, _ := netip.AddrFromSlice(b.IP)
	return aIP.Unmap().Compare(bIP.Unmap())
}

// needSamePort returns true iff a and b only differ in the host IP address,
// meaning they should be allocated the same host port (so that, if v4/v6
// addresses are returned in a DNS response or similar, clients can bind without
// needing to adjust the port number depending on which address is used).
func needSamePort(a, b portmapperapi.PortBindingReq) bool {
	return a.Port == b.Port &&
		a.Proto == b.Proto &&
		a.HostPort == b.HostPort &&
		a.HostPortEnd == b.HostPortEnd
}

// mergeChildHostIPs take a slice of PortBinding and returns a slice of
// types.PortBinding, where the HostIP in each of the results has the
// value of ChildHostIP from the input (if present).
func mergeChildHostIPs(pbs []portmapperapi.PortBinding) []types.PortBinding {
	res := make([]types.PortBinding, 0, len(pbs))
	for _, b := range pbs {
		pb := b.PortBinding
		if b.ChildHostIP != nil {
			pb.HostIP = b.ChildHostIP
		}
		res = append(res, pb)
	}
	return res
}

// configurePortBindingIPv4 returns a new port binding with the HostIP field
// populated and true, if a binding is required. Else, false and an empty
// binding.
func configurePortBindingIPv4(
	ctx context.Context,
	pdc portDriverClient,
	disableNAT bool,
	bnd portmapperapi.PortBindingReq,
	containerIPv4,
	defHostIP net.IP,
) (portmapperapi.PortBindingReq, bool) {
	if len(containerIPv4) == 0 {
		return portmapperapi.PortBindingReq{}, false
	}
	if len(bnd.HostIP) > 0 && bnd.HostIP.To4() == nil {
		// The mapping is explicitly IPv6.
		return portmapperapi.PortBindingReq{}, false
	}
	// If there's no host address, use the default.
	if len(bnd.HostIP) == 0 {
		if defHostIP.To4() == nil {
			// The default binding address is IPv6.
			return portmapperapi.PortBindingReq{}, false
		}
		// The default binding IP is an IPv4 address, use it - unless NAT is disabled,
		// in which case it's not possible to bind to a specific host address (the port
		// mapping only opens the container's port for direct routing).
		if disableNAT {
			bnd.HostIP = net.IPv4zero
		} else {
			bnd.HostIP = defHostIP
		}
	}

	if disableNAT && len(bnd.HostIP) != 0 && !bnd.HostIP.Equal(net.IPv4zero) {
		// Ignore the default binding when nat is disabled - it may have been set
		// up for IPv6 if nat is enabled there.
		// Don't treat this as an error because, as networks are connected/disconnected,
		// the container's gateway endpoint might change to a network where this config
		// makes more sense.
		log.G(ctx).WithFields(log.Fields{"mapping": bnd}).Info(
			"Using address 0.0.0.0 because NAT is disabled")
		bnd.HostIP = net.IPv4zero
	}

	// Unmap the addresses if they're IPv4-mapped IPv6.
	bnd.HostIP = bnd.HostIP.To4()
	bnd.IP = containerIPv4.To4()
	bnd.DisableNAT = disableNAT
	return setChildHostIP(pdc, bnd), true
}

// configurePortBindingIPv6 returns a new port binding with the HostIP field
// populated and true, if a binding is required. Else, false and an empty
// binding.
func configurePortBindingIPv6(
	ctx context.Context,
	pdc portDriverClient,
	disableNAT bool,
	bnd portmapperapi.PortBindingReq,
	containerIP, defHostIP net.IP,
) (portmapperapi.PortBindingReq, bool) {
	if containerIP == nil {
		return portmapperapi.PortBindingReq{}, false
	}
	if len(bnd.HostIP) > 0 && bnd.HostIP.To4() != nil {
		// The mapping is explicitly IPv4.
		return portmapperapi.PortBindingReq{}, false
	}

	// If there's no host address, use the default.
	if len(bnd.HostIP) == 0 {
		if defHostIP.Equal(net.IPv4zero) {
			if !netutils.IsV6Listenable() {
				// No implicit binding if the host has no IPv6 support.
				return portmapperapi.PortBindingReq{}, false
			}
			// Implicit binding to "::", no explicit HostIP and the default is 0.0.0.0
			bnd.HostIP = net.IPv6zero
		} else if defHostIP.To4() == nil {
			// The default binding IP is an IPv6 address, use it - unless NAT is disabled, in
			// which case it's not possible to bind to a specific host address (the port
			// mapping only opens the container's port for direct routing).
			if disableNAT {
				bnd.HostIP = net.IPv6zero
			} else {
				bnd.HostIP = defHostIP
			}
		} else {
			// The default binding IP is an IPv4 address, nothing to do here.
			return portmapperapi.PortBindingReq{}, false
		}
	}

	if disableNAT && len(bnd.HostIP) != 0 && !bnd.HostIP.Equal(net.IPv6zero) {
		// Ignore the default binding when nat is disabled - it may have been set
		// up for IPv4 if nat is enabled there.
		// Don't treat this as an error because, as networks are connected/disconnected,
		// the container's gateway endpoint might change to a network where this config
		// makes more sense.
		log.G(ctx).WithFields(log.Fields{"mapping": bnd}).Info(
			"Using address [::] because NAT is disabled")
		bnd.HostIP = net.IPv6zero
	}

	bnd.IP = containerIP
	bnd.DisableNAT = disableNAT
	return setChildHostIP(pdc, bnd), true
}

func setChildHostIP(pdc portDriverClient, req portmapperapi.PortBindingReq) portmapperapi.PortBindingReq {
	if pdc == nil {
		req.ChildHostIP = req.HostIP
		return req
	}
	hip, _ := netip.AddrFromSlice(req.HostIP)
	req.ChildHostIP = pdc.ChildHostIP(hip).AsSlice()
	return req
}

// setupForwardedPorts sets up firewall rules to allow direct remote access to
// the container's ports in cfg.
func setupForwardedPorts(ctx context.Context, cfg []portmapperapi.PortBindingReq, fwn firewaller.Network) ([]portmapperapi.PortBinding, error) {
	if len(cfg) == 0 {
		return nil, nil
	}

	res := make([]portmapperapi.PortBinding, 0, len(cfg))
	bindings := make([]types.PortBinding, 0, len(cfg))
	for _, c := range cfg {
		pb := portmapperapi.PortBinding{PortBinding: c.GetCopy()}
		if pb.HostPort != 0 || pb.HostPortEnd != 0 {
			log.G(ctx).WithFields(log.Fields{"mapping": pb}).Infof(
				"Host port ignored, because NAT is disabled")
			pb.HostPort = 0
			pb.HostPortEnd = 0
		}
		res = append(res, pb)
		bindings = append(bindings, pb.PortBinding)
	}

	if err := fwn.AddPorts(ctx, bindings); err != nil {
		return nil, err
	}

	return res, nil
}

// bindHostPorts allocates and binds host ports for the given cfg. The
// caller is responsible for ensuring that all entries in cfg map the same proto,
// container port, and host port range (their host addresses must differ).
func bindHostPorts(
	ctx context.Context,
	cfg []portmapperapi.PortBindingReq,
	proxyPath string,
	pdc portDriverClient,
	fwn firewaller.Network,
) ([]portmapperapi.PortBinding, error) {
	if len(cfg) == 0 {
		return nil, nil
	}
	// Ensure that all of cfg's entries have the same proto and ports.
	proto, port, hostPort, hostPortEnd := cfg[0].Proto, cfg[0].Port, cfg[0].HostPort, cfg[0].HostPortEnd
	for _, c := range cfg[1:] {
		if c.Proto != proto || c.Port != port || c.HostPort != hostPort || c.HostPortEnd != hostPortEnd {
			return nil, types.InternalErrorf("port binding mismatch %d/%s:%d-%d, %d/%s:%d-%d",
				port, proto, hostPort, hostPortEnd,
				port, c.Proto, c.HostPort, c.HostPortEnd)
		}
	}

	// Try up to maxAllocatePortAttempts times to get a port that's not already allocated.
	var err error
	for i := 0; i < maxAllocatePortAttempts; i++ {
		var b []portmapperapi.PortBinding
		b, err = attemptBindHostPorts(ctx, cfg, proto, hostPort, hostPortEnd, proxyPath, pdc, fwn)
		if err == nil {
			return b, nil
		}
		// There is no point in immediately retrying to map an explicitly chosen port.
		if hostPort != 0 && hostPort == hostPortEnd {
			log.G(ctx).WithError(err).Warnf("Failed to allocate and map port")
			break
		}
		log.G(ctx).WithFields(log.Fields{
			"error":   err,
			"attempt": i + 1,
		}).Warn("Failed to allocate and map port")
	}
	return nil, err
}

// attemptBindHostPorts allocates host ports for each NAT port mapping, and
// reserves those ports by binding them.
//
// If the allocator doesn't have an available port in the required range, or the
// port can't be bound (perhaps because another process has already bound it),
// all resources are released and an error is returned. When ports are
// successfully reserved, a PortBinding is returned for each mapping.
func attemptBindHostPorts(
	ctx context.Context,
	cfg []portmapperapi.PortBindingReq,
	proto types.Protocol,
	hostPortStart, hostPortEnd uint16,
	proxyPath string,
	pdc portDriverClient,
	fwn firewaller.Network,
) (_ []portmapperapi.PortBinding, retErr error) {
	var err error
	var port int

	addrs := make([]net.IP, 0, len(cfg))
	for _, c := range cfg {
		addrs = append(addrs, c.ChildHostIP)
	}

	pa := portallocator.NewOSAllocator()
	port, socks, err := pa.RequestPortsInRange(addrs, proto, int(hostPortStart), int(hostPortEnd))
	if err != nil {
		return nil, err
	}
	defer func() {
		if retErr != nil {
			pa.ReleasePorts(addrs, proto, port)
		}
	}()

	if len(socks) != len(cfg) {
		for _, sock := range socks {
			if err := sock.Close(); err != nil {
				log.G(ctx).WithError(err).Warn("Failed to close socket")
			}
		}
		return nil, types.InternalErrorf("port allocator returned %d sockets for %d port bindings", len(socks), len(cfg))
	}

	res := make([]portmapperapi.PortBinding, 0, len(cfg))
	defer func() {
		if retErr != nil {
			if err := releasePortBindings(res, fwn); err != nil {
				log.G(ctx).WithError(err).Warn("Failed to release port bindings")
			}
		}
	}()

	for i := range cfg {
		pb := portmapperapi.PortBinding{
			PortBinding: cfg[i].PortBinding.GetCopy(),
			BoundSocket: socks[i],
			ChildHostIP: cfg[i].ChildHostIP,
		}
		pb.PortBinding.HostPort = uint16(port)
		pb.PortBinding.HostPortEnd = pb.HostPort
		res = append(res, pb)
	}

	if err := configPortDriver(ctx, res, pdc); err != nil {
		return nil, err
	}
	if err := fwn.AddPorts(ctx, mergeChildHostIPs(res)); err != nil {
		return nil, err
	}
	// Now the firewall rules are set up, it's safe to listen on the socket. (Listening
	// earlier could result in dropped connections if the proxy becomes unreachable due
	// to NAT rules sending packets directly to the container.)
	//
	// If not starting the proxy, nothing will ever accept a connection on the
	// socket. Listen here anyway because SO_REUSEADDR is set, so bind() won't notice
	// the problem if a port's bound to both INADDR_ANY and a specific address. (Also
	// so the binding shows up in "netstat -at".)
	if err := listenBoundPorts(res, proxyPath); err != nil {
		return nil, err
	}
	return res, nil
}

// configPortDriver passes the port binding's details to rootlesskit, and updates the
// port binding with callbacks to remove the rootlesskit config (or marks the binding as
// unsupported by rootlesskit).
func configPortDriver(ctx context.Context, pbs []portmapperapi.PortBinding, pdc portDriverClient) error {
	for i := range pbs {
		b := pbs[i]
		if pdc != nil && b.HostPort != 0 {
			var err error
			hip, ok := netip.AddrFromSlice(b.HostIP)
			if !ok {
				return fmt.Errorf("invalid host IP address in %s", b)
			}
			chip, ok := netip.AddrFromSlice(b.ChildHostIP)
			if !ok {
				return fmt.Errorf("invalid child host IP address %s in %s", b.ChildHostIP, b)
			}
			pbs[i].PortDriverRemove, err = pdc.AddPort(ctx, b.Proto.String(), hip, chip, int(b.HostPort))
			if err != nil {
				var pErr *rlkclient.ProtocolUnsupportedError
				if errors.As(err, &pErr) {
					log.G(ctx).WithFields(log.Fields{
						"error": pErr,
					}).Warnf("discarding request for %q", net.JoinHostPort(hip.String(), strconv.Itoa(int(b.HostPort))))
					pbs[i].RootlesskitUnsupported = true
					continue
				}
				return err
			}
		}
	}
	return nil
}

func listenBoundPorts(pbs []portmapperapi.PortBinding, proxyPath string) error {
	for i := range pbs {
		if pbs[i].BoundSocket == nil || pbs[i].RootlesskitUnsupported || pbs[i].Proto == types.UDP {
			continue
		}
		rc, err := pbs[i].BoundSocket.SyscallConn()
		if err != nil {
			return fmt.Errorf("raw conn not available on %s socket: %w", pbs[i].Proto, err)
		}
		if errC := rc.Control(func(fd uintptr) {
			somaxconn := 0
			// SCTP sockets do not support somaxconn=0
			if proxyPath != "" || pbs[i].Proto == types.SCTP {
				somaxconn = -1 // silently capped to "/proc/sys/net/core/somaxconn"
			}
			err = syscall.Listen(int(fd), somaxconn)
		}); errC != nil {
			return fmt.Errorf("failed to Control %s socket: %w", pbs[i].Proto, err)
		}
		if err != nil {
			return fmt.Errorf("failed to listen on %s socket: %w", pbs[i].Proto, err)
		}
	}
	return nil
}

// releasePorts attempts to release all port bindings, does not stop on failure
func (n *bridgeNetwork) releasePorts(ep *bridgeEndpoint) error {
	n.Lock()
	pbs := ep.portMapping
	ep.portMapping = nil
	ep.portBindingState = portBindingMode{}
	n.Unlock()

	return releasePortBindings(pbs, n.firewallerNetwork)
}

func releasePortBindings(pbs []portmapperapi.PortBinding, fwn firewaller.Network) error {
	var errs []error
	for _, pb := range pbs {
		if pb.BoundSocket != nil {
			if err := pb.BoundSocket.Close(); err != nil {
				errs = append(errs, fmt.Errorf("failed to close socket for port mapping %s: %w", pb, err))
			}
		}
		if pb.PortDriverRemove != nil {
			if err := pb.PortDriverRemove(); err != nil {
				errs = append(errs, err)
			}
		}
		if pb.StopProxy != nil {
			if err := pb.StopProxy(); err != nil && !errors.Is(err, os.ErrProcessDone) {
				errs = append(errs, fmt.Errorf("failed to stop userland proxy for port mapping %s: %w", pb, err))
			}
		}
	}
	if err := fwn.DelPorts(context.TODO(), mergeChildHostIPs(pbs)); err != nil {
		errs = append(errs, err)
	}
	for _, pb := range pbs {
		if pb.HostPort > 0 {
			portallocator.Get().ReleasePort(pb.ChildHostIP, pb.Proto.String(), int(pb.HostPort))
		}
	}
	return errors.Join(errs...)
}

func (n *bridgeNetwork) reapplyPerPortIptables() {
	n.Lock()
	var allPBs []portmapperapi.PortBinding
	for _, ep := range n.endpoints {
		allPBs = append(allPBs, ep.portMapping...)
	}
	n.Unlock()

	if err := n.firewallerNetwork.AddPorts(context.Background(), mergeChildHostIPs(allPBs)); err != nil {
		log.G(context.TODO()).Warnf("Failed to reconfigure NAT: %s", err)
	}
}
