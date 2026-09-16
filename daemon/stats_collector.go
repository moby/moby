package daemon

import (
	"time"

	"github.com/moby/moby/v2/daemon/stats"
)

// newStatsCollector returns a new statsCollector that collections
// stats for a registered container at the specified interval.
// The collector allows non-running containers to be added
// and will start processing stats when they are started.
func (daemon *Daemon) newStatsCollector(interval time.Duration) *stats.Collector {
	s := stats.NewCollector(daemon, interval)
	go s.Run()
	return s
}
