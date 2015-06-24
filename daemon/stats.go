package daemon

import (
	"encoding/json"
	"io"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/daemon/execdriver"
)

type ContainerStatsConfig struct {
	Stream    bool
	OutStream io.Writer
	Stop      <-chan bool
}

func (daemon *Daemon) ContainerStats(name string, config *ContainerStatsConfig) error {
	updates, err := daemon.SubscribeToContainerStats(name)
	if err != nil {
		return err
	}

	if config.Stream {
		config.OutStream.Write(nil)
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

	enc := json.NewEncoder(config.OutStream)

	defer daemon.UnsubscribeToContainerStats(name, updates)

	noStreamFirstFrame := true
	for {
		select {
		case v, ok := <-updates:
			if !ok {
				return nil
			}

			s := getStat(v)
			if !config.Stream && noStreamFirstFrame {
				// prime the cpu stats so they aren't 0 in the final output
				noStreamFirstFrame = false
				continue
			}

			if err := enc.Encode(s); err != nil {
				return err
			}

			if !config.Stream {
				return nil
			}
		case <-config.Stop:
			return nil
		}
	}
}
