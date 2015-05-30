package daemon

import (
	"encoding/json"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/daemon/execdriver"
	"io"
)

func (daemon *Daemon) ContainerStats(name string, stream bool, out io.Writer) error {
	updates, err := daemon.SubscribeToContainerStats(name)
	if err != nil {
		return err
	}
	var pre_cpu_stats types.CpuStats
	for first_v := range updates {
		first_update := first_v.(*execdriver.ResourceStats)
		first_stats := convertToAPITypes(first_update.Stats)
		pre_cpu_stats = first_stats.CpuStats
		pre_cpu_stats.SystemUsage = first_update.SystemUsage
		break
	}
	enc := json.NewEncoder(out)
	for v := range updates {
		update := v.(*execdriver.ResourceStats)
		// Retrieve the nw statistics from libnetwork and inject them in the Stats
		if nwStats, err := daemon.getNetworkStats(name); err == nil {
			update.Stats.Interfaces = nwStats
		}
		ss := convertToAPITypes(update.Stats)
		ss.PreCpuStats = pre_cpu_stats
		ss.MemoryStats.Limit = uint64(update.MemoryLimit)
		ss.Read = update.Read
		ss.CpuStats.SystemUsage = update.SystemUsage
		pre_cpu_stats = ss.CpuStats
		if err := enc.Encode(ss); err != nil {
			// TODO: handle the specific broken pipe
			daemon.UnsubscribeToContainerStats(name, updates)
			return err
		}
	}
	return nil
}
