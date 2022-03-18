package daemon // import "github.com/docker/docker/daemon"

import (
	"context"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/container"
)

// Windows network stats are obtained directly through HCS, hence this is a no-op.
func (daemon *Daemon) getNetworkStats(ctx context.Context, c *container.Container) (map[string]types.NetworkStats, error) {
	return make(map[string]types.NetworkStats), nil
}
