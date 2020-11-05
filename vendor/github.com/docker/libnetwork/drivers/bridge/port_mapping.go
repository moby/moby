package bridge

import (
	"bytes"
	"errors"
	"fmt"
	"net"

	"github.com/docker/libnetwork/types"
	"github.com/ishidawataru/sctp"
	"github.com/sirupsen/logrus"
)

var (
	defaultBindingIP   = net.IPv4(0, 0, 0, 0)
	defaultBindingIPV6 = net.ParseIP("::")
)

func (n *bridgeNetwork) allocatePorts(ep *bridgeEndpoint, reqDefBindIP net.IP, ulPxyEnabled bool) ([]types.PortBinding, error) {
	if ep.extConnConfig == nil || ep.extConnConfig.PortBindings == nil {
		return nil, nil
	}

	defHostIP := defaultBindingIP
	if reqDefBindIP != nil {
		defHostIP = reqDefBindIP
	}

	// IPv4 port binding including user land proxy
	pb, err := n.allocatePortsInternal(ep.extConnConfig.PortBindings, ep.addr.IP, defHostIP, ulPxyEnabled)
	if err != nil {
		return nil, err
	}

	// IPv6 port binding excluding user land proxy
	if n.driver.config.EnableIP6Tables && ep.addrv6 != nil {
		// TODO IPv6 custom default binding IP
		pbv6, err := n.allocatePortsInternal(ep.extConnConfig.PortBindings, ep.addrv6.IP, defaultBindingIPV6, false)
		if err != nil {
			// ensure we clear the previous allocated IPv4 ports
			n.releasePortsInternal(pb)
			return nil, err
		}

		pb = append(pb, pbv6...)
	}
	return pb, nil
}

func (n *bridgeNetwork) allocatePortsInternal(bindings []types.PortBinding, containerIP, defHostIP net.IP, ulPxyEnabled bool) ([]types.PortBinding, error) {
	bs := make([]types.PortBinding, 0, len(bindings))
	for _, c := range bindings {
		b := c.GetCopy()
		if err := n.allocatePort(&b, containerIP, defHostIP, ulPxyEnabled); err != nil {
			// On allocation failure, release previously allocated ports. On cleanup error, just log a warning message
			if cuErr := n.releasePortsInternal(bs); cuErr != nil {
				logrus.Warnf("Upon allocation failure for %v, failed to clear previously allocated port bindings: %v", b, cuErr)
			}
			return nil, err
		}
		bs = append(bs, b)
	}
	return bs, nil
}

func (n *bridgeNetwork) allocatePort(bnd *types.PortBinding, containerIP, defHostIP net.IP, ulPxyEnabled bool) error {
	var (
		host net.Addr
		err  error
	)

	// Store the container interface address in the operational binding
	bnd.IP = containerIP

	// Adjust the host address in the operational binding
	if len(bnd.HostIP) == 0 {
		bnd.HostIP = defHostIP
	}

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

	if containerIP.To4() == nil {
		portmapper = n.portMapperV6
	}

	// Try up to maxAllocatePortAttempts times to get a port that's not already allocated.
	for i := 0; i < maxAllocatePortAttempts; i++ {
		if host, err = portmapper.MapRange(container, bnd.HostIP, int(bnd.HostPort), int(bnd.HostPortEnd), ulPxyEnabled); err == nil {
			break
		}
		// There is no point in immediately retrying to map an explicitly chosen port.
		if bnd.HostPort != 0 {
			logrus.Warnf("Failed to allocate and map port %d-%d: %s", bnd.HostPort, bnd.HostPortEnd, err)
			break
		}
		logrus.Warnf("Failed to allocate and map port: %s, retry: %d", err, i+1)
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
