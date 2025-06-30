//go:build windows

package windows

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net"

	"github.com/containerd/log"
	"github.com/moby/moby/v2/daemon/libnetwork/portmapper"
	"github.com/moby/moby/v2/daemon/libnetwork/types"
)

const maxAllocatePortAttempts = 10

// AllocatePorts allocates ports specified in bindings from the portMapper
func AllocatePorts(portMapper *portmapper.PortMapper, bindings []types.PortBinding) ([]types.PortBinding, error) {
	bs := make([]types.PortBinding, 0, len(bindings))
	for _, c := range bindings {
		b, err := allocatePort(portMapper, c)
		if err != nil {
			// On allocation failure, release previously allocated ports. On cleanup error, just log a warning message
			if cuErr := ReleasePorts(portMapper, bs); cuErr != nil {
				log.G(context.TODO()).Warnf("Upon allocation failure for %v, failed to clear previously allocated port bindings: %v", b, cuErr)
			}
			return nil, err
		}
		bs = append(bs, b)
	}
	return bs, nil
}

func allocatePort(portMapper *portmapper.PortMapper, bnd types.PortBinding) (types.PortBinding, error) {
	// Windows does not support a host ip for port bindings (this is validated in ConvertPortBindings()).
	// If the HostIP is nil, force it to be 0.0.0.0 for use as the key in portMapper.
	if bnd.HostIP == nil {
		bnd.HostIP = net.IPv4zero
	}

	// Adjust HostPortEnd if this is not a range.
	if bnd.HostPortEnd == 0 {
		bnd.HostPortEnd = bnd.HostPort
	}

	// Try up to maxAllocatePortAttempts times to get a port that's not already allocated.
	var allocatedPort int
	var err error
	for i := 0; i < maxAllocatePortAttempts; i++ {
		allocatedPort, err = portMapper.MapRange(bnd.HostIP, bnd.Proto, int(bnd.HostPort), int(bnd.HostPortEnd))
		if err == nil {
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
		return types.PortBinding{}, err
	}

	bnd.HostPort = uint16(allocatedPort)
	bnd.HostPortEnd = uint16(allocatedPort)

	return bnd, nil
}

// ReleasePorts releases ports specified in bindings from the portMapper
func ReleasePorts(portMapper *portmapper.PortMapper, bindings []types.PortBinding) error {
	var errorBuf bytes.Buffer

	// Attempt to release all port bindings, do not stop on failure
	for _, m := range bindings {
		if err := releasePort(portMapper, m); err != nil {
			errorBuf.WriteString(fmt.Sprintf("\ncould not release %v because of %v", m, err))
		}
	}

	if errorBuf.Len() != 0 {
		return errors.New(errorBuf.String())
	}
	return nil
}

func releasePort(portMapper *portmapper.PortMapper, bnd types.PortBinding) error {
	return portMapper.Unmap(bnd.HostIP, bnd.Proto, int(bnd.HostPort))
}
