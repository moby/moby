// FIXME(thaJeztah): remove once we are a module; the go:build directive prevents go from downgrading language version to go1.16:
//go:build go1.22

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
	"unsafe"

	"github.com/containerd/log"
	"github.com/docker/docker/libnetwork/drivers/bridge/internal/rlkclient"
	"github.com/docker/docker/libnetwork/iptables"
	"github.com/docker/docker/libnetwork/netutils"
	"github.com/docker/docker/libnetwork/portallocator"
	"github.com/docker/docker/libnetwork/portmapper"
	"github.com/docker/docker/libnetwork/types"
	"github.com/ishidawataru/sctp"
)

type portBinding struct {
	types.PortBinding
	// boundSocket is used to reserve a host port for the binding. If the
	// userland proxy is in-use, it's passed to the proxy when the proxy is
	// started, then it's closed and set to nil here.
	boundSocket *os.File
	// childHostIP is the host IP address, as seen from the daemon. This
	// is normally the same as PortBinding.HostIP but, in rootless mode, it
	// will be an address in the rootless network namespace. RootlessKit
	// binds the port on the real (parent) host address and maps it to the
	// same port number on the address dockerd sees in the child namespace.
	// So, for example, docker-proxy and DNAT rules need to use the child
	// namespace's host address. (PortBinding.HostIP isn't replaced by the
	// child address, because it's stored as user-config and the child
	// address may change if RootlessKit is configured differently.)
	childHostIP net.IP
	// portDriverRemove is a function that will inform the RootlessKit
	// port driver about removal of a port binding, or nil.
	portDriverRemove func() error
	// stopProxy is a function to stop the userland proxy for this binding,
	// if a proxy has been started - else nil.
	stopProxy func() error
	// rootlesskitUnsupported is set to true when the port binding is not
	// supported by the port driver of RootlessKit.
	rootlesskitUnsupported bool
}

// childPortBinding is pb.PortBinding, with the host address the daemon
// will see - which, in rootless mode, will be an address in the RootlessKit's
// child namespace (see portBinding.childHostIP).
func (pb portBinding) childPortBinding() types.PortBinding {
	res := pb.PortBinding
	res.HostIP = pb.childHostIP
	return res
}

type portBindingReq struct {
	types.PortBinding
	childHostIP net.IP
	disableNAT  bool
}

// Allow unit tests to supply a dummy StartProxy.
var startProxy = portmapper.StartProxy

