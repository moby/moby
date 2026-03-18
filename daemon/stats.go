package daemon

import (
	"context"
	"encoding/json"
	"runtime"
	"time"

	cerrdefs "github.com/containerd/errdefs"
	"github.com/containerd/log"
	containertypes "github.com/moby/moby/api/types/container"
	"github.com/moby/moby/v2/daemon/container"
	"github.com/moby/moby/v2/daemon/server/backend"
)

// ContainerStats writes information about the container to the stream
// given in the config object.
func (daemon *Daemon) ContainerStats(ctx context.Context, prefixOrName string, config *backend.ContainerStatsConfig) error {
	ctr, err := daemon.GetContainer(prefixOrName)
	if err != nil {
		return err
	}

	// We take two samples for the first non-streaming result if OneShot
	// is disabled (OneShot=false), to populate the PreRead and PreCPUStats
	// fields.
	var needPrevSample bool
	if !config.Stream {
		if !ctr.State.IsRunning() || ctr.State.IsRestarting() {
			// The container is either not running or restarting, return an empty stats.
			return json.NewEncoder(config.OutStream()).Encode(&containertypes.StatsResponse{
				ID:     ctr.ID,
				Name:   ctr.Name,
				OSType: runtime.GOOS,
			})
		}
		if config.OneShot {
			// In OneShot-mode, we only collect a single sample, return immediately.
			//
			// In streaming mode, OneShot has no effect, as we never populate
			// the Pre* fields for the first result.
			stats, err := daemon.GetContainerStats(ctr)
			if err != nil {
				return err
			}
			return json.NewEncoder(config.OutStream()).Encode(stats)
		}

		// Non-streaming and not OneShot; need two samples to populate Pre*.
		needPrevSample = true
	}

	updates, cancel := daemon.subscribeToContainerStats(ctr)
	defer cancel()

	var (
		previousRead     time.Time               // Previous Read time to populate the PreRead field.
		previousCPUStats containertypes.CPUStats // Previous CPUStats to populate the PreCPUStats field.
	)

	enc := json.NewEncoder(config.OutStream())
	enc.SetEscapeHTML(false)
	for {
		select {
		case v, ok := <-updates:
			if !ok {
				return nil
			}

			statsJSON, ok := v.(containertypes.StatsResponse)
			if !ok {
				return cerrdefs.ErrInternal.WithMessage("stats: unexpected value type")
			}

			if needPrevSample {
				// Take first sample only to populate Pre* for the next one.
				previousRead = statsJSON.Read
				previousCPUStats = statsJSON.CPUStats
				needPrevSample = false
				continue
			}

			statsJSON.PreRead = previousRead
			statsJSON.PreCPUStats = previousCPUStats
			if err := enc.Encode(&statsJSON); err != nil {
				return err
			}

			if !config.Stream {
				return nil
			}

			previousRead = statsJSON.Read
			previousCPUStats = statsJSON.CPUStats
		case <-ctx.Done():
			return nil
		}
	}
}

// subscribeToContainerStats starts collecting stats for the given container.
// It returns a channel containing [containertypes.StatsResponse] records,
// and a cancel function to unsubscribe and stop collecting stats.
func (daemon *Daemon) subscribeToContainerStats(c *container.Container) (updates chan any, cancel func()) {
	ch := daemon.statsCollector.Collect(c)
	cancel = func() {
		daemon.statsCollector.Unsubscribe(c, ch)
	}
	return ch, cancel
}

// GetContainerStats collects all the stats published by a container
func (daemon *Daemon) GetContainerStats(ctr *container.Container) (*containertypes.StatsResponse, error) {
	stats, err := daemon.stats(ctr)
	if err != nil {
		goto done
	}

	// Sample system CPU usage close to container usage to avoid
	// noise in metric calculations.
	// FIXME: move to containerd on Linux (not Windows)
	stats.CPUStats.SystemUsage, stats.CPUStats.OnlineCPUs, err = getSystemCPUUsage()
	if err != nil {
		goto done
	}

	// We already have the network stats on Windows directly from HCS.
	if !ctr.Config.NetworkDisabled && runtime.GOOS != "windows" {
		stats.Networks, err = daemon.getNetworkStats(ctr)
	}

done:
	if err != nil {
		if cerrdefs.IsNotFound(err) || cerrdefs.IsConflict(err) {
			// return empty stats containing only name and ID if not running or not found
			return &containertypes.StatsResponse{
				ID:     ctr.ID,
				Name:   ctr.Name,
				OSType: runtime.GOOS,
			}, nil
		}
		log.G(context.TODO()).Errorf("collecting stats for container %s: %v", ctr.Name, err)
		return nil, err
	}
	return stats, nil
}
