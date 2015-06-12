package daemon

import (
	"encoding/json"
	"io"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/daemon/execdriver"
)

func (daemon *Daemon) ContainerStats(name string, stream bool, out io.Writer) error {
	updates, err := daemon.SubscribeToContainerStats(name)
	if err != nil {
		return err
	}

	var preCpuStats types.CpuStats
	getStat := func(v interface{}) *types.Stats {
		update := v.(*execdriver.ResourceStats)
		ss := convertStatsToAPITypes(update.Stats)
		ss.PreCpuStats = preCpuStats
		ss.MemoryStats.Limit = uint64(update.MemoryLimit)
		ss.Read = update.Read
		ss.CpuStats.SystemUsage = update.SystemUsage
		preCpuStats = ss.CpuStats
		return ss
	}

	enc := json.NewEncoder(out)

	if !stream {
		// prime the cpu stats so they aren't 0 in the final output
		s := getStat(<-updates)

		// now pull stats again with the cpu stats primed
		s = getStat(<-updates)
		err := enc.Encode(s)
		daemon.UnsubscribeToContainerStats(name, updates)
		return err
	}

	for v := range updates {
		s := getStat(v)
		if err := enc.Encode(s); err != nil {
			// TODO: handle the specific broken pipe
			daemon.UnsubscribeToContainerStats(name, updates)
			return err
		}
	}
	return nil
}
