//go:build linux

package bridge

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net"
	"sync"

	"github.com/containerd/containerd/log"
	"github.com/docker/docker/libnetwork/types"
	"github.com/ishidawataru/sctp"
)

func (n *bridgeNetwork) allocatePorts(ep *bridgeEndpoint, reqDefBindIP net.IP, ulPxyEnabled bool) ([]types.PortBinding, error) {
	if ep.extConnConfig == nil || ep.extConnConfig.PortBindings == nil {
		return nil, nil
	}

	defHostIP := net.IPv4zero // 0.0.0.0
	if reqDefBindIP != nil {
		defHostIP = reqDefBindIP
	}

	var containerIPv6 net.IP
	if ep.addrv6 != nil {
		containerIPv6 = ep.addrv6.IP
	}

	pb, err := n.allocatePortsInternal(ep.extConnConfig.PortBindings, ep.addr.IP, containerIPv6, defHostIP, ulPxyEnabled)
	if err != nil {
		return nil, err
	}
	return pb, nil
}

func (n *bridgeNetwork) allocatePortsInternal(bindings []types.PortBinding, containerIPv4, containerIPv6, defHostIP net.IP, ulPxyEnabled bool) ([]types.PortBinding, error) {
	bs := make([]types.PortBinding, 0, len(bindings))
	for _, c := range bindings {
		bIPv4 := c.GetCopy()
		bIPv6 := c.GetCopy()
		// Allocate IPv4 Port mappings
		if ok := n.validatePortBindingIPv4(&bIPv4, containerIPv4, defHostIP); ok {
			if err := n.allocatePort(&bIPv4, ulPxyEnabled); err != nil {
				// On allocation failure, release previously allocated ports. On cleanup error, just log a warning message
				if cuErr := n.releasePortsInternal(bs); cuErr != nil {
					log.G(context.TODO()).Warnf("allocation failure for %v, failed to clear previously allocated ipv4 port bindings: %v", bIPv4, cuErr)
				}
				return nil, err
			}
			bs = append(bs, bIPv4)
		}

		// skip adding implicit v6 addr, when the kernel was booted with `ipv6.disable=1`
		// https://github.com/moby/moby/issues/42288
		isV6Binding := c.HostIP != nil && c.HostIP.To4() == nil
		if !isV6Binding && !IsV6Listenable() {
			continue
		}

		// Allocate IPv6 Port mappings
		// If the container has no IPv6 address, allow proxying host IPv6 traffic to it
		// by setting up the binding with the IPv4 interface if the userland proxy is enabled
		// This change was added to keep backward compatibility
		containerIP := containerIPv6
		if ulPxyEnabled && (containerIPv6 == nil) {
			containerIP = containerIPv4
		}
		if ok := n.validatePortBindingIPv6(&bIPv6, containerIP, defHostIP); ok {
			if err := n.allocatePort(&bIPv6, ulPxyEnabled); err != nil {
				// On allocation failure, release previously allocated ports. On cleanup error, just log a warning message
				if cuErr := n.releasePortsInternal(bs); cuErr != nil {
					log.G(context.TODO()).Warnf("allocation failure for %v, failed to clear previously allocated ipv6 port bindings: %v", bIPv6, cuErr)
				}
				return nil, err
			}
			bs = append(bs, bIPv6)
		}
	}
	return bs, nil
}

// validatePortBindingIPv4 validates the port binding, populates the missing Host IP field and returns true
// if this is a valid IPv4 binding, else returns false
func (n *bridgeNetwork) validatePortBindingIPv4(bnd *types.PortBinding, containerIPv4, defHostIP net.IP) bool {
	// Return early if there is a valid Host IP, but its not a IPv4 address
	if len(bnd.HostIP) > 0 && bnd.HostIP.To4() == nil {
		return false
	}
	// Adjust the host address in the operational binding
	if len(bnd.HostIP) == 0 {
		// Return early if the default binding address is an IPv6 address
		if defHostIP.To4() == nil {
			return false
		}
		bnd.HostIP = defHostIP
	}
	bnd.IP = containerIPv4
	return true
}

