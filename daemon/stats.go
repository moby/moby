package daemon // import "github.com/docker/docker/daemon"

import (
	"context"
	"encoding/json"
	"errors"
	"runtime"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/backend"
	"github.com/docker/docker/api/types/versions"
	"github.com/docker/docker/api/types/versions/v1p20"
	"github.com/docker/docker/container"
	"github.com/docker/docker/errdefs"
	"github.com/docker/docker/pkg/ioutils"
)

// ContainerStats writes information about the container to the stream
// given in the config object.
func (daemon *Daemon) ContainerStats(ctx context.Context, prefixOrName string, config *backend.ContainerStatsConfig) error {
	// Engine API version (used for backwards compatibility)
	apiVersion := config.Version

	if isWindows && versions.LessThan(apiVersion, "1.21") {
		return errors.New("API versions pre v1.21 do not support stats on Windows")
	}

	ctr, err := daemon.GetContainer(ctx, prefixOrName)
	if err != nil {
		return err
	}

	if config.Stream && config.OneShot {
		return errdefs.InvalidParameter(errors.New("cannot have stream=true and one-shot=true"))
	}

	// If the container is either not running or restarting and requires no stream, return an empty stats.
	if (!ctr.IsRunning() || ctr.IsRestarting()) && !config.Stream {
		return json.NewEncoder(config.OutStream).Encode(&types.StatsJSON{
			Name: ctr.Name,
			ID:   ctr.ID,
		})
	}

	outStream := config.OutStream
	if config.Stream {
		wf := ioutils.NewWriteFlusher(outStream)
		defer wf.Close()
		wf.Flush()
		outStream = wf
	}

	var preCPUStats types.CPUStats
	var preRead time.Time
	getStatJSON := func(v interface{}) *types.StatsJSON {
		ss := v.(types.StatsJSON)
		ss.Name = ctr.Name
		ss.ID = ctr.ID
		ss.PreCPUStats = preCPUStats
		ss.PreRead = preRead
		preCPUStats = ss.CPUStats
		preRead = ss.Read
		return &ss
	}

	enc := json.NewEncoder(outStream)

	updates := daemon.subscribeToContainerStats(ctr)
	defer daemon.unsubscribeToContainerStats(ctr, updates)

	noStreamFirstFrame := !config.OneShot
	for {
		select {
		case v, ok := <-updates:
			if !ok {
				return nil
			}

			var statsJSON interface{}
			statsJSONPost120 := getStatJSON(v)
			if versions.LessThan(apiVersion, "1.21") {
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
		case <-ctx.Done():
			return nil
		}
	}
}

func (daemon *Daemon) subscribeToContainerStats(c *container.Container) chan interface{} {
	return daemon.statsCollector.Collect(c)
}

func (daemon *Daemon) unsubscribeToContainerStats(c *container.Container, ch chan interface{}) {
	daemon.statsCollector.Unsubscribe(c, ch)
}

// GetContainerStats collects all the stats published by a container
func (daemon *Daemon) GetContainerStats(ctx context.Context, container *container.Container) (*types.StatsJSON, error) {
	stats, err := daemon.stats(ctx, container)
	if err != nil {
		return nil, err
	}

	// We already have the network stats on Windows directly from HCS.
	if !container.Config.NetworkDisabled && runtime.GOOS != "windows" {
		if stats.Networks, err = daemon.getNetworkStats(ctx, container); err != nil {
			return nil, err
		}
	}

	return stats, nil
}
