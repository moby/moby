package daemon

import (
	"encoding/json"
	"errors"
	"runtime"

	"github.com/docker/docker/api/types/backend"
	"github.com/docker/docker/pkg/ioutils"
	"github.com/docker/docker/pkg/version"
	"github.com/docker/engine-api/types"
	"github.com/docker/engine-api/types/versions/v1p20"
)

// ContainerStats writes information about the container to the stream
// given in the config object.
func (daemon *Daemon) ContainerStats(prefixOrName string, config *backend.ContainerStatsConfig) error {
	if runtime.GOOS == "windows" {
		return errors.New("Windows does not support stats")
	}
	// Remote API version (used for backwards compatibility)
	apiVersion := version.Version(config.Version)

	container, err := daemon.GetContainer(prefixOrName)
	if err != nil {
		return err
	}

	// If the container is not running and requires no stream, return an empty stats.
	if !container.IsRunning() && !config.Stream {
		return json.NewEncoder(config.OutStream).Encode(&types.Stats{})
	}

	outStream := config.OutStream
	if config.Stream {
		wf := ioutils.NewWriteFlusher(outStream)
		defer wf.Close()
		wf.Flush()
		outStream = wf
	}

	var preCPUStats types.CPUStats
	getStatJSON := func(v interface{}) *types.StatsJSON {
		ss := v.(*types.StatsJSON)
		ss.PreCPUStats = preCPUStats
		// ss.MemoryStats.Limit = uint64(update.MemoryLimit)
		preCPUStats = ss.CPUStats
		return ss
	}

	enc := json.NewEncoder(outStream)

	updates := daemon.subscribeToContainerStats(container)
	defer daemon.unsubscribeToContainerStats(container, updates)

	noStreamFirstFrame := true
	for {
		select {
		case v, ok := <-updates:
			if !ok {
				return nil
			}

			var statsJSON interface{}
			statsJSONPost120 := getStatJSON(v)
			if apiVersion.LessThan("1.21") {
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
				for _, v := range statsJSONPost120.Networks {
					rxBytes += v.RxBytes
					rxPackets += v.RxPackets
					rxErrors += v.RxErrors
					rxDropped += v.RxDropped
					txBytes += v.TxBytes
					txPackets += v.TxPackets
					txErrors += v.TxErrors
					txDropped += v.TxDropped
				}
				statsJSON = &v1p20.StatsJSON{
					Stats: statsJSONPost120.Stats,
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
			} else {
				statsJSON = statsJSONPost120
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
