package daemon // import "github.com/docker/docker/daemon"

import (
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/container"
)

// Windows network stats are obtained directly through HCS, hence this is a no-op.
func (daemon *Daemon) getNetworkStats(c *container.Container) (map[string]types.NetworkStats, error) {
	return make(map[string]types.NetworkStats), nil
}

// getSystemCPUUsage returns the host system's cpu usage in
// nanoseconds and number of online CPUs. An error is returned
// if the format of the underlying file does not match.
// This is a no-op on Windows.
func getSystemCPUUsage() (uint64, uint32, error) {
	return 0, 0, nil
}
