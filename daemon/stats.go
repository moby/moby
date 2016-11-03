package daemon

import (
	"encoding/json"
	"errors"
	"fmt"
	"runtime"
	"strings"

	"golang.org/x/net/context"

	"github.com/Sirupsen/logrus"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/backend"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/versions"
	"github.com/docker/docker/api/types/versions/v1p20"
	"github.com/docker/docker/container"
	"github.com/docker/docker/pkg/ioutils"
)

// ContainerStats writes information about the container to the stream
// given in the config object.
func (daemon *Daemon) ContainerStats(ctx context.Context, prefixOrName string, config *backend.ContainerStatsConfig) error {
	if runtime.GOOS == "solaris" {
		return fmt.Errorf("%+v does not support stats", runtime.GOOS)
	}
	// Engine API version (used for backwards compatibility)
	apiVersion := config.Version

	container, err := daemon.GetContainer(prefixOrName)
	if err != nil {
		return err
	}

	// If the container is either not running or restarting and requires no stream, return an empty stats.
	if (!container.IsRunning() || container.IsRestarting()) && !config.Stream {
		return json.NewEncoder(config.OutStream).Encode(&types.Stats{})
	}

	outStream := config.OutStream
	if config.Stream {
		wf := ioutils.NewWriteFlusher(outStream)
		defer wf.Close()
		wf.Flush()
		outStream = wf
	}

	filters := filters.NewArgs()
	filters.Add("id", container.ID)
	statsChan, errChan := daemon.fetchAndFilterAllStats(ctx, config.Stream, filters)

	enc := json.NewEncoder(outStream)
	for {
		select {
		case allStats, ok := <-statsChan:
			if !ok {
				return nil
			}

			statsJSON, ok := allStats[container.ID]
			if !ok || statsJSON == nil {
				return fmt.Errorf("can't get stats data for %q", container.ID[:12])
			}

			// this is for backward compatibity with API Pre 1.21
			json, err := daemon.transformNetworkStats(apiVersion, statsJSON)
			if err != nil {
				return err
			}

			// we have filtered the results by "id", now we only send the
			// StatsJSON instead of map[string]StatsJSONComplex for backward compatibility
			if err := enc.Encode(json); err != nil {
				return err
			}
			if !config.Stream {
				return nil
			}
		case err := <-errChan:
			if err != nil {
				return err
			}
			return nil
		}
	}
}

func (daemon *Daemon) subscribeToContainerStats(c *container.Container) chan interface{} {
	return daemon.statsCollector.collect(c)
}

func (daemon *Daemon) unsubscribeToContainerStats(c *container.Container, ch chan interface{}) {
	daemon.statsCollector.unsubscribe(c, ch)
}

// GetContainerStats collects all the stats published by a container
func (daemon *Daemon) GetContainerStats(container *container.Container) (*types.StatsJSON, error) {
	stats, err := daemon.stats(container)
	if err != nil {
		return nil, err
	}

	// We already have the network stats on Windows directly from HCS.
	if !container.Config.NetworkDisabled && runtime.GOOS != "windows" {
		if stats.Networks, err = daemon.getNetworkStats(container); err != nil {
			return nil, err
		}
	}

	return stats, nil
}

// ContainerStatsAll writes information about containers to the stream
// given in the config object.
func (daemon *Daemon) ContainerStatsAll(ctx context.Context, config *backend.ContainerStatsAllConfig) error {
	outStream := config.OutStream
	if config.Stream {
		wf := ioutils.NewWriteFlusher(outStream)
		defer wf.Close()
		wf.Flush()
		outStream = wf
	}

	enc := json.NewEncoder(outStream)

	statsChan, errChan := daemon.fetchAndFilterAllStats(ctx, config.Stream, config.Filters)

	for {
		select {
		case allStats, ok := <-statsChan:
			if !ok {
				return nil
			}
			if err := enc.Encode(allStats); err != nil {
				return err
			}
			if !config.Stream {
				return nil
			}
		case err := <-errChan:
			if err != nil {
				return err
			}
		}
	}
}

