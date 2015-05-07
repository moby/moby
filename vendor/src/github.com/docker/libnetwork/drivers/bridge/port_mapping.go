package bridge

import (
	"bytes"
	"errors"
	"fmt"
	"net"

	"github.com/Sirupsen/logrus"
	"github.com/docker/libnetwork/netutils"
	"github.com/docker/libnetwork/sandbox"
)

var (
	defaultBindingIP = net.IPv4(0, 0, 0, 0)
)

func allocatePorts(epConfig *EndpointConfiguration, intf *sandbox.Interface, reqDefBindIP net.IP, ulPxyEnabled bool) ([]netutils.PortBinding, error) {
	if epConfig == nil || epConfig.PortBindings == nil {
		return nil, nil
	}

	defHostIP := defaultBindingIP
	if reqDefBindIP != nil {
		defHostIP = reqDefBindIP
	}

	return allocatePortsInternal(epConfig.PortBindings, intf.Address.IP, defHostIP, ulPxyEnabled)
}

func allocatePortsInternal(bindings []netutils.PortBinding, containerIP, defHostIP net.IP, ulPxyEnabled bool) ([]netutils.PortBinding, error) {
	bs := make([]netutils.PortBinding, 0, len(bindings))
	for _, c := range bindings {
		b := c.GetCopy()
		if err := allocatePort(&b, containerIP, defHostIP, ulPxyEnabled); err != nil {
			// On allocation failure, release previously allocated ports. On cleanup error, just log a warning message
			if cuErr := releasePortsInternal(bs); cuErr != nil {
				logrus.Warnf("Upon allocation failure for %v, failed to clear previously allocated port bindings: %v", b, cuErr)
			}
			return nil, err
		}
		bs = append(bs, b)
	}
	return bs, nil
}

func allocatePort(bnd *netutils.PortBinding, containerIP, defHostIP net.IP, ulPxyEnabled bool) error {
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

	// Construct the container side transport address
	container, err := bnd.ContainerAddr()
	if err != nil {
		return err
	}

	// Try up to maxAllocatePortAttempts times to get a port that's not already allocated.
	for i := 0; i < maxAllocatePortAttempts; i++ {
		if host, err = portMapper.Map(container, bnd.HostIP, int(bnd.HostPort), ulPxyEnabled); err == nil {
			break
		}
		// There is no point in immediately retrying to map an explicitly chosen port.
		if bnd.HostPort != 0 {
			logrus.Warnf("Failed to allocate and map port %d: %s", bnd.HostPort, err)
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
	default:
		// For completeness
		return ErrUnsupportedAddressType(fmt.Sprintf("%T", netAddr))
	}
}

func releasePorts(ep *bridgeEndpoint) error {
	return releasePortsInternal(ep.portMapping)
}

func releasePortsInternal(bindings []netutils.PortBinding) error {
	var errorBuf bytes.Buffer

	// Attempt to release all port bindings, do not stop on failure
	for _, m := range bindings {
		if err := releasePort(m); err != nil {
			errorBuf.WriteString(fmt.Sprintf("\ncould not release %v because of %v", m, err))
		}
	}

	if errorBuf.Len() != 0 {
		return errors.New(errorBuf.String())
	}
	return nil
}

func releasePort(bnd netutils.PortBinding) error {
	// Construct the host side transport address
	host, err := bnd.HostAddr()
	if err != nil {
		return err
	}
	return portMapper.Unmap(host)
}
