package nat

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"strconv"

	"github.com/containerd/log"
	"github.com/moby/moby/v2/daemon/libnetwork/internal/rlkclient"
	"github.com/moby/moby/v2/daemon/libnetwork/portallocator"
	"github.com/moby/moby/v2/daemon/libnetwork/portmapperapi"
	"github.com/moby/moby/v2/daemon/libnetwork/types"
)

const driverName = "nat"

type PortDriverClient interface {
	ChildHostIP(hostIP netip.Addr) netip.Addr
	AddPort(ctx context.Context, proto string, hostIP, childIP netip.Addr, hostPort int) (func() error, error)
}

// Register the "nat" port-mapper with libnetwork.
func Register(r portmapperapi.Registerer, cfg Config) error {
	return r.Register(driverName, NewPortMapper(cfg))
}

type PortMapper struct {
	// pdc is used to interact with rootlesskit port driver.
	pdc PortDriverClient
}

type Config struct {
	// RlkClient is called by MapPorts to determine the ChildHostIP and ask
	// rootlesskit to map ports in its netns.
	RlkClient PortDriverClient
}

func NewPortMapper(cfg Config) PortMapper {
	return PortMapper{
		pdc: cfg.RlkClient,
	}
}

// MapPorts allocates and binds host ports for the given cfg. The caller is
// responsible for ensuring that all entries in cfg have the same proto,
// container port, and host port range (their host addresses must differ).
func (pm PortMapper) MapPorts(ctx context.Context, cfg []portmapperapi.PortBindingReq) (_ []portmapperapi.PortBinding, retErr error) {
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

	bindings := make([]portmapperapi.PortBinding, 0, len(cfg))
	defer func() {
		if retErr != nil {
			if err := pm.UnmapPorts(ctx, bindings); err != nil {
				log.G(ctx).WithFields(log.Fields{
					"pbs":   bindings,
					"error": err,
				}).Warn("Failed to release port bindings")
			}
		}
	}()

	addrs := make([]net.IP, 0, len(cfg))
	for i := range cfg {
		cfg[i] = setChildHostIP(pm.pdc, cfg[i])
		addrs = append(addrs, cfg[i].ChildHostIP)
	}

	pa := portallocator.NewOSAllocator()
	allocatedPort, socks, err := pa.RequestPortsInRange(addrs, proto, int(hostPort), int(hostPortEnd))
	if err != nil {
		return nil, err
	}

	for i := range cfg {
		pb := portmapperapi.PortBinding{
			PortBinding: cfg[i].PortBinding.Copy(),
			BoundSocket: socks[i],
			ChildHostIP: cfg[i].ChildHostIP,
		}
		pb.PortBinding.HostPort = uint16(allocatedPort)
		pb.PortBinding.HostPortEnd = pb.HostPort

		childHIP, _ := netip.AddrFromSlice(cfg[i].ChildHostIP)
		pb.NAT = netip.AddrPortFrom(childHIP, pb.PortBinding.HostPort)

		bindings = append(bindings, pb)
	}

	if err := configPortDriver(ctx, bindings, pm.pdc); err != nil {
		return nil, err
	}

	return bindings, nil
}

func (pm PortMapper) UnmapPorts(ctx context.Context, pbs []portmapperapi.PortBinding) error {
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
	}
	for _, pb := range pbs {
		portallocator.Get().ReleasePort(pb.ChildHostIP, pb.Proto.String(), int(pb.HostPort))
	}
	return errors.Join(errs...)
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