func (daemon *Daemon) fetchAndFilterAllStats(ctx context.Context, stream bool, filters filters.Args) (chan map[string]*types.StatsJSON, chan error) {
	statsChan := make(chan (map[string]*types.StatsJSON), 16)
	errChan := make(chan error)

	go func() {
		updates := daemon.subscribeToContainerStatsAll()
		defer daemon.unsubscribeToContainerStatsAll(updates)

		defer close(statsChan)
		for {
			select {
			case v, ok := <-updates:
				if !ok {
					errChan <- nil
					return
				}

				origStatsJSON, ok := v.(map[string]*types.StatsJSON)
				if !ok {
					// malformed data!
					logrus.Errorf("receive malformed stats data")
					continue
				}

				// copy map to avoid concurrent write
				statsJSON := make(map[string]*types.StatsJSON)
				for k, v := range origStatsJSON {
					statsJSON[k] = v
				}

				var preCPUNotExits bool
				if !stream {
					// prime the cpu stats so they aren't 0 in the final output
					for _, ss := range statsJSON {
						if ss.PreCPUStats == nil {
							preCPUNotExits = true
							break
						}
					}

					// if there's stats without preCPUNotExits
					if preCPUNotExits {
						continue
					}
				}

				// filter container
				filterdCtrs, err := daemon.reduceStatsContainers(filters)
				if err != nil {
					logrus.Errorf("can't filter containers: %v", err)
					continue
				}

				survivedContainers := make(map[string]*types.Container)
				for _, ctr := range filterdCtrs {
					survivedContainers[ctr.ID] = ctr
				}

				// if container didn't survive from the filter, remove it!
				var toDelete []string
				for id := range statsJSON {
					if _, ok := survivedContainers[id]; !ok {
						toDelete = append(toDelete, id)
					} else {
						delete(survivedContainers, id)
					}
				}

				// iterate the toDelete list, delete statsJSON with related id
				for _, id := range toDelete {
					delete(statsJSON, id)
				}

				// if we didn't get stats data for one survived container,
				// this means that the container isn't in running state,
				// we need to add an empty item for it in statsJSON
				for id, ctr := range survivedContainers {
					statsJSON[id] = &types.StatsJSON{
						ID:   ctr.ID,
						Name: strings.Join(ctr.Names, ","),
					}
				}
				statsChan <- statsJSON

				if !stream {
					errChan <- nil
					return
				}
			case <-ctx.Done():
				errChan <- nil
				return
			}
		}
	}()
	return statsChan, errChan
}

// transformNetworkStats is basically used for backward compatibility
// it will transform new stats format into old format for old API
func (daemon *Daemon) transformNetworkStats(version string, stats *types.StatsJSON) (interface{}, error) {
	if versions.LessThan(version, "1.21") && runtime.GOOS == "windows" {
		return nil, errors.New("API versions pre v1.21 do not support stats on Windows")
	}

	var statsJSON interface{}
	if versions.LessThan(version, "1.21") {
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
		for _, v := range stats.Networks {
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
			Stats: stats.Stats,
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
		statsJSON = stats
	}

	return statsJSON, nil
}

func (daemon *Daemon) subscribeToContainerStatsAll() chan interface{} {
	return daemon.statsCollector.collectAll()
}

func (daemon *Daemon) unsubscribeToContainerStatsAll(ch chan interface{}) {
	daemon.statsCollector.unsubscribeAll(ch)
}

// GetContainerStatsAllRunning collects stats of all the running containers
func (daemon *Daemon) GetContainerStatsAllRunning() map[string]*types.StatsJSON {
	allStats := make(map[string]*types.StatsJSON)
	containers := daemon.List()
	for _, cnt := range containers {
		if !cnt.IsRunning() {
			continue
		}
		stats, err := daemon.GetContainerStats(cnt)
		if err != nil {
			if _, ok := err.(errNotRunning); !ok {
				logrus.Errorf("collecting stats for %s: %v", cnt.ID, err)
			}
			continue
		}

		stats.ID = cnt.ID
		stats.Name = cnt.Name
		allStats[cnt.ID] = stats
	}
	return allStats
}

func (daemon *Daemon) reduceStatsContainers(filter filters.Args) ([]*types.Container, error) {
	config := &types.ContainerListOptions{
		All:     true,
		Size:    true,
		Filters: filter,
	}

	return daemon.reduceContainers(config, daemon.transformContainer)
}

// transformContainer generates the container type expected by the docker ps command.
func (daemon *Daemon) transformStatsContainer(container *container.Container, ctx *listContext) (*types.Container, error) {
	// For stats data, we only care about its ID for next step filtering
	newC := &types.Container{
		ID: container.ID,
	}
	return newC, nil
}
