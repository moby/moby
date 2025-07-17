package portmapperapi

import (
	"context"

	"github.com/docker/docker/daemon/libnetwork/types"
)

type Firewaller interface {
	// AddPorts adds the configuration needed for NATing ports.
	AddPorts(ctx context.Context, pbs []types.PortBinding) error
	// DelPorts deletes the configuration needed for NATing ports.
	DelPorts(ctx context.Context, pbs []types.PortBinding) error
}
