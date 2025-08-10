package nat

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"os"
	"strconv"
	"syscall"

	"github.com/containerd/log"
	"github.com/moby/moby/v2/daemon/libnetwork/internal/rlkclient"
	"github.com/moby/moby/v2/daemon/libnetwork/portallocator"
	"github.com/moby/moby/v2/daemon/libnetwork/portmapperapi"
	"github.com/moby/moby/v2/daemon/libnetwork/types"
)

const (
	driverName              = "nat"
	maxAllocatePortAttempts = 10
)

type PortDriverClient interface {
	ChildHostIP(hostIP netip.Addr) netip.Addr
	AddPort(ctx context.Context, proto string, hostIP, childIP netip.Addr, hostPort int) (func() error, error)
}

type proxyStarter func(types.PortBinding, *os.File) (func() error, error)

// Register the "nat" port-mapper with libnetwork.
func Register(r portmapperapi.Registerer, cfg Config) error {
	return r.Register(driverName, NewPortMapper(cfg))
}

type PortMapper struct {
	// pdc is used to interact with rootlesskit port driver.
	pdc         PortDriverClient
	startProxy  proxyStarter
	enableProxy bool
}

type Config struct {
	// RlkClient is called by MapPorts to determine the ChildHostIP and ask
	// rootlesskit to map ports in its netns.
	RlkClient   PortDriverClient
	StartProxy  proxyStarter
	EnableProxy bool
}

func NewPortMapper(cfg Config) PortMapper {
	return PortMapper{
		pdc:         cfg.RlkClient,
		startProxy:  cfg.StartProxy,
		enableProxy: cfg.EnableProxy,
	}
}

// MapPorts allocates and binds host ports for the given cfg. The caller is
// responsible for ensuring that all entries in cfg map the same proto,
// container port, and host port range (their host addresses must differ).
func (pm PortMapper) MapPorts(ctx context.Context, cfg []portmapperapi.PortBindingReq, fwn portmapperapi.Firewaller) ([]portmapperapi.PortBinding, error) {
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
	var bindings []portmapperapi.PortBinding
	var err error
	for i := 0; i < maxAllocatePortAttempts; i++ {
		bindings, err = pm.attemptBindHostPorts(ctx, cfg, proto, hostPort, hostPortEnd, fwn)
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
	if pm.enableProxy {
		for i := range bindings {
			if bindings[i].BoundSocket == nil || bindings[i].RootlesskitUnsupported || bindings[i].StopProxy != nil {
				continue
			}
			var err error
			bindings[i].StopProxy, err = pm.startProxy(
				bindings[i].ChildPortBinding(), bindings[i].BoundSocket,
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

func (pm PortMapper) UnmapPorts(ctx context.Context, pbs []portmapperapi.PortBinding, fwn portmapperapi.Firewaller) error {
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
				errs = append(errs, fmt.Errorf("failed to stop userland proxy: %w", err))
			}
		}
	}
	if err := fwn.DelPorts(ctx, mergeChildHostIPs(pbs)); err != nil {
		errs = append(errs, err)
	}
	for _, pb := range pbs {
		portallocator.Get().ReleasePort(pb.ChildHostIP, pb.Proto.String(), int(pb.HostPort))
	}
	return errors.Join(errs...)
}

// attemptBindHostPorts allocates host ports for each NAT port mapping, and
// reserves those ports by binding them.
//
// If the allocator doesn't have an available port in the required range, or the
// port can't be bound (perhaps because another process has already bound it),
// all resources are released and an error is returned. When ports are
// successfully reserved, a PortBinding is returned for each mapping.
func (pm PortMapper) attemptBindHostPorts(
	ctx context.Context,
	cfg []portmapperapi.PortBindingReq,
	proto types.Protocol,
	hostPortStart, hostPortEnd uint16,
	fwn portmapperapi.Firewaller,
) (_ []portmapperapi.PortBinding, retErr error) {
	var err error
	var port int

	addrs := make([]net.IP, 0, len(cfg))
	for i := range cfg {
		cfg[i] = setChildHostIP(pm.pdc, cfg[i])
		addrs = append(addrs, cfg[i].ChildHostIP)
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
			if err := pm.UnmapPorts(ctx, res, fwn); err != nil {
				log.G(ctx).WithFields(log.Fields{
					"pbs":   res,
					"error": err,
				}).Warn("Failed to release port bindings")
			}
		}
	}()

	for i := range cfg {
		pb := portmapperapi.PortBinding{
			PortBinding: cfg[i].PortBinding.Copy(),
			BoundSocket: socks[i],
			ChildHostIP: cfg[i].ChildHostIP,
		}
		pb.PortBinding.HostPort = uint16(port)
		pb.PortBinding.HostPortEnd = pb.HostPort
		res = append(res, pb)
	}

	if err := configPortDriver(ctx, res, pm.pdc); err != nil {
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
	if err := listenBoundPorts(res, pm.enableProxy); err != nil {
		return nil, err
	}
	return res, nil
}

func setChildHostIP(pdc PortDriverClient, req portmapperapi.PortBindingReq) portmapperapi.PortBindingReq {
	if pdc == nil {
		req.ChildHostIP = req.HostIP
		return req
	}
	hip, _ := netip.AddrFromSlice(req.HostIP)
	req.ChildHostIP = pdc.ChildHostIP(hip).AsSlice()
	return req
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

// configPortDriver passes the port binding's details to rootlesskit, and updates the
// port binding with callbacks to remove the rootlesskit config (or marks the binding as
// unsupported by rootlesskit).
func configPortDriver(ctx context.Context, pbs []portmapperapi.PortBinding, pdc PortDriverClient) error {
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

func listenBoundPorts(pbs []portmapperapi.PortBinding, proxyEnabled bool) error {
	for i := range pbs {
		if pbs[i].BoundSocket == nil || pbs[i].RootlesskitUnsupported || pbs[i].Proto == types.UDP {
			continue
		}
		rc, err := pbs[i].BoundSocket.SyscallConn()
		if err != nil {
			return fmt.Errorf("raw conn not available on %d socket: %w", pbs[i].Proto, err)
		}
		if errC := rc.Control(func(fd uintptr) {
			somaxconn := 0
			// SCTP sockets do not support somaxconn=0
			if proxyEnabled || pbs[i].Proto == types.SCTP {
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
