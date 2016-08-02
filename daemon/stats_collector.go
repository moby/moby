// +build !solaris

package daemon

import (
	"bufio"
	"sync"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/container"
	"github.com/docker/docker/pkg/pubsub"
)

type statsSupervisor interface {
	// GetContainerStats collects all the stats related to a container
	GetContainerStats(container *container.Container) (*types.StatsJSON, error)
	// GetContainerStatsAllRunning collects stats of all the running containers
	GetContainerStatsAllRunning() map[string]*types.StatsJSON
}

// newStatsCollector returns a new statsCollector that collections
// stats for a registered container at the specified interval.
// The collector allows non-running containers to be added
// and will start processing stats when they are started.
func (daemon *Daemon) newStatsCollector(interval time.Duration) *statsCollector {
	s := &statsCollector{
		interval:   interval,
		supervisor: daemon,
		publishers: make(map[*container.Container]*pubsub.Publisher),
		bufReader:  bufio.NewReaderSize(nil, 128),
	}
	platformNewStatsCollector(s)
	go s.runCollectorForSome()
	go s.runCollectorForAllRunning()
	return s
}

// statsCollector manages and provides container resource stats
type statsCollector struct {
	m            sync.Mutex
	supervisor   statsSupervisor
	interval     time.Duration
	publishers   map[*container.Container]*pubsub.Publisher
	publisherAll *pubsub.Publisher
	bufReader    *bufio.Reader

	// The following fields are not set on Windows currently.
	clockTicksPerSecond uint64
	machineMemory       uint64
}

// collect registers the container with the collector and adds it to
// the event loop for collection on the specified interval returning
// a channel for the subscriber to receive on.
func (s *statsCollector) collect(c *container.Container) chan interface{} {
	s.m.Lock()
	defer s.m.Unlock()
	publisher, exists := s.publishers[c]
	if !exists {
		publisher = pubsub.NewPublisher(100*time.Millisecond, 1024)
		s.publishers[c] = publisher
	}
	return publisher.Subscribe()
}

// stopCollection closes the channels for all subscribers and removes
// the container from metrics collection.
func (s *statsCollector) stopCollection(c *container.Container) {
	s.m.Lock()
	if publisher, exists := s.publishers[c]; exists {
		publisher.Close()
		delete(s.publishers, c)
	}
	s.m.Unlock()
}

// unsubscribe removes a specific subscriber from receiving updates for a container's stats.
func (s *statsCollector) unsubscribe(c *container.Container, ch chan interface{}) {
	s.m.Lock()
	publisher := s.publishers[c]
	if publisher != nil {
		publisher.Evict(ch)
		if publisher.Len() == 0 {
			delete(s.publishers, c)
		}
	}
	s.m.Unlock()
}

// collectAll start the event loop for collecting on all containers
// it will return a channel for subscriber to receive on.
func (s *statsCollector) collectAll() chan interface{} {
	s.m.Lock()
	defer s.m.Unlock()
	if s.publisherAll == nil {
		s.publisherAll = pubsub.NewPublisher(100*time.Millisecond, 1024)
	}
	return s.publisherAll.Subscribe()
}

// stopCollectionAll closes the channels for all subscribers and removes
// the container from metrics collection.
func (s *statsCollector) stopCollectionAll() {
	s.m.Lock()
	if s.publisherAll != nil {
		s.publisherAll.Close()
		s.publisherAll = nil
	}
	s.m.Unlock()
}

// unsubscribeAll removes a specific subscriber from receiving updates for all container's stats.
func (s *statsCollector) unsubscribeAll(ch chan interface{}) {
	s.m.Lock()
	if s.publisherAll != nil {
		s.publisherAll.Evict(ch)
		if s.publisherAll.Len() == 0 {
			s.publisherAll = nil
		}
	}
	s.m.Unlock()
}

func (s *statsCollector) runCollectorForSome() {
	type publishersPair struct {
		container *container.Container
		publisher *pubsub.Publisher
	}
	// we cannot determine the capacity here.
	// it will grow enough in first iteration
	var pairs []publishersPair

	for range time.Tick(s.interval) {
		// it does not make sense in the first iteration,
		// but saves allocations in further iterations
		pairs = pairs[:0]

		s.m.Lock()
		for container, publisher := range s.publishers {
			// copy pointers here to release the lock ASAP
			pairs = append(pairs, publishersPair{container, publisher})
		}
		s.m.Unlock()
		if len(pairs) == 0 {
			continue
		}

		systemUsage, err := s.getSystemCPUUsage()
		if err != nil {
			logrus.Errorf("collecting system cpu usage: %v", err)
			continue
		}

		for _, pair := range pairs {
			stats, err := s.supervisor.GetContainerStats(pair.container)
			if err != nil {
				if _, ok := err.(errNotRunning); !ok {
					logrus.Errorf("collecting stats for %s: %v", pair.container.ID, err)
				}
				continue
			}
			// FIXME: move to containerd on Linux (not Windows)
			stats.CPUStats.SystemUsage = systemUsage

			pair.publisher.Publish(*stats)
		}
	}
}

func (s *statsCollector) runCollectorForAllRunning() {
	// preCPUStats save CPUStats collected from last time
	// if fails to collect during s.interval, clear it all
	// we don't need any outdated data
	preCPUStats := make(map[string]*types.CPUStats)
	preRead := make(map[string]time.Time)
	for range time.Tick(s.interval) {
		if s.publisherAll == nil {
			// no subscriber, preCPUStats is out of date
			// clear it all
			preCPUStats = nil
			preRead = nil
			continue
		}

		systemUsage, err := s.getSystemCPUUsage()
		if err != nil {
			logrus.Errorf("collecting system cpu usage: %v", err)
			preCPUStats = nil
			preRead = nil
			continue
		}

		if preCPUStats == nil {
			preCPUStats = make(map[string]*types.CPUStats)
			preRead = make(map[string]time.Time)
		}

		pubResults := s.supervisor.GetContainerStatsAllRunning()

		// remove outdated data from preCPUStats and preRead
		for id := range preCPUStats {
			if _, ok := pubResults[id]; !ok {
				// if pubResults doesn't contains container with "id",
				// remove it from preCPUStats. This can happen when container
				// stops running
				delete(preCPUStats, id)
			}
		}
		for id := range preRead {
			if _, ok := pubResults[id]; !ok {
				delete(preRead, id)
			}
		}

		for id, stats := range pubResults {
			// FIXME: move to containerd
			stats.CPUStats.SystemUsage = systemUsage
			if _, ok := preCPUStats[id]; ok {
				stats.PreCPUStats = preCPUStats[id]
			}
			preCPUStats[id] = &stats.CPUStats
			if _, ok := preRead[id]; ok {
				stats.PreRead = preRead[id]
			}
			preRead[id] = stats.Read
		}

		// publish results no matter it's empty or not
		s.m.Lock()
		if s.publisherAll != nil {
			s.publisherAll.Publish(pubResults)
		}
		s.m.Unlock()
	}
}