// addPortMappings takes cfg, the configuration for port mappings, selects host
// ports when ranges are given, binds host ports to check they're available and
// reserve them, starts docker-proxy if required, and sets up iptables
// NAT/forwarding rules as necessary. If anything goes wrong, it will undo any
// work it's done and return an error. Otherwise, the returned slice of
// portBinding has an entry per address family (if cfg describes a mapping for
// 'any' host address, it's expanded into mappings for IPv4 and IPv6, because
// that's how the mapping is presented in 'inspect'). HostPort and HostPortEnd in
// each returned portBinding are set to the selected and reserved port.
func (n *bridgeNetwork) addPortMappings(
	ctx context.Context,
	epAddrV4, epAddrV6 *net.IPNet,
	cfg []types.PortBinding,
	defHostIP net.IP,
	noProxy6To4 bool,
) (_ []portBinding, retErr error) {
	if len(defHostIP) == 0 {
		defHostIP = net.IPv4zero
	} else if addr4 := defHostIP.To4(); addr4 != nil {
		// Unmap the address if it's IPv4-mapped IPv6.
		defHostIP = addr4
	}

	var containerIPv4, containerIPv6 net.IP
	if epAddrV4 != nil {
		containerIPv4 = epAddrV4.IP
	}
	if epAddrV6 != nil {
		containerIPv6 = epAddrV6.IP
	}

	bindings := make([]portBinding, 0, len(cfg)*2)

	defer func() {
		if retErr != nil {
			if err := n.releasePortBindings(bindings); err != nil {
				log.G(ctx).Warnf("Release port bindings: %s", err.Error())
			}
		}
	}()

	sortedCfg := slices.Clone(cfg)
	sortAndNormPBs(sortedCfg)

	proxyPath := n.userlandProxyPath()
	pdc := n.getPortDriverClient()
	disableNAT4, disableNAT6 := n.getNATDisabled()

	// toBind accumulates port bindings that should be allocated the same host port
	// (if required by NAT config). If the host address is unspecified, and defHostIP
	// is 0.0.0.0, one iteration of the loop may generate bindings for v4 and v6. If
	// a host address is specified, it'll either be IPv4 or IPv6, and only one
	// binding will be added per iteration. Config for bindings that only differ in
	// host IP are sorted next to each other, the loop continues until toBind has
	// collected them all, for both v4 and v6. The addresses may be 0.0.0.0 and [::],
	// or multiple addresses of both address families. Once there are no more
	// bindings to collect, they're applied and toBind is reset.
	var toBind []portBindingReq
	for i, c := range sortedCfg {
		if bindingIPv4, ok := configurePortBindingIPv4(ctx, pdc, disableNAT4, c, containerIPv4, defHostIP); ok {
			toBind = append(toBind, bindingIPv4)
		}

		// If the container has no IPv6 address, allow proxying host IPv6 traffic to it
		// by setting up the binding with the IPv4 interface if the userland proxy is enabled
		// This change was added to keep backward compatibility
		containerIP := containerIPv6
		if containerIPv6 == nil && !noProxy6To4 {
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
		if bindingIPv6, ok := configurePortBindingIPv6(ctx, pdc, disableNAT6, c, containerIP, defHostIP); ok {
			toBind = append(toBind, bindingIPv6)
		}

		if i < len(sortedCfg)-1 && needSamePort(c, sortedCfg[i+1]) {
			// This port binding matches the next, apart from host IP. So, continue
			// collecting bindings, then allocate the same host port for all addresses.
			continue
		}

		// Allocate and bind a host port.
		newB, err := bindHostPorts(ctx, toBind, proxyPath)
		if err != nil {
			return nil, err
		}
		bindings = append(bindings, newB...)

		// Reset toBind now the ports are bound.
		toBind = toBind[:0]
	}

	for i := range bindings {
		b := bindings[i]
		if pdc != nil && b.HostPort != 0 {
			var err error
			hip, ok := netip.AddrFromSlice(b.HostIP)
			if !ok {
				return nil, fmt.Errorf("invalid host IP address in %s", b)
			}
			chip, ok := netip.AddrFromSlice(b.childHostIP)
			if !ok {
				return nil, fmt.Errorf("invalid child host IP address %s in %s", b.childHostIP, b)
			}
			bindings[i].portDriverRemove, err = pdc.AddPort(ctx, b.Proto.String(), hip, chip, int(b.HostPort))
			if err != nil {
				var pErr *rlkclient.ProtocolUnsupportedError
				if errors.As(err, &pErr) {
					log.G(ctx).WithFields(log.Fields{
						"error": pErr,
					}).Warnf("discarding request for %q", net.JoinHostPort(hip.String(), strconv.Itoa(int(b.HostPort))))
					bindings[i].rootlesskitUnsupported = true
					continue
				}
				return nil, err
			}
		}
		if err := n.setPerPortIptables(b, true); err != nil {
			return nil, err
		}
	}

	// Now the iptables rules are set up, it's safe to start the userland proxy.
	// (If it was started before the iptables rules were created, it may have
	// accepted a connection, then become unreachable due to NAT rules sending
	// packets directly to the container.)
	// If not starting the proxy, nothing will ever accept a connection on the
	// socket. But, listen anyway so that the binding shows up in "netstat -at".
	somaxconn := 0
	if proxyPath != "" {
		somaxconn = -1 // silently capped to "/proc/sys/net/core/somaxconn"
	}
	for i := range bindings {
		if bindings[i].boundSocket == nil || bindings[i].rootlesskitUnsupported {
			continue
		}
		if bindings[i].Proto == types.TCP {
			rc, err := bindings[i].boundSocket.SyscallConn()
			if err != nil {
				return nil, fmt.Errorf("raw conn not available on TCP socket: %w", err)
			}
			if errC := rc.Control(func(fd uintptr) {
				err = syscall.Listen(int(fd), somaxconn)
			}); errC != nil {
				return nil, fmt.Errorf("failed to Control TCP socket: %w", err)
			}
			if err != nil {
				return nil, fmt.Errorf("failed to listen on TCP socket: %w", err)
			}
		}
		if proxyPath != "" {
			var err error
			bindings[i].stopProxy, err = startProxy(
				bindings[i].childPortBinding(), proxyPath, bindings[i].boundSocket,
			)
			if err != nil {
				return nil, fmt.Errorf("failed to start userland proxy for port mapping %s: %w",
					bindings[i].PortBinding, err)
			}
			if err := bindings[i].boundSocket.Close(); err != nil {
				log.G(ctx).WithFields(log.Fields{
					"error":   err,
					"mapping": bindings[i].PortBinding,
				}).Warnf("failed to close proxy socket")
			}
			bindings[i].boundSocket = nil
		}
	}

	return bindings, nil
}

// sortAndNormPBs normalises cfg by making HostPortEnd=HostPort (rather than 0) if the
// host port isn't a range - and sorts it into the ordering defined by cmpPortBinding.
func sortAndNormPBs(cfg []types.PortBinding) {
	for i := range cfg {
		if cfg[i].HostPortEnd == 0 {
			cfg[i].HostPortEnd = cfg[i].HostPort
		}
	}
	slices.SortFunc(cfg, cmpPortBinding)
}

// cmpPortBinding defines an ordering over PortBinding such that bindings that differ
// only in host IP are adjacent (those bindings should be allocated the same port).
//
// Exact host ports are placed before ranges (in case exact ports fall within ranges,
// giving a better chance of allocating the exact ports), then PortBindings with the:
// - same container port are adjacent (lowest ports first), then
// - same protocols are adjacent (tcp < udp < sctp), then
// - same host ports or ranges are adjacent, then
// - ordered by container IP (then host IP, if set).
func cmpPortBinding(a, b types.PortBinding) int {
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
func needSamePort(a, b types.PortBinding) bool {
	return a.Port == b.Port &&
		a.Proto == b.Proto &&
		a.HostPort == b.HostPort &&
		a.HostPortEnd == b.HostPortEnd &&
		a.IP.Equal(b.IP)
}

// configurePortBindingIPv4 returns a new port binding with the HostIP field
// populated and true, if a binding is required. Else, false and an empty
// binding.
func configurePortBindingIPv4(
	ctx context.Context,
	pdc portDriverClient,
	disableNAT bool,
	bnd types.PortBinding,
	containerIPv4,
	defHostIP net.IP,
) (portBindingReq, bool) {
	if len(containerIPv4) == 0 {
		return portBindingReq{}, false
	}
	if len(bnd.HostIP) > 0 && bnd.HostIP.To4() == nil {
		// The mapping is explicitly IPv6.
		return portBindingReq{}, false
	}
	// If there's no host address, use the default.
	if len(bnd.HostIP) == 0 {
		if defHostIP.To4() == nil {
			// The default binding address is IPv6.
			return portBindingReq{}, false
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
	return setChildHostIP(pdc, portBindingReq{
		PortBinding: bnd,
		disableNAT:  disableNAT,
	}), true
}

// configurePortBindingIPv6 returns a new port binding with the HostIP field
// populated and true, if a binding is required. Else, false and an empty
// binding.
func configurePortBindingIPv6(
	ctx context.Context,
	pdc portDriverClient,
	disableNAT bool,
	bnd types.PortBinding,
	containerIP, defHostIP net.IP,
) (portBindingReq, bool) {
	if containerIP == nil {
		return portBindingReq{}, false
	}
	if len(bnd.HostIP) > 0 && bnd.HostIP.To4() != nil {
		// The mapping is explicitly IPv4.
		return portBindingReq{}, false
	}

	// If there's no host address, use the default.
	if len(bnd.HostIP) == 0 {
		if defHostIP.Equal(net.IPv4zero) {
			if !netutils.IsV6Listenable() {
				// No implicit binding if the host has no IPv6 support.
				return portBindingReq{}, false
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
			return portBindingReq{}, false
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
	return setChildHostIP(pdc, portBindingReq{
		PortBinding: bnd,
		disableNAT:  disableNAT,
	}), true
}

func setChildHostIP(pdc portDriverClient, req portBindingReq) portBindingReq {
	if pdc == nil {
		req.childHostIP = req.HostIP
		return req
	}
	hip, _ := netip.AddrFromSlice(req.HostIP)
	req.childHostIP = pdc.ChildHostIP(hip).AsSlice()
	return req
}

// bindHostPorts allocates and binds host ports for the given cfg. The
// caller is responsible for ensuring that all entries in cfg map the same proto,
// container port, and host port range (their host addresses must differ).
func bindHostPorts(
	ctx context.Context,
	cfg []portBindingReq,
	proxyPath string,
) ([]portBinding, error) {
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
		var b []portBinding
		b, err = attemptBindHostPorts(ctx, cfg, proto.String(), hostPort, hostPortEnd, proxyPath)
		if err == nil {
			return b, nil
		}
		// There is no point in immediately retrying to map an explicitly chosen port.
		if hostPort != 0 && hostPort == hostPortEnd {
			log.G(ctx).Warnf("Failed to allocate and map port: %s", err)
			break
		}
		log.G(ctx).Warnf("Failed to allocate and map port: %s, retry: %d", err, i+1)
	}
	return nil, err
}

// attemptBindHostPorts allocates host ports for each port mapping that requires
// one, and reserves those ports by binding them.
//
// If the allocator doesn't have an available port in the required range, or the
// port can't be bound (perhaps because another process has already bound it),
// all resources are released and an error is returned. When ports are
// successfully reserved, a portBinding is returned for each mapping.
//
// If NAT is disabled for any of the bindings, no host port reservation is
// needed. Include these bindings in results, as the container port itself
// needs to be opened in the firewall.
func attemptBindHostPorts(
	ctx context.Context,
	cfg []portBindingReq,
	proto string,
	hostPortStart, hostPortEnd uint16,
	proxyPath string,
) (_ []portBinding, retErr error) {
	var err error
	var port int

	addrs := make([]net.IP, 0, len(cfg))
	for _, c := range cfg {
		if !c.disableNAT {
			addrs = append(addrs, c.childHostIP)
		}
	}

	if len(addrs) > 0 {
		pa := portallocator.Get()
		port, err = pa.RequestPortsInRange(addrs, proto, int(hostPortStart), int(hostPortEnd))
		if err != nil {
			return nil, err
		}
		defer func() {
			if retErr != nil {
				for _, a := range addrs {
					pa.ReleasePort(a, proto, port)
				}
			}
		}()
	}

	res := make([]portBinding, 0, len(cfg))
	defer func() {
		if retErr != nil {
			for _, pb := range res {
				if pb.boundSocket != nil {
					if err := pb.boundSocket.Close(); err != nil {
						log.G(ctx).Warnf("Failed to close port binding for %s: %s", pb, err)
					}
				}
				// TODO(robmry) - this is only needed because the userland proxy may have
				//  been started for SCTP. If a bound socket is passed to the proxy after
				//  iptables rules have been configured (as it is for TCP/UDP), remove this.
				if pb.stopProxy != nil {
					if err := pb.stopProxy(); err != nil {
						log.G(ctx).Warnf("Failed to stop proxy for %s: %s", pb, err)
					}
				}
			}
		}
	}()

	for _, c := range cfg {
		var pb portBinding
		if c.disableNAT {
			pb = portBinding{PortBinding: c.GetCopy()}
			if pb.HostPort != 0 || pb.HostPortEnd != 0 {
				log.G(ctx).WithFields(log.Fields{"mapping": pb}).Infof(
					"Host port ignored, because NAT is disabled")
				pb.HostPort = 0
				pb.HostPortEnd = 0
			}
		} else {
			switch proto {
			case "tcp":
				pb, err = bindTCPOrUDP(c, port, syscall.SOCK_STREAM, syscall.IPPROTO_TCP)
			case "udp":
				pb, err = bindTCPOrUDP(c, port, syscall.SOCK_DGRAM, syscall.IPPROTO_UDP)
			case "sctp":
				if proxyPath == "" {
					pb, err = bindSCTP(c, port)
				} else {
					// TODO(robmry) - it's not currently possible to pass a bound SCTP port
					//  to the userland proxy, because the proxy is not able to convert the
					//  file descriptor into an sctp.SCTPListener (fd is an unexported member
					//  of the struct, and ListenSCTP is the only constructor).
					//  So, it is possible for the proxy to start listening and accept
					//  connections before iptables rules are created that would bypass
					//  the proxy for external connections.
					//  Remove this and pb.stopProxy() from the cleanup function above if
					//  this is fixed.
					pb, err = startSCTPProxy(c, port, proxyPath)
				}
			default:
				return nil, fmt.Errorf("Unknown addr type: %s", proto)
			}
			if err != nil {
				return nil, err
			}
		}
		res = append(res, pb)
	}
	return res, nil
}

func bindTCPOrUDP(cfg portBindingReq, port, typ, proto int) (_ portBinding, retErr error) {
	pb := portBinding{PortBinding: cfg.PortBinding.GetCopy()}
	pb.HostPort = uint16(port)
	pb.HostPortEnd = pb.HostPort
	pb.childHostIP = cfg.childHostIP

	var domain int
	var sa syscall.Sockaddr
	if hip := cfg.childHostIP.To4(); hip != nil {
		domain = syscall.AF_INET
		sa4 := syscall.SockaddrInet4{Port: port}
		copy(sa4.Addr[:], hip)
		sa = &sa4
	} else {
		domain = syscall.AF_INET6
		sa6 := syscall.SockaddrInet6{Port: port}
		copy(sa6.Addr[:], cfg.childHostIP)
		sa = &sa6
	}

	sd, err := syscall.Socket(domain, typ|syscall.SOCK_CLOEXEC, proto)
	if err != nil {
		return portBinding{}, fmt.Errorf("failed to create socket for userland proxy for %s: %w", cfg, err)
	}
	defer func() {
		if retErr != nil {
			syscall.Close(sd)
		}
	}()

	if err := syscall.SetsockoptInt(sd, syscall.SOL_SOCKET, syscall.SO_REUSEADDR, 1); err != nil {
		return portBinding{}, fmt.Errorf("failed to setsockopt(SO_REUSEADDR) for %s: %w", cfg, err)
	}

	if domain == syscall.AF_INET6 {
		syscall.SetsockoptInt(sd, syscall.IPPROTO_IPV6, syscall.IPV6_V6ONLY, 1)
	}
	if typ == syscall.SOCK_DGRAM {
		// Enable IP_PKTINFO for UDP sockets to get the destination address.
		// The destination address will be used as the source address when
		// sending back replies coming from the container.
		lvl := syscall.IPPROTO_IP
		opt := syscall.IP_PKTINFO
		optName := "IP_PKTINFO"
		if domain == syscall.AF_INET6 {
			lvl = syscall.IPPROTO_IPV6
			opt = syscall.IPV6_RECVPKTINFO
			optName = "IPV6_RECVPKTINFO"
		}
		if err := syscall.SetsockoptInt(sd, lvl, opt, 1); err != nil {
			return portBinding{}, fmt.Errorf("failed to setsockopt(%s) for %s: %w", optName, cfg, err)
		}
	}
	if err := syscall.Bind(sd, sa); err != nil {
		if cfg.HostPort == cfg.HostPortEnd {
			return portBinding{}, fmt.Errorf("failed to bind host port for %s: %w", cfg, err)
		}
		return portBinding{}, fmt.Errorf("failed to bind host port %d for %s: %w", port, cfg, err)
	}

	pb.boundSocket = os.NewFile(uintptr(sd), "listener")
	if pb.boundSocket == nil {
		return portBinding{}, fmt.Errorf("failed to convert socket for userland proxy for %s", cfg)
	}
	return pb, nil
}

// bindSCTP is based on sctp.ListenSCTP. The socket is created and bound, but
// does not start listening.
func bindSCTP(cfg portBindingReq, port int) (_ portBinding, retErr error) {
	pb := portBinding{PortBinding: cfg.GetCopy()}
	pb.HostPort = uint16(port)
	pb.HostPortEnd = pb.HostPort
	pb.childHostIP = cfg.childHostIP

	domain := syscall.AF_INET
	if cfg.childHostIP.To4() == nil {
		domain = syscall.AF_INET6
	}

	sd, err := syscall.Socket(domain, syscall.SOCK_STREAM|syscall.SOCK_CLOEXEC, syscall.IPPROTO_SCTP)
	if err != nil {
		return portBinding{}, fmt.Errorf("failed to create socket for userland proxy for %s: %w", cfg, err)
	}
	defer func() {
		if retErr != nil {
			syscall.Close(sd)
		}
	}()

	if domain == syscall.AF_INET6 {
		syscall.SetsockoptInt(sd, syscall.IPPROTO_IPV6, syscall.IPV6_V6ONLY, 1)
	}

	options := sctp.InitMsg{NumOstreams: sctp.SCTP_MAX_STREAM}
	if _, _, errno := syscall.Syscall6(syscall.SYS_SETSOCKOPT,
		uintptr(sd),
		sctp.SOL_SCTP,
		sctp.SCTP_INITMSG,
		uintptr(unsafe.Pointer(&options)), // #nosec G103 -- Ignore "G103: Use of unsafe calls should be audited"
		unsafe.Sizeof(options),
		0); errno != 0 {
		return portBinding{}, errno
	}

	if err := sctp.SCTPBind(sd,
		&sctp.SCTPAddr{IPAddrs: []net.IPAddr{{IP: cfg.childHostIP}}, Port: int(cfg.HostPort)},
		sctp.SCTP_BINDX_ADD_ADDR); err != nil {
		return portBinding{}, fmt.Errorf("failed to bind socket for userland proxy for %s: %w", cfg, err)
	}

	pb.boundSocket = os.NewFile(uintptr(sd), "listener")
	if pb.boundSocket == nil {
		return portBinding{}, fmt.Errorf("failed to convert socket for userland proxy for %s", cfg)
	}
	return pb, nil
}

func startSCTPProxy(cfg portBindingReq, port int, proxyPath string) (_ portBinding, retErr error) {
	pb := portBinding{PortBinding: cfg.GetCopy()}
	pb.HostPort = uint16(port)
	pb.HostPortEnd = pb.HostPort
	pb.childHostIP = cfg.childHostIP

	var err error
	pb.stopProxy, err = startProxy(pb.childPortBinding(), proxyPath, nil)
	if err != nil {
		return portBinding{}, err
	}
	return pb, nil
}

// releasePorts attempts to release all port bindings, does not stop on failure
func (n *bridgeNetwork) releasePorts(ep *bridgeEndpoint) error {
	n.Lock()
	pbs := ep.portMapping
	ep.portMapping = nil
	n.Unlock()

	return n.releasePortBindings(pbs)
}

func (n *bridgeNetwork) releasePortBindings(pbs []portBinding) error {
	var errs []error
	for _, pb := range pbs {
		if pb.boundSocket != nil {
			if err := pb.boundSocket.Close(); err != nil {
				errs = append(errs, fmt.Errorf("failed to close socket for port mapping %s: %w", pb, err))
			}
		}
		if pb.portDriverRemove != nil {
			if err := pb.portDriverRemove(); err != nil {
				errs = append(errs, err)
			}
		}
		if pb.stopProxy != nil {
			if err := pb.stopProxy(); err != nil {
				errs = append(errs, fmt.Errorf("failed to stop userland proxy for port mapping %s: %w", pb, err))
			}
		}
		if err := n.setPerPortIptables(pb, false); err != nil {
			errs = append(errs, fmt.Errorf("failed to remove iptables rules for port mapping %s: %w", pb, err))
		}
		if pb.HostPort > 0 {
			portallocator.Get().ReleasePort(pb.childHostIP, pb.Proto.String(), int(pb.HostPort))
		}
	}
	return errors.Join(errs...)
}

func (n *bridgeNetwork) setPerPortIptables(b portBinding, enable bool) error {
	v := iptables.IPv4
	if b.IP.To4() == nil {
		v = iptables.IPv6
	}
	if enabled, err := n.iptablesEnabled(v); err != nil || !enabled {
		// Nothing to do, iptables/ip6tables is not enabled.
		return nil
	}

	if err := n.filterPortMappedOnLoopback(b, enable); err != nil {
		return err
	}

	if err := n.filterDirectAccess(b, enable); err != nil {
		return err
	}

	if (b.IP.To4() != nil) != (b.HostIP.To4() != nil) {
		// The binding is between containerV4 and hostV6 (not vice versa as that
		// will have been rejected earlier). It's handled by docker-proxy. So, no
		// further iptables rules are required.
		return nil
	}

	bridgeName := n.getNetworkBridgeName()
	proxyPath := n.userlandProxyPath()
	if err := setPerPortNAT(b, v, proxyPath, bridgeName, enable); err != nil {
		return err
	}

	if !n.gwMode(v).unprotected() {
		if err := setPerPortForwarding(b, v, bridgeName, enable); err != nil {
			return err
		}
	}
	return nil
}

func setPerPortNAT(b portBinding, ipv iptables.IPVersion, proxyPath string, bridgeName string, enable bool) error {
	if b.HostPort == 0 {
		// NAT is disabled.
		return nil
	}
	// iptables interprets "0.0.0.0" as "0.0.0.0/32", whereas we
	// want "0.0.0.0/0". "0/0" is correctly interpreted as "any
	// value" by both iptables and ip6tables.
	hostIP := "0/0"
	if !b.childHostIP.IsUnspecified() {
		hostIP = b.childHostIP.String()
	}
	args := []string{
		"-p", b.Proto.String(),
		"-d", hostIP,
		"--dport", strconv.Itoa(int(b.HostPort)),
		"-j", "DNAT",
		"--to-destination", net.JoinHostPort(b.IP.String(), strconv.Itoa(int(b.Port))),
	}
	hairpinMode := proxyPath == ""
	if !hairpinMode {
		args = append(args, "!", "-i", bridgeName)
	}
	if ipv == iptables.IPv6 {
		args = append(args, "!", "-s", "fe80::/10")
	}
	rule := iptables.Rule{IPVer: ipv, Table: iptables.Nat, Chain: DockerChain, Args: args}
	if err := appendOrDelChainRule(rule, "DNAT", enable); err != nil {
		return err
	}

	rule = iptables.Rule{IPVer: ipv, Table: iptables.Nat, Chain: "POSTROUTING", Args: []string{
		"-p", b.Proto.String(),
		"-s", b.IP.String(),
		"-d", b.IP.String(),
		"--dport", strconv.Itoa(int(b.Port)),
		"-j", "MASQUERADE",
	}}
	if err := appendOrDelChainRule(rule, "MASQUERADE", hairpinMode && enable); err != nil {
		return err
	}

	return nil
}

func setPerPortForwarding(b portBinding, ipv iptables.IPVersion, bridgeName string, enable bool) error {
	// Insert rules for open ports at the top of the filter table's DOCKER
	// chain (a per-network DROP rule, which must come after these per-port
	// per-container ACCEPT rules, is appended to the chain when the network
	// is created).
	rule := iptables.Rule{IPVer: ipv, Table: iptables.Filter, Chain: DockerChain, Args: []string{
		"!", "-i", bridgeName,
		"-o", bridgeName,
		"-p", b.Proto.String(),
		"-d", b.IP.String(),
		"--dport", strconv.Itoa(int(b.Port)),
		"-j", "ACCEPT",
	}}
	if err := programChainRule(rule, "OPEN PORT", enable); err != nil {
		return err
	}

	if b.Proto == types.SCTP && os.Getenv("DOCKER_IPTABLES_SCTP_CHECKSUM") == "1" {
		// Linux kernel v4.9 and below enables NETIF_F_SCTP_CRC for veth by
		// the following commit.
		// This introduces a problem when combined with a physical NIC without
		// NETIF_F_SCTP_CRC. As for a workaround, here we add an iptables entry
		// to fill the checksum.
		//
		// https://github.com/torvalds/linux/commit/c80fafbbb59ef9924962f83aac85531039395b18
		rule := iptables.Rule{IPVer: ipv, Table: iptables.Mangle, Chain: "POSTROUTING", Args: []string{
			"-p", b.Proto.String(),
			"--sport", strconv.Itoa(int(b.Port)),
			"-j", "CHECKSUM",
			"--checksum-fill",
		}}
		if err := appendOrDelChainRule(rule, "SCTP CHECKSUM", enable); err != nil {
			return err
		}
	}

	return nil
}

// filterPortMappedOnLoopback adds an iptables rule that drops remote
// connections to ports mapped on loopback addresses.
//
// This is a no-ip if the portBinding is for IPv6 (IPv6 loopback address is
// non-routable), or over a network with gw_mode=routed (PBs in routed mode
// don't map ports on the host).
func (n *bridgeNetwork) filterPortMappedOnLoopback(b portBinding, enable bool) error {
	hostIP := b.childHostIP
	if b.HostPort == 0 || !hostIP.IsLoopback() || b.childHostIP.To4() == nil {
		return nil
	}

	acceptMirrored := iptables.Rule{IPVer: iptables.IPv4, Table: iptables.Raw, Chain: "PREROUTING", Args: []string{
		"-p", b.Proto.String(),
		"-d", hostIP.String(),
		"--dport", strconv.Itoa(int(b.HostPort)),
		"-i", "loopback0",
		"-j", "ACCEPT",
	}}
	enableMirrored := enable && isRunningUnderWSL2MirroredMode()
	if err := appendOrDelChainRule(acceptMirrored, "LOOPBACK FILTERING - ACCEPT MIRRORED", enableMirrored); err != nil {
		return err
	}

	drop := iptables.Rule{IPVer: iptables.IPv4, Table: iptables.Raw, Chain: "PREROUTING", Args: []string{
		"-p", b.Proto.String(),
		"-d", hostIP.String(),
		"--dport", strconv.Itoa(int(b.HostPort)),
		"!", "-i", "lo",
		"-j", "DROP",
	}}
	if err := appendOrDelChainRule(drop, "LOOPBACK FILTERING - DROP", enable); err != nil {
		return err
	}

	return nil
}

// filterDirectAccess adds an iptables rule that drops 'direct' remote
// connections made to the container's IP address, when the network gateway
// mode is "nat".
//
// This is a no-op if the gw_mode is "nat-unprotected" or "routed".
func (n *bridgeNetwork) filterDirectAccess(b portBinding, enable bool) error {
	ipv := iptables.IPv4
	if b.IP.To4() == nil {
		ipv = iptables.IPv6
	}

	// gw_mode=nat-unprotected means there's minimal security for NATed ports,
	// so don't filter direct access.
	if n.gwMode(ipv).unprotected() || n.gwMode(ipv).routed() {
		return nil
	}

	bridgeName := n.getNetworkBridgeName()
	drop := iptables.Rule{IPVer: ipv, Table: iptables.Raw, Chain: "PREROUTING", Args: []string{
		"-p", b.Proto.String(),
		"-d", b.IP.String(), // Container IP address
		"--dport", strconv.Itoa(int(b.Port)), // Container port
		"!", "-i", bridgeName,
		"-j", "DROP",
	}}
	if err := appendOrDelChainRule(drop, "DIRECT ACCESS FILTERING - DROP", enable); err != nil {
		return err
	}

	return nil
}

func (n *bridgeNetwork) reapplyPerPortIptables4() {
	n.reapplyPerPortIptables(func(b portBinding) bool { return b.IP.To4() != nil })
}

func (n *bridgeNetwork) reapplyPerPortIptables6() {
	n.reapplyPerPortIptables(func(b portBinding) bool { return b.IP.To4() == nil })
}

func (n *bridgeNetwork) reapplyPerPortIptables(needsReconfig func(portBinding) bool) {
	n.Lock()
	var allPBs []portBinding
	for _, ep := range n.endpoints {
		allPBs = append(allPBs, ep.portMapping...)
	}
	n.Unlock()

	for _, b := range allPBs {
		if needsReconfig(b) {
			if err := n.setPerPortIptables(b, true); err != nil {
				log.G(context.TODO()).Warnf("Failed to reconfigure iptables on firewalld reload %s: %s", b, err)
			}
		}
	}
}
