package bridge

import (
	"context"
	"errors"
	"fmt"
	"net"

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

	bindings := make([]portBinding, 0, len(cfg)*2)

	defer func() {
		if retErr != nil {
			if err := n.releasePortBindings(bindings); err != nil {
				log.G(context.TODO()).Warnf("Release port bindings: %s", err.Error())
			}
		}
	}()

	proxyPath := n.userlandProxyPath()
	for _, c := range cfg {
		toBind := make([]types.PortBinding, 0, 2)
		if bindingIPv4, ok := configurePortBindingIPv4(c, containerIPv4, defHostIP); ok {
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
		if bindingIPv6, ok := configurePortBindingIPv6(c, containerIP, defHostIP); ok {
			toBind = append(toBind, bindingIPv6)
		}

		newB, err := bindHostPorts(toBind, proxyPath)
		if err != nil {
			return nil, err
		}
		bindings = append(bindings, newB...)
	}

	for _, b := range bindings {
		if err := n.setPerPortIptables(b, true); err != nil {
			return nil, err
		}
	}

	return bindings, nil
}

// configurePortBindingIPv4 returns a new port binding with the HostIP field populated
// if a binding is required, else nil.
func configurePortBindingIPv4(bnd types.PortBinding, containerIPv4, defHostIP net.IP) (types.PortBinding, bool) {
	if len(containerIPv4) == 0 {
		return types.PortBinding{}, false
	}
	if len(bnd.HostIP) > 0 && bnd.HostIP.To4() == nil {
		// The mapping is explicitly IPv6.
		return types.PortBinding{}, false
	}
	// If there's no host address, use the default.
	if len(bnd.HostIP) == 0 {
		if defHostIP.To4() == nil {
			// The default binding address is IPv6.
			return types.PortBinding{}, false
		}
		bnd.HostIP = defHostIP
	}
	// Unmap the addresses if they're IPv4-mapped IPv6.
	bnd.HostIP = bnd.HostIP.To4()
	bnd.IP = containerIPv4.To4()
	// Adjust HostPortEnd if this is not a range.
	if bnd.HostPortEnd == 0 {
		bnd.HostPortEnd = bnd.HostPort
	}
	return bnd, true
}

// configurePortBindingIPv6 returns a new port binding with the HostIP field populated
// if a binding is required, else nil.
func configurePortBindingIPv6(bnd types.PortBinding, containerIP, defHostIP net.IP) (types.PortBinding, bool) {
	if containerIP == nil {
		return types.PortBinding{}, false
	}
	if len(bnd.HostIP) > 0 && bnd.HostIP.To4() != nil {
		// The mapping is explicitly IPv4.
		return types.PortBinding{}, false
	}

	// If there's no host address, use the default.
	if len(bnd.HostIP) == 0 {
		if defHostIP.Equal(net.IPv4zero) {
			if !netutils.IsV6Listenable() {
				// No implicit binding if the host has no IPv6 support.
				return types.PortBinding{}, false
			}
			// Implicit binding to "::", no explicit HostIP and the default is 0.0.0.0
			bnd.HostIP = net.IPv6zero
		} else if defHostIP.To4() == nil {
			// The default binding IP is an IPv6 address, use it.
			bnd.HostIP = defHostIP
		} else {
			// The default binding IP is an IPv4 address, nothing to do here.
			return types.PortBinding{}, false
		}
	}
	bnd.IP = containerIP
	// Adjust HostPortEnd if this is not a range.
	if bnd.HostPortEnd == 0 {
		bnd.HostPortEnd = bnd.HostPort
	}
	return bnd, true
}

// bindHostPorts allocates ports and starts docker-proxy for the given cfg. The
// caller is responsible for ensuring that all entries in cfg map the same proto,
// container port, and host port range (their host addresses must differ).
func bindHostPorts(cfg []types.PortBinding, proxyPath string) ([]portBinding, error) {
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
func attemptBindHostPorts(
	cfg []types.PortBinding,
	proto string,
	hostPortStart, hostPortEnd uint16,
	proxyPath string,
) (_ []portBinding, retErr error) {
	addrs := make([]net.IP, 0, len(cfg))
	for _, c := range cfg {
		addrs = append(addrs, c.HostIP)
	}

	pa := portallocator.Get()
	port, err := pa.RequestPortsInRange(addrs, proto, int(hostPortStart), int(hostPortEnd))
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

	res := make([]portBinding, 0, len(cfg))
	for _, c := range cfg {
		pb := portBinding{PortBinding: c.GetCopy()}
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
		errP := pb.stopProxy()
		if errP != nil {
			errP = fmt.Errorf("failed to stop docker-proxy for port mapping %s: %w", pb, errP)
		}
		errN := n.setPerPortIptables(pb, false)
		if errN != nil {
			errN = fmt.Errorf("failed to remove iptables rules for port mapping %s: %w", pb, errN)
		}
		portallocator.Get().ReleasePort(pb.HostIP, pb.Proto.String(), int(pb.HostPort))
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

	natChain, _, _, _, err := n.getDriverChains(v)
	if err != nil || natChain == nil {
		// Nothing to do, iptables/ip6tables is not enabled.
		return nil
	}
	action := iptables.Delete
	if enable {
		action = iptables.Insert
	}
	return natChain.Forward(
		action,
		b.HostIP,
		int(b.HostPort),
		b.Proto.String(),
		b.IP.String(),
		int(b.Port),
		n.getNetworkBridgeName(),
	)
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
