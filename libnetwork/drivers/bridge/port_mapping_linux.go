// FIXME(thaJeztah): remove once we are a module; the go:build directive prevents go from downgrading language version to go1.16:
//go:build go1.21

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

	"github.com/containerd/log"
	"github.com/docker/docker/libnetwork/iptables"
	"github.com/docker/docker/libnetwork/netutils"
	"github.com/docker/docker/libnetwork/portallocator"
	"github.com/docker/docker/libnetwork/portmapper"
	"github.com/docker/docker/libnetwork/types"
)

type portBinding struct {
	types.PortBinding
	stopProxy func() error
}

type portBindingReq struct {
	types.PortBinding
	disableNAT bool
}

// addPortMappings takes cfg, the configuration for port mappings, selects host
// ports when ranges are given, starts docker-proxy or its dummy to reserve
// host ports, and sets up iptables NAT/forwarding rules as necessary. If
// anything goes wrong, it will undo any work it's done and return an error.
// Otherwise, the returned slice of portBinding has an entry per address
// family (if cfg describes a mapping for 'any' host address, it's expanded
// into mappings for IPv4 and IPv6, because that's how the mapping is presented
// in 'inspect'). HostPort and HostPortEnd in each returned portBinding are set
// to the selected and reserved port.
func (n *bridgeNetwork) addPortMappings(
	epAddrV4, epAddrV6 *net.IPNet,
	cfg []types.PortBinding,
	defHostIP net.IP,
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

	disableNAT4, disableNAT6 := n.getNATDisabled()
	if err := validatePortBindings(cfg, !disableNAT4, !disableNAT6, containerIPv6); err != nil {
		return nil, err
	}

	bindings := make([]portBinding, 0, len(cfg)*2)

	defer func() {
		if retErr != nil {
			if err := n.releasePortBindings(bindings); err != nil {
				log.G(context.TODO()).Warnf("Release port bindings: %s", err.Error())
			}
		}
	}()

	sortedCfg := slices.Clone(cfg)
	sortAndNormPBs(sortedCfg)

	proxyPath := n.userlandProxyPath()

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
		if bindingIPv4, ok := configurePortBindingIPv4(disableNAT4, c, containerIPv4, defHostIP); ok {
			toBind = append(toBind, bindingIPv4)
		}

		// If the container has no IPv6 address, allow proxying host IPv6 traffic to it
		// by setting up the binding with the IPv4 interface if the userland proxy is enabled
		// This change was added to keep backward compatibility
		// TODO(robmry) - this will silently ignore port bindings with an explicit IPv6
		//  host address, when docker-proxy is disabled, and the container is IPv4-only.
		//  If there's no proxying and the container has no IPv6, should probably error if ...
		//  - the mapping's host address is IPv6, or
		//  - the mapping has no host address, but the default address is IPv6.
		containerIP := containerIPv6
		if proxyPath != "" && (containerIPv6 == nil) {
			containerIP = containerIPv4
		}
		if bindingIPv6, ok := configurePortBindingIPv6(disableNAT6, c, containerIP, defHostIP); ok {
			toBind = append(toBind, bindingIPv6)
		}

		if i < len(sortedCfg)-1 && needSamePort(c, sortedCfg[i+1]) {
			// This port binding matches the next, apart from host IP. So, continue
			// collecting bindings, then allocate the same host port for all addresses.
			continue
		}

		// Allocate a host port, and reserve it by starting docker-proxy for each host
		// address in toBind.
		newB, err := bindHostPorts(toBind, proxyPath)
		if err != nil {
			return nil, err
		}
		bindings = append(bindings, newB...)

		// Reset the collection of bindings now they're bound.
		toBind = toBind[:0]
	}

	for _, b := range bindings {
		if err := n.setPerPortIptables(b, true); err != nil {
			return nil, err
		}
	}

	return bindings, nil
}

// Limit the number of errors reported, because there may be a lot of port
// bindings (host port ranges are expanded by the CLI).
const validationErrLimit = 6

