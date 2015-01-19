package daemon

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/docker/docker/daemon/execdriver"
	"github.com/docker/libcontainer/system"
)

// newStatsCollector returns a new statsCollector that collections
// network and cgroup stats for a registered container at the specified
// interval.  The collector allows non-running containers to be added
// and will start processing stats when they are started.
func newStatsCollector(interval time.Duration) *statsCollector {
	s := &statsCollector{
		interval:   interval,
		containers: make(map[string]*statsData),
		clockTicks: uint64(system.GetClockTicks()),
	}
	s.start()
	return s
}

type statsData struct {
	c         *Container
	lastStats *execdriver.ResourceStats
	subs      []chan *execdriver.ResourceStats
}

// statsCollector manages and provides container resource stats
type statsCollector struct {
	m          sync.Mutex
	interval   time.Duration
	clockTicks uint64
	containers map[string]*statsData
}

// collect registers the container with the collector and adds it to
// the event loop for collection on the specified interval returning
// a channel for the subscriber to receive on.
func (s *statsCollector) collect(c *Container) chan *execdriver.ResourceStats {
	s.m.Lock()
	defer s.m.Unlock()
	ch := make(chan *execdriver.ResourceStats, 1024)
	if _, exists := s.containers[c.ID]; exists {
		s.containers[c.ID].subs = append(s.containers[c.ID].subs, ch)
		return ch
	}
	s.containers[c.ID] = &statsData{
		c: c,
		subs: []chan *execdriver.ResourceStats{
			ch,
		},
	}
	return ch
}

// stopCollection closes the channels for all subscribers and removes
// the container from metrics collection.
func (s *statsCollector) stopCollection(c *Container) {
	s.m.Lock()
	defer s.m.Unlock()
	d := s.containers[c.ID]
	if d == nil {
		return
	}
	for _, sub := range d.subs {
		close(sub)
	}
	delete(s.containers, c.ID)
}

// unsubscribe removes a specific subscriber from receiving updates for a
// container's stats.
func (s *statsCollector) unsubscribe(c *Container, ch chan *execdriver.ResourceStats) {
	s.m.Lock()
	cd := s.containers[c.ID]
	for i, sub := range cd.subs {
		if ch == sub {
			cd.subs = append(cd.subs[:i], cd.subs[i+1:]...)
			close(ch)
		}
	}
	// if there are no more subscribers then remove the entire container
	// from collection.
	if len(cd.subs) == 0 {
		delete(s.containers, c.ID)
	}
	s.m.Unlock()
}

func (s *statsCollector) start() {
	go func() {
		for _ = range time.Tick(s.interval) {
			s.m.Lock()
			for id, d := range s.containers {
				systemUsage, err := s.getSystemCpuUsage()
				if err != nil {
					log.Errorf("collecting system cpu usage for %s: %v", id, err)
					continue
				}
				stats, err := d.c.Stats()
				if err != nil {
					if err == execdriver.ErrNotRunning {
						continue
					}
					// if the error is not because the container is currently running then
					// evict the container from the collector and close the channel for
					// any subscribers currently waiting on changes.
					log.Errorf("collecting stats for %s: %v", id, err)
					for _, sub := range s.containers[id].subs {
						close(sub)
					}
					delete(s.containers, id)
					continue
				}
				stats.SystemUsage = systemUsage
				for _, sub := range s.containers[id].subs {
					sub <- stats
				}
			}
			s.m.Unlock()
		}
	}()
}

const nanoSeconds = 1e9

// getSystemdCpuUSage returns the host system's cpu usage in nanoseconds
// for the system to match the cgroup readings are returned in the same format.
func (s *statsCollector) getSystemCpuUsage() (uint64, error) {
	f, err := os.Open("/proc/stat")
	if err != nil {
		return 0, err
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		parts := strings.Fields(sc.Text())
		switch parts[0] {
		case "cpu":
			if len(parts) < 8 {
				return 0, fmt.Errorf("invalid number of cpu fields")
			}
			var sum uint64
			for _, i := range parts[1:8] {
				v, err := strconv.ParseUint(i, 10, 64)
				if err != nil {
					return 0, fmt.Errorf("Unable to convert value %s to int: %s", i, err)
				}
				sum += v
			}
			return (sum * nanoSeconds) / s.clockTicks, nil
		}
	}
	return 0, fmt.Errorf("invalid stat format")
}
