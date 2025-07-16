// FIXME(thaJeztah): remove once we are a module; the go:build directive prevents go from downgrading language version to go1.16:
//go:build go1.23

package routed

import (
	"context"

	"github.com/containerd/log"
	"github.com/docker/docker/daemon/libnetwork/portmapperapi"
	"github.com/docker/docker/daemon/libnetwork/types"
	"github.com/docker/docker/internal/sliceutil"
)

const driverName = "routed"

// Register the "routed" port-mapper with libnetwork.
func Register(r portmapperapi.Registerer) error {
	return r.Register(driverName, NewPortMapper())
}

type PortMapper struct{}

func NewPortMapper() portmapperapi.PortMapper {
	return PortMapper{}
}

// MapPorts sets up firewall rules to allow direct remote access to pbs.
func (pm PortMapper) MapPorts(ctx context.Context, reqs []portmapperapi.PortBindingReq, fwn portmapperapi.Firewaller) ([]portmapperapi.PortBinding, error) {
	if len(reqs) == 0 {
		return nil, nil
	}

	res := make([]portmapperapi.PortBinding, 0, len(reqs))
	bindings := make([]types.PortBinding, 0, len(reqs))
	for _, c := range reqs {
		pb := portmapperapi.PortBinding{PortBinding: c.GetCopy()}
		if pb.HostPort != 0 || pb.HostPortEnd != 0 {
			log.G(ctx).WithFields(log.Fields{"mapping": pb}).Infof(
				"Host port ignored, because NAT is disabled")
			pb.HostPort = 0
			pb.HostPortEnd = 0
		}
		res = append(res, pb)
		bindings = append(bindings, pb.PortBinding)
	}

	if err := fwn.AddPorts(ctx, bindings); err != nil {
		return nil, err
	}

	return res, nil
}

// UnmapPorts removes firewall rules allowing direct remote access to the pbs.
func (pm PortMapper) UnmapPorts(ctx context.Context, pbs []portmapperapi.PortBinding, fwn portmapperapi.Firewaller) error {
	return fwn.DelPorts(ctx, sliceutil.Map(pbs, func(pb portmapperapi.PortBinding) types.PortBinding {
		return pb.PortBinding
	}))
}
