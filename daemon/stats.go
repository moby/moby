package daemon

import (
	"encoding/json"
	"io"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/versions/v1p20"
	"github.com/docker/docker/daemon/execdriver"
	"github.com/docker/docker/pkg/version"
	"github.com/docker/libnetwork/osl"
	"github.com/opencontainers/runc/libcontainer"
)

// ContainerStatsConfig holds information for configuring the runtime
// behavior of a daemon.ContainerStats() call.
type ContainerStatsConfig struct {
	Stream    bool
	OutStream io.Writer
	Stop      <-chan bool
	Version   version.Version
}

// ContainerStats writes information about the container to the stream
// given in the config object.
func (daemon *Daemon) ContainerStats(prefixOrName string, config *ContainerStatsConfig) error {

	container, err := daemon.Get(prefixOrName)
	if err != nil {
		return err
	}

	// If the container is not running and requires no stream, return an empty stats.
	if !container.IsRunning() && !config.Stream {
		return json.NewEncoder(config.OutStream).Encode(&types.Stats{})
	}

	updates, err := daemon.subscribeToContainerStats(container)
	if err != nil {
		return err
	}

	if config.Stream {
		// Write an empty chunk of data.
		// This is to ensure that the HTTP status code is sent immediately,
		// even if the container has not yet produced any data.
		config.OutStream.Write(nil)
	}

	var preCPUStats types.CPUStats
	getStatJSON := func(v interface{}) *types.StatsJSON {
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

			statsJSON := getStatJSON(v)
			if config.Version.LessThan("1.21") {
				var (
					rxBytes   uint64
					rxPackets uint64
					rxErrors  uint64
					rxDropped uint64
					txBytes   uint64
					txPackets uint64
					txErrors  uint64
					txDropped uint64
				)
				for _, v := range statsJSON.Networks {
					rxBytes += v.RxBytes
					rxPackets += v.RxPackets
					rxErrors += v.RxErrors
					rxDropped += v.RxDropped
					txBytes += v.TxBytes
					txPackets += v.TxPackets
					txErrors += v.TxErrors
					txDropped += v.TxDropped
				}
				statsJSONPre121 := &v1p20.StatsJSON{
					Stats: statsJSON.Stats,
					Network: types.NetworkStats{
						RxBytes:   rxBytes,
						RxPackets: rxPackets,
						RxErrors:  rxErrors,
						RxDropped: rxDropped,
						TxBytes:   txBytes,
						TxPackets: txPackets,
						TxErrors:  txErrors,
						TxDropped: txDropped,
					},
				}

				if !config.Stream && noStreamFirstFrame {
					// prime the cpu stats so they aren't 0 in the final output
					noStreamFirstFrame = false
					continue
				}

				if err := enc.Encode(statsJSONPre121); err != nil {
					return err
				}

				if !config.Stream {
					return nil
				}
			}

			if !config.Stream && noStreamFirstFrame {
				// prime the cpu stats so they aren't 0 in the final output
				noStreamFirstFrame = false
				continue
			}

			if err := enc.Encode(statsJSON); err != nil {
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
