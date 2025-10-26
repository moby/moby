package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"runtime"
	"time"

	cerrdefs "github.com/containerd/errdefs"
	"github.com/containerd/log"
	containertypes "github.com/moby/moby/api/types/container"
	"github.com/moby/moby/v2/daemon/container"
	"github.com/moby/moby/v2/daemon/server/backend"
	"github.com/moby/moby/v2/errdefs"
)

// ContainerStats writes information about the container to the stream
// given in the config object.
func (daemon *Daemon) ContainerStats(ctx context.Context, prefixOrName string, config *backend.ContainerStatsConfig) error {
	ctr, err := daemon.GetContainer(prefixOrName)
	if err != nil {
		return err
	}

	if config.Stream && config.OneShot {
		return errdefs.InvalidParameter(errors.New("cannot have stream=true and one-shot=true"))
	}

	// If the container is either not running or restarting and requires no stream, return an empty stats.
	if !config.Stream && (!ctr.State.IsRunning() || ctr.State.IsRestarting()) {
		return json.NewEncoder(config.OutStream()).Encode(&containertypes.StatsResponse{
			Name: ctr.Name,
			ID:   ctr.ID,
		})
	}

	// Get container stats directly if OneShot is set
	if config.OneShot {
		stats, err := daemon.GetContainerStats(ctr)
		if err != nil {
			return err
		}
		return json.NewEncoder(config.OutStream()).Encode(stats)
	}

	updates, cancel := daemon.subscribeToContainerStats(ctr)
	defer cancel()

	var (
		previousRead     time.Time               // Previous Read time to populate the PreRead field.
		previousCPUStats containertypes.CPUStats // Previous CPUStats to populate the PreCPUStats field.

		// For the first non-streaming result (when OneShot=false), we
		// collect two samples for the first result to populate the PreRead
		// and PreCPUStats fields.
		needPrevSample = !config.Stream && !config.OneShot
	)

	enc := json.NewEncoder(config.OutStream())
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
				Name: ctr.Name,
				ID:   ctr.ID,
			}, nil
		}
		log.G(context.TODO()).Errorf("collecting stats for container %s: %v", ctr.Name, err)
		return nil, err
	}
	return stats, nil
}