// validatePortBindings checks that, if NAT is disabled for all uses of a
// PortBinding, no HostPort, or non-zero HostIP, is specified - because they have
// no meaning. A zero HostIP is allowed, as it's used to determine the address
// family.
//
// The default binding IP is not considered, meaning that no error is raised if
// there is a default binding address that is not used but could have been.
//
// For example, the default is an IPv6 interface address, no HostIP is specified,
// and NAT6 is disabled; the default is ignored and no error will be raised. (Note
// that this example may be valid if the container has no IPv6 address, and
// docker-proxy is used to forward between the default IPv6 address and the
// container's IPv4. So, simply disallowing a non-zero IPv6 default when NAT6
// is disabled for the network would be incorrect.)
func validatePortBindings(pbs []types.PortBinding, nat4, nat6 bool, cIPv6 net.IP) error {
	var errs []error
	for i := range pbs {
		pb := &pbs[i]
		disallowHostPort := false
		if !nat4 && len(pb.HostIP) > 0 && pb.HostIP.To4() != nil && !pb.HostIP.Equal(net.IPv4zero) {
			// There's no NAT4, so don't allow a nonzero IPv4 host address in the mapping. The port will
			// be accessible via any host interface.
			errs = append(errs,
				fmt.Errorf("NAT is disabled, omit host address in port mapping %s, or use 0.0.0.0::%d to open port %d for IPv4-only",
					pb, pb.Port, pb.Port))
			// The mapping is IPv4-specific but there's no NAT4, so a host port would make no sense.
			disallowHostPort = true
		} else if !nat6 && len(pb.HostIP) > 0 && pb.HostIP.To4() == nil && !pb.HostIP.Equal(net.IPv6zero) {
			// If the container has no IPv6 address, the userland proxy will proxy between the
			// host's IPv6 address and the container's IPv4. So, even with no NAT6, it's ok for
			// an IPv6 port mapping to include a specific host address or port.
			if len(cIPv6) > 0 {
				// There's no NAT6, so don't allow an IPv6 host address in the mapping. The port will
				// accessible via any host interface.
				errs = append(errs,
					fmt.Errorf("NAT is disabled, omit host address in port mapping %s, or use [::]::%d to open port %d for IPv6-only",
						pb, pb.Port, pb.Port))
				// The mapping is IPv6-specific but there's no NAT6, so a host port would make no sense.
				disallowHostPort = true
			}
		} else if !nat4 && !nat6 {
			// There's no NAT, so it would make no sense to specify a host port.
			disallowHostPort = true
		}
		if disallowHostPort && pb.HostPort != 0 {
			errs = append(errs,
				fmt.Errorf("host port must not be specified in mapping %s because NAT is disabled", pb))
		}
		if len(errs) >= validationErrLimit {
			break
		}
	}
	return errors.Join(errs...)
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

// configurePortBindingIPv4 returns a new port binding with the HostIP field populated
// if a binding is required, else nil.
func configurePortBindingIPv4(disableNAT bool, bnd types.PortBinding, containerIPv4, defHostIP net.IP) (portBindingReq, bool) {
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
		bnd.HostIP = defHostIP
	}
	// Unmap the addresses if they're IPv4-mapped IPv6.
	bnd.HostIP = bnd.HostIP.To4()
	bnd.IP = containerIPv4.To4()
	return portBindingReq{
		PortBinding: bnd,
		disableNAT:  disableNAT,
	}, true
}

// configurePortBindingIPv6 returns a new port binding with the HostIP field populated
// if a binding is required, else nil.
func configurePortBindingIPv6(disableNAT bool, bnd types.PortBinding, containerIP, defHostIP net.IP) (portBindingReq, bool) {
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
			// The default binding IP is an IPv6 address, use it.
			bnd.HostIP = defHostIP
		} else {
			// The default binding IP is an IPv4 address, nothing to do here.
			return portBindingReq{}, false
		}
	}
	bnd.IP = containerIP
	return portBindingReq{
		PortBinding: bnd,
		disableNAT:  disableNAT,
	}, true
}

// bindHostPorts allocates ports and starts docker-proxy for the given cfg. The
// caller is responsible for ensuring that all entries in cfg map the same proto,
// container port, and host port range (their host addresses must differ).
func bindHostPorts(cfg []portBindingReq, proxyPath string) ([]portBinding, error) {
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
		b, err = attemptBindHostPorts(cfg, proto.String(), hostPort, hostPortEnd, proxyPath)
		if err == nil {
			return b, nil
		}
		// There is no point in immediately retrying to map an explicitly chosen port.
		if hostPort != 0 && hostPort == hostPortEnd {
			log.G(context.TODO()).Warnf("Failed to allocate and map port: %s", err)
			break
		}
		log.G(context.TODO()).Warnf("Failed to allocate and map port: %s, retry: %d", err, i+1)
	}
	return nil, err
}

// Allow unit tests to supply a dummy StartProxy.
var startProxy = portmapper.StartProxy

