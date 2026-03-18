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

// MapPorts returns a PortBinding for every PortBindingReq received, with Forwarding enabled for each. If a HostPort is
// specified, it's logged and ignored.
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

func (pm PortMapper) UnmapPorts(_ context.Context, _ []portmapperapi.PortBinding) error {
	return nil
}
