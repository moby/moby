package routed

import (
	"context"

	"github.com/containerd/log"
	"github.com/moby/moby/v2/daemon/libnetwork/portmapperapi"
)

const driverName = "routed"

// Register the "routed" port-mapper with libnetwork.
func Register(r portmapperapi.Registerer) error {
	return r.Register(driverName, NewPortMapper())
}

type PortMapper struct{}

func NewPortMapper() PortMapper {
	return PortMapper{}
}

// MapPorts sets up firewall rules to allow direct remote access to pbs.
func (pm PortMapper) MapPorts(ctx context.Context, reqs []portmapperapi.PortBindingReq) ([]portmapperapi.PortBinding, error) {
	if len(reqs) == 0 {
		return nil, nil
	}

	res := make([]portmapperapi.PortBinding, 0, len(reqs))
	for _, c := range reqs {
		pb := portmapperapi.PortBinding{
			PortBinding: c.Copy(),
			Forwarding:  true,
		}
		if pb.HostPort != 0 || pb.HostPortEnd != 0 {
			log.G(ctx).WithFields(log.Fields{"mapping": pb}).Infof(
				"Host port ignored, because NAT is disabled")
			pb.HostPort = 0
			pb.HostPortEnd = 0
		}
		res = append(res, pb)
	}

	return res, nil
}

// UnmapPorts removes firewall rules allowing direct remote access to the pbs.
func (pm PortMapper) UnmapPorts(ctx context.Context, pbs []portmapperapi.PortBinding) error {
	return nil
}
