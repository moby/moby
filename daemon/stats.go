package daemon

import (
	"encoding/json"

	"github.com/docker/docker/api/stats"
	"github.com/docker/docker/engine"
)

func (daemon *Daemon) ContainerStats(job *engine.Job) engine.Status {
	s, err := daemon.SubscribeToContainerStats(job.Args[0])
	if err != nil {
		return job.Error(err)
	}
	enc := json.NewEncoder(job.Stdout)
	for update := range s {
		ss := stats.ToStats(update.ContainerStats)
		ss.MemoryStats.Limit = uint64(update.MemoryLimit)
		ss.Read = update.Read
		ss.ClockTicks = update.ClockTicks
		ss.CpuStats.SystemUsage = update.SystemUsage
		if err := enc.Encode(ss); err != nil {
			// TODO: handle the specific broken pipe
			daemon.UnsubscribeToContainerStats(job.Args[0], s)
			return job.Error(err)
		}
	}
	return engine.StatusOK
}