// attemptBindHostPorts allocates host ports for each port mapping that requires
// one, and reserves those ports by starting docker-proxy.
//
// If the allocator doesn't have an available port in the required range, or the
// docker-proxy process doesn't start (perhaps because another process has
// already bound the port), all resources are released and an error is returned.
// When ports are successfully reserved, a portBinding is returned for each
// mapping.
//
// If NAT is disabled for any of the bindings, no host port reservation is
// needed. These bindings are included in results, as the container port itself
// needs to be opened in the firewall.
func attemptBindHostPorts(
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
			addrs = append(addrs, c.HostIP)
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
	for _, c := range cfg {
		pb := portBinding{PortBinding: c.GetCopy()}
		if c.disableNAT {
			pb.HostPort = 0
		} else {
			pb.stopProxy, err = startProxy(c.Proto.String(), c.HostIP, port, c.IP, int(c.Port), proxyPath)
			if err != nil {
				return nil, fmt.Errorf("failed to bind port %s:%d/%s: %w", c.HostIP, port, c.Proto, err)
			}
			defer func() {
				if retErr != nil {
					if err := pb.stopProxy(); err != nil {
						log.G(context.TODO()).Warnf("Failed to stop userland proxy for port mapping %s: %s", pb, err)
					}
				}
			}()
			pb.HostPort = uint16(port)
		}
		pb.HostPortEnd = pb.HostPort
		res = append(res, pb)
	}
	return res, nil
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
		var errP error
		if pb.stopProxy != nil {
			errP = pb.stopProxy()
			if errP != nil {
				errP = fmt.Errorf("failed to stop docker-proxy for port mapping %s: %w", pb, errP)
			}
		}
		errN := n.setPerPortIptables(pb, false)
		if errN != nil {
			errN = fmt.Errorf("failed to remove iptables rules for port mapping %s: %w", pb, errN)
		}
		if pb.HostPort > 0 {
			portallocator.Get().ReleasePort(pb.HostIP, pb.Proto.String(), int(pb.HostPort))
		}
		errs = append(errs, errP, errN)
	}
	return errors.Join(errs...)
}

func (n *bridgeNetwork) setPerPortIptables(b portBinding, enable bool) error {
	if (b.IP.To4() != nil) != (b.HostIP.To4() != nil) {
		// The binding is between containerV4 and hostV6 (not vice-versa as that
		// will have been rejected earlier). It's handled by docker-proxy, so no
		// additional iptables rules are required.
		return nil
	}
	v := iptables.IPv4
	if b.IP.To4() == nil {
		v = iptables.IPv6
	}

	if enabled, err := n.iptablesEnabled(v); err != nil || !enabled {
		// Nothing to do, iptables/ip6tables is not enabled.
		return nil
	}

	bridgeName := n.getNetworkBridgeName()
	proxyPath := n.userlandProxyPath()
	if err := setPerPortNAT(b, v, proxyPath, bridgeName, enable); err != nil {
		return err
	}
	if err := setPerPortForwarding(b, v, bridgeName, enable); err != nil {
		return err
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
	if !b.HostIP.IsUnspecified() {
		hostIP = b.HostIP.String()
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
	rule := iptRule{ipv: ipv, table: iptables.Nat, chain: DockerChain, args: args}
	if err := appendOrDelChainRule(rule, "DNAT", enable); err != nil {
		return err
	}

	args = []string{
		"-p", b.Proto.String(),
		"-s", b.IP.String(),
		"-d", b.IP.String(),
		"--dport", strconv.Itoa(int(b.Port)),
		"-j", "MASQUERADE",
	}
	rule = iptRule{ipv: ipv, table: iptables.Nat, chain: "POSTROUTING", args: args}
	if err := appendOrDelChainRule(rule, "MASQUERADE", enable); err != nil {
		return err
	}

	return nil
}

func setPerPortForwarding(b portBinding, ipv iptables.IPVersion, bridgeName string, enable bool) error {
	args := []string{
		"!", "-i", bridgeName,
		"-o", bridgeName,
		"-p", b.Proto.String(),
		"-d", b.IP.String(),
		"--dport", strconv.Itoa(int(b.Port)),
		"-j", "ACCEPT",
	}
	rule := iptRule{ipv: ipv, table: iptables.Filter, chain: DockerChain, args: args}
	if err := appendOrDelChainRule(rule, "MASQUERADE", enable); err != nil {
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
		args = []string{
			"-p", b.Proto.String(),
			"--sport", strconv.Itoa(int(b.Port)),
			"-j", "CHECKSUM",
			"--checksum-fill",
		}
		rule := iptRule{ipv: ipv, table: iptables.Mangle, chain: "POSTROUTING", args: args}
		if err := appendOrDelChainRule(rule, "MASQUERADE", enable); err != nil {
			return err
		}
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
				log.G(context.TODO()).Warnf("Failed to reconfigure NAT %s: %s", b, err)
			}
		}
	}
}
