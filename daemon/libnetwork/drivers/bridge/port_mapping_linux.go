package bridge

import (
	"context"
	"errors"
	"fmt"
	"net"
	"slices"

	"github.com/containerd/log"
	"github.com/moby/moby/v2/daemon/internal/sliceutil"
	"github.com/moby/moby/v2/daemon/libnetwork/netutils"
	"github.com/moby/moby/v2/daemon/libnetwork/portmapperapi"
	"github.com/moby/moby/v2/daemon/libnetwork/types"
)

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

	pms := n.portMappers()

	bindings := make([]portmapperapi.PortBinding, 0, len(cfg)*2)
	defer func() {
		if retErr != nil {
			if err := n.unmapPBs(ctx, bindings); err != nil {
				log.G(ctx).WithFields(log.Fields{
					"bindings": bindings,
					"error":    err,
					"origErr":  retErr,
				}).Warn("Failed to unmap port bindings after error")
			}
		}
	}()

	bindingReqs := n.sortAndNormPBs(ctx, ep, cfg, defHostIP, pbmReq)

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
		if i < len(bindingReqs)-1 && c.Mapper == bindingReqs[i+1].Mapper && needSamePort(c, bindingReqs[i+1]) {
			// This port binding matches the next, apart from host IP. So, continue
			// collecting bindings, then allocate the same host port for all addresses.
			continue
		}

		pm, err := pms.Get(c.Mapper)
		if err != nil {
			return nil, err
		}

		newB, err := pm.MapPorts(ctx, toBind, n.firewallerNetwork)
		if err != nil {
			return nil, err
		}
		bindings = append(bindings, sliceutil.Map(newB, func(b portmapperapi.PortBinding) portmapperapi.PortBinding {
			b.Mapper = c.Mapper
			return b
		})...)

		// Reset toBind now the ports are bound.
		toBind = toBind[:0]
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
// Finally, port bindings are sorted into the ordering defined by
// [PortBindingReqs.Compare] in order to form groups of port bindings that
// should be processed in one go.
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

	hairpin := n.hairpin()
	disableNAT4, disableNAT6 := n.getNATDisabled()

	add4 := !ep.portBindingState.ipv4 && pbmReq.ipv4 || (disableNAT4 && !ep.portBindingState.routed && pbmReq.routed)
	add6 := !ep.portBindingState.ipv6 && pbmReq.ipv6 || (disableNAT6 && !ep.portBindingState.routed && pbmReq.routed)

	reqs := make([]portmapperapi.PortBindingReq, 0, len(cfg))
	for _, c := range cfg {
		if c.HostPortEnd == 0 {
			c.HostPortEnd = c.HostPort
		}

		if add4 {
			if bindingIPv4, ok := configurePortBindingIPv4(ctx, disableNAT4, c, containerIPv4, defHostIP); ok {
				reqs = append(reqs, bindingIPv4)
			}
		}

		// If the container has no IPv6 address, allow proxying host IPv6 traffic to it
		// by setting up the binding with the IPv4 interface if the userland proxy is enabled
		// This change was added to keep backward compatibility
		containerIP := containerIPv6
		if containerIPv6 == nil && pbmReq.ipv4 && add6 {
			if hairpin {
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
			if bindingIPv6, ok := configurePortBindingIPv6(ctx, disableNAT6, c, containerIP, defHostIP); ok {
				reqs = append(reqs, bindingIPv6)
			}
		}
	}
	slices.SortFunc(reqs, func(a, b portmapperapi.PortBindingReq) int {
		return a.Compare(b)
	})
	return reqs
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
	bnd.Mapper = "nat"
	if disableNAT {
		bnd.Mapper = "routed"
	}
	return bnd, true
}

// configurePortBindingIPv6 returns a new port binding with the HostIP field
// populated and true, if a binding is required. Else, false and an empty
// binding.
func configurePortBindingIPv6(
	ctx context.Context,
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
	bnd.Mapper = "nat"
	if disableNAT {
		bnd.Mapper = "routed"
	}
	return bnd, true
}

// releasePorts attempts to release all port bindings, does not stop on failure
func (n *bridgeNetwork) releasePorts(ep *bridgeEndpoint) error {
	n.Lock()
	pbs := ep.portMapping
	ep.portMapping = nil
	ep.portBindingState = portBindingMode{}
	n.Unlock()

	return n.unmapPBs(context.TODO(), pbs)
}

func (n *bridgeNetwork) unmapPBs(ctx context.Context, bindings []portmapperapi.PortBinding) error {
	pms := n.portMappers()

	var errs []error
	for _, b := range bindings {
		pm, err := pms.Get(b.Mapper)
		if err != nil {
			errs = append(errs, fmt.Errorf("unmapping port binding %s: %w", b.PortBinding, err))
			continue
		}

		if err := pm.UnmapPorts(ctx, []portmapperapi.PortBinding{b}, n.firewallerNetwork); err != nil {
			errs = append(errs, fmt.Errorf("unmapping port binding %s: %w", b.PortBinding, err))
		}
	}

	return errors.Join(errs...)
}

func (n *bridgeNetwork) reapplyPerPortIptables() {
	n.Lock()
	var allPBs []portmapperapi.PortBinding
	var allEPs []*bridgeEndpoint
	for _, ep := range n.endpoints {
		allPBs = append(allPBs, ep.portMapping...)
		allEPs = append(allEPs, ep)
	}
	n.Unlock()

	for _, ep := range allEPs {
		netip4, netip6 := ep.netipAddrs()
		if err := n.firewallerNetwork.AddEndpoint(context.TODO(), netip4, netip6); err != nil {
			log.G(context.TODO()).Warnf("Failed to reconfigure Endpoint: %s", err)
		}
	}

	if err := n.firewallerNetwork.AddPorts(context.Background(), mergeChildHostIPs(allPBs)); err != nil {
		log.G(context.TODO()).Warnf("Failed to reconfigure NAT: %s", err)
	}
}
