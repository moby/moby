package daemon

import (
	"encoding/json"
	"io"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/daemon/execdriver"
	"github.com/docker/libnetwork/osl"
	"github.com/opencontainers/runc/libcontainer"
)

// ContainerStatsConfig holds information for configuring the runtime
// behavior of a daemon.ContainerStats() call.
type ContainerStatsConfig struct {
	Stream    bool
	OutStream io.Writer
	Stop      <-chan bool
}

// ContainerStats writes information about the container to the stream
// given in the config object.
func (daemon *Daemon) ContainerStats(container *Container, config *ContainerStatsConfig) error {
	updates, err := daemon.subscribeToContainerStats(container)
	if err != nil {
		return err
	}

	if config.Stream {
		config.OutStream.Write(nil)
	}

	var preCPUStats types.CPUStats
	getStat := func(v interface{}) *types.Stats {
		update := v.(*execdriver.ResourceStats)
		// Retrieve the nw statistics from libnetwork and inject them in the Stats
		if nwStats, err := daemon.getNetworkStats(container); err == nil {
			update.Stats.Interfaces = nwStats
		}
		ss := convertStatsToAPITypes(update.Stats)
		ss.PreCPUStats = preCPUStats
		ss.MemoryStats.Limit = uint64(update.MemoryLimit)
		ss.Read = update.Read
		ss.CPUStats.SystemUsage = update.SystemUsage
		preCPUStats = ss.CPUStats
		return ss
	}

	enc := json.NewEncoder(config.OutStream)

	defer daemon.unsubscribeToContainerStats(container, updates)

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

func (daemon *Daemon) getNetworkStats(c *Container) ([]*libcontainer.NetworkInterface, error) {
	var list []*libcontainer.NetworkInterface

	sb, err := daemon.netController.SandboxByID(c.NetworkSettings.SandboxID)
	if err != nil {
		return list, err
	}

	stats, err := sb.Statistics()
	if err != nil {
		return list, err
	}

	// Convert libnetwork nw stats into libcontainer nw stats
	for ifName, ifStats := range stats {
		list = append(list, convertLnNetworkStats(ifName, ifStats))
	}

	return list, nil
}

func convertLnNetworkStats(name string, stats *osl.InterfaceStatistics) *libcontainer.NetworkInterface {
	n := &libcontainer.NetworkInterface{Name: name}
	n.RxBytes = stats.RxBytes
	n.RxPackets = stats.RxPackets
	n.RxErrors = stats.RxErrors
	n.RxDropped = stats.RxDropped
	n.TxBytes = stats.TxBytes
	n.TxPackets = stats.TxPackets
	n.TxErrors = stats.TxErrors
	n.TxDropped = stats.TxDropped
	return n
}