// validatePortBindingIPv6 validates the port binding, populates the missing Host IP field and returns true
// if this is a valid IPv6 binding, else returns false
func (n *bridgeNetwork) validatePortBindingIPv6(bnd *types.PortBinding, containerIP, defHostIP net.IP) bool {
	// Return early if there is no container endpoint
	if containerIP == nil {
		return false
	}
	// Return early if there is a valid Host IP, which is a IPv4 address
	if len(bnd.HostIP) > 0 && bnd.HostIP.To4() != nil {
		return false
	}

	// Setup a binding to  "::" if Host IP is empty and the default binding IP is 0.0.0.0
	if len(bnd.HostIP) == 0 {
		if defHostIP.Equal(net.IPv4zero) {
			bnd.HostIP = net.IPv6zero
			// If the default binding IP is an IPv6 address, use it
		} else if defHostIP.To4() == nil {
			bnd.HostIP = defHostIP
			// Return false if default binding ip is an IPv4 address
		} else {
			return false
		}
	}
	bnd.IP = containerIP
	return true
}

func (n *bridgeNetwork) allocatePort(bnd *types.PortBinding, ulPxyEnabled bool) error {
	var (
		host net.Addr
		err  error
	)

	// Adjust HostPortEnd if this is not a range.
	if bnd.HostPortEnd == 0 {
		bnd.HostPortEnd = bnd.HostPort
	}

	// Construct the container side transport address
	container, err := bnd.ContainerAddr()
	if err != nil {
		return err
	}

	portmapper := n.portMapper

	if bnd.HostIP.To4() == nil {
		portmapper = n.portMapperV6
	}

	// Try up to maxAllocatePortAttempts times to get a port that's not already allocated.
	for i := 0; i < maxAllocatePortAttempts; i++ {
		if host, err = portmapper.MapRange(container, bnd.HostIP, int(bnd.HostPort), int(bnd.HostPortEnd), ulPxyEnabled); err == nil {
			break
		}
		// There is no point in immediately retrying to map an explicitly chosen port.
		if bnd.HostPort != 0 {
			log.G(context.TODO()).Warnf("Failed to allocate and map port %d-%d: %s", bnd.HostPort, bnd.HostPortEnd, err)
			break
		}
		log.G(context.TODO()).Warnf("Failed to allocate and map port: %s, retry: %d", err, i+1)
	}
	if err != nil {
		return err
	}

	// Save the host port (regardless it was or not specified in the binding)
	switch netAddr := host.(type) {
	case *net.TCPAddr:
		bnd.HostPort = uint16(host.(*net.TCPAddr).Port)
		return nil
	case *net.UDPAddr:
		bnd.HostPort = uint16(host.(*net.UDPAddr).Port)
		return nil
	case *sctp.SCTPAddr:
		bnd.HostPort = uint16(host.(*sctp.SCTPAddr).Port)
		return nil
	default:
		// For completeness
		return ErrUnsupportedAddressType(fmt.Sprintf("%T", netAddr))
	}
}

func (n *bridgeNetwork) releasePorts(ep *bridgeEndpoint) error {
	return n.releasePortsInternal(ep.portMapping)
}

func (n *bridgeNetwork) releasePortsInternal(bindings []types.PortBinding) error {
	var errorBuf bytes.Buffer

	// Attempt to release all port bindings, do not stop on failure
	for _, m := range bindings {
		if err := n.releasePort(m); err != nil {
			errorBuf.WriteString(fmt.Sprintf("\ncould not release %v because of %v", m, err))
		}
	}

	if errorBuf.Len() != 0 {
		return errors.New(errorBuf.String())
	}
	return nil
}

func (n *bridgeNetwork) releasePort(bnd types.PortBinding) error {
	// Construct the host side transport address
	host, err := bnd.HostAddr()
	if err != nil {
		return err
	}

	portmapper := n.portMapper

	if bnd.HostIP.To4() == nil {
		portmapper = n.portMapperV6
	}

	return portmapper.Unmap(host)
}

var (
	v6ListenableCached bool
	v6ListenableOnce   sync.Once
)

// IsV6Listenable returns true when `[::1]:0` is listenable.
// IsV6Listenable returns false mostly when the kernel was booted with `ipv6.disable=1` option.
func IsV6Listenable() bool {
	v6ListenableOnce.Do(func() {
		ln, err := net.Listen("tcp6", "[::1]:0")
		if err != nil {
			// When the kernel was booted with `ipv6.disable=1`,
			// we get err "listen tcp6 [::1]:0: socket: address family not supported by protocol"
			// https://github.com/moby/moby/issues/42288
			log.G(context.TODO()).Debugf("port_mapping: v6Listenable=false (%v)", err)
		} else {
			v6ListenableCached = true
			ln.Close()
		}
	})
	return v6ListenableCached
}
