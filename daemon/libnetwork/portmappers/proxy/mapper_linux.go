// FIXME(thaJeztah): remove once we are a module; the go:build directive prevents go from downgrading language version to go1.16:
//go:build go1.23

package proxy

import (
	"context"
	"errors"
	"fmt"
	"net"
	"syscall"

	"github.com/containerd/log"
	"github.com/docker/docker/daemon/libnetwork/portallocator"
	"github.com/docker/docker/daemon/libnetwork/portmapperapi"
	"github.com/docker/docker/daemon/libnetwork/types"
	"github.com/docker/docker/internal/sliceutil"
)

const (
	DriverName              = "proxy"
	maxAllocatePortAttempts = 10
)

func Register(r portmapperapi.Registerer, proxyMgr ProxyManager) error {
	return r.Register(DriverName, NewPortMapper(proxyMgr))
}

var _ portmapperapi.PortMapper = (*PortMapper)(nil)

type PortMapper struct {
	proxyMgr portmapperapi.ProxyManager
}

func NewPortMapper(proxyMgr portmapperapi.ProxyManager) *PortMapper {
	return &PortMapper{
		proxyMgr: proxyMgr,
	}
}

// MapPorts allocates and binds host ports for the given reqs. The caller is
// responsible for ensuring that all entries in reqs map the same proto,
// container port, and host port range (their host addresses must differ).
func (pm PortMapper) MapPorts(
	ctx context.Context,
	reqs []portmapperapi.PortBindingReq,
	_ portmapperapi.Firewaller,
) (_ []portmapperapi.PortBinding, retErr error) {
	if len(reqs) == 0 {
		return nil, nil
	}
	proto, port, hostPort, hostPortEnd := reqs[0].Proto, reqs[0].Port, reqs[0].HostPort, reqs[0].HostPortEnd
	for _, req := range reqs[1:] {
		if req.Proto != proto || req.Port != port || req.HostPort != hostPort || req.HostPortEnd != hostPortEnd {
			return nil, types.InternalErrorf("port binding mismatch %d/%s:%d-%d, %d/%s:%d-%d",
				port, proto, hostPort, hostPortEnd,
				port, req.Proto, req.HostPort, req.HostPortEnd)
		}
	}

	var bindings []portmapperapi.PortBinding
	defer func() {
		if retErr != nil {
			if err := pm.unmapPorts(bindings); err != nil {
				log.G(ctx).WithFields(log.Fields{
					"pbs":   bindings,
					"error": err,
				}).Warn("Failed to release port bindings")
			}
		}
	}()

	// Try up to maxAllocatePortAttempts times to get a port that's not already allocated.
	var err error
	for i := 0; i < maxAllocatePortAttempts; i++ {
		bindings, err = pm.attemptBindHostPorts(ctx, reqs, proto, hostPort, hostPortEnd)
		if err == nil {
			break
		}
		// There is no point in immediately retrying to map an explicitly chosen port.
		if hostPort != 0 && hostPort == hostPortEnd {
			log.G(ctx).WithError(err).Warnf("Failed to allocate and map port")
			return nil, err
		}
		log.G(ctx).WithFields(log.Fields{
			"error":   err,
			"attempt": i + 1,
		}).Warn("Failed to allocate and map port")
	}

	if err != nil {
		// If the retry budget is exhausted and no free port could be found, return
		// the latest error.
		return nil, err
	}

	// Start userland proxy processes.
	for i := range bindings {
		var err error
		bindings[i].Proxy, err = pm.proxyMgr.StartProxy(bindings[i].PortBinding, bindings[i].BoundSocket)
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

	return bindings, nil
}

// attemptBindHostPorts allocates host ports for each proxied port mapping, and
// reserves those ports by binding them.
//
// If the allocator doesn't have an available port in the required range, or the
// port can't be bound (perhaps because another process has already bound it),
// all resources are released and an error is returned. When ports are
// successfully reserved, a PortBinding is returned for each mapping.
func (pm PortMapper) attemptBindHostPorts(
	ctx context.Context,
	reqs []portmapperapi.PortBindingReq,
	proto types.Protocol,
	hostPortStart, hostPortEnd uint16,
) (_ []portmapperapi.PortBinding, retErr error) {
	addrs := sliceutil.Map(reqs, func(req portmapperapi.PortBindingReq) net.IP {
		return req.HostIP
	})

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

	res := make([]portmapperapi.PortBinding, 0, len(reqs))
	defer func() {
		if retErr != nil {
			if err := pm.unmapPorts(res); err != nil {
				log.G(ctx).WithFields(log.Fields{
					"pbs":   res,
					"error": err,
				}).Warn("Failed to release port bindings")
			}
		}
	}()

	for i := range reqs {
		pb := portmapperapi.PortBinding{
			PortBinding: reqs[i].PortBinding.GetCopy(),
			BoundSocket: socks[i],
		}
		pb.PortBinding.HostPort = uint16(port)
		pb.PortBinding.HostPortEnd = pb.HostPort
		res = append(res, pb)
	}

	if err := listenBoundPorts(res); err != nil {
		return nil, err
	}
	return res, nil
}

func listenBoundPorts(pbs []portmapperapi.PortBinding) error {
	for i := range pbs {
		if pbs[i].Proto == types.UDP {
			continue
		}
		rc, err := pbs[i].BoundSocket.SyscallConn()
		if err != nil {
			return fmt.Errorf("raw conn not available on %d socket: %w", pbs[i].Proto, err)
		}
		if errC := rc.Control(func(fd uintptr) {
			somaxconn := 0
			// SCTP sockets do not support somaxconn=0
			if pbs[i].Proto == types.SCTP {
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

func (pm PortMapper) UnmapPorts(_ context.Context, pbs []portmapperapi.PortBinding, _ portmapperapi.Firewaller) error {
	return pm.unmapPorts(pbs)
}

func (pm PortMapper) unmapPorts(pbs []portmapperapi.PortBinding) error {
	var errs []error
	for _, pb := range pbs {
		if pb.BoundSocket != nil {
			if err := pb.BoundSocket.Close(); err != nil {
				errs = append(errs, fmt.Errorf("failed to close socket for port mapping %s: %w", pb, err))
			}
		}
		if pb.Proxy != nil {
			if err := pb.Proxy.Stop(); err != nil {
				errs = append(errs, fmt.Errorf("failed to stop userland proxy: %w", err))
			}
		}

		portallocator.Get().ReleasePort(pb.HostIP, pb.Proto.String(), int(pb.HostPort))
	}
	return errors.Join(errs...)
}
