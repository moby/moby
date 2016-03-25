// +build !windows,!solaris

package daemon

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/docker/docker/container"
	"github.com/docker/docker/pkg/pubsub"
	sysinfo "github.com/docker/docker/pkg/system"
	"github.com/docker/engine-api/types"
	"github.com/opencontainers/runc/libcontainer/system"
)

type statsSupervisor interface {
	// GetContainerStats collects all the stats related to a container
	GetContainerStats(container *container.Container) (*types.StatsJSON, error)
}

// newStatsCollector returns a new statsCollector that collections
// network and cgroup stats for a registered container at the specified
// interval.  The collector allows non-running containers to be added
// and will start processing stats when they are started.
func (daemon *Daemon) newStatsCollector(interval time.Duration) *statsCollector {
	s := &statsCollector{
		interval:            interval,
		supervisor:          daemon,
		publishers:          make(map[*container.Container]*pubsub.Publisher),
		clockTicksPerSecond: uint64(system.GetClockTicks()),
		bufReader:           bufio.NewReaderSize(nil, 128),
	}
	meminfo, err := sysinfo.ReadMemInfo()
	if err == nil && meminfo.MemTotal > 0 {
		s.machineMemory = uint64(meminfo.MemTotal)
	}

	go s.run()
	return s
}

// statsCollector manages and provides container resource stats
type statsCollector struct {
	m                   sync.Mutex
	supervisor          statsSupervisor
	interval            time.Duration
	clockTicksPerSecond uint64
	publishers          map[*container.Container]*pubsub.Publisher
	bufReader           *bufio.Reader
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

func (s *statsCollector) run() {
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
			// FIXME: move to containerd
			stats.CPUStats.SystemUsage = systemUsage

			pair.publisher.Publish(*stats)
		}
	}
}

const nanoSecondsPerSecond = 1e9

// getSystemCPUUsage returns the host system's cpu usage in
// nanoseconds. An error is returned if the format of the underlying
// file does not match.
//
// Uses /proc/stat defined by POSIX. Looks for the cpu
// statistics line and then sums up the first seven fields
// provided. See `man 5 proc` for details on specific field
// information.
func (s *statsCollector) getSystemCPUUsage() (uint64, error) {
	var line string
	f, err := os.Open("/proc/stat")
	if err != nil {
		return 0, err
	}
	defer func() {
		s.bufReader.Reset(nil)
		f.Close()
	}()
	s.bufReader.Reset(f)
	err = nil
	for err == nil {
		line, err = s.bufReader.ReadString('\n')
		if err != nil {
			break
		}
		parts := strings.Fields(line)
		switch parts[0] {
		case "cpu":
			if len(parts) < 8 {
				return 0, fmt.Errorf("invalid number of cpu fields")
			}
			var totalClockTicks uint64
			for _, i := range parts[1:8] {
				v, err := strconv.ParseUint(i, 10, 64)
				if err != nil {
					return 0, fmt.Errorf("Unable to convert value %s to int: %s", i, err)
				}
				totalClockTicks += v
			}
			return (totalClockTicks * nanoSecondsPerSecond) /
				s.clockTicksPerSecond, nil
		}
	}
	return 0, fmt.Errorf("invalid stat format. Error trying to parse the '/proc/stat' file")
}
