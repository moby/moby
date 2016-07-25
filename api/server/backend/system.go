package backend

import "github.com/docker/engine-api/types"

// SystemInfo returns information about the host server the daemon
// is running on, as well as the current cluster state.
func (b *Backend) SystemInfo() (*types.Info, error) {
	info, err := b.Daemon.SystemInfo()
	if err != nil {
		return nil, err
	}
	if b.clusterProvider != nil {
		info.Swarm = b.clusterProvider.Info()
	}
	return info, nil
}
