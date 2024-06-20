package stats // import "github.com/docker/docker/daemon/stats"

import (
	"sync"
	"time"

	containertypes "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/container"
	"github.com/moby/pubsub"
)

// Collector manages and provides container resource stats
type Collector struct {
	m          sync.Mutex
	cond       *sync.Cond
	supervisor supervisor
	interval   time.Duration
	publishers map[*container.Container]*pubsub.Publisher
}

// NewCollector creates a stats collector that will poll the supervisor with the specified interval
func NewCollector(supervisor supervisor, interval time.Duration) *Collector {
	s := &Collector{
		interval:   interval,
		supervisor: supervisor,
		publishers: make(map[*container.Container]*pubsub.Publisher),
	}
	s.cond = sync.NewCond(&s.m)
	return s
}

type supervisor interface {
	// GetContainerStats collects all the stats related to a container
	GetContainerStats(container *container.Container) (*containertypes.StatsResponse, error)
}

// Collect registers the container with the collector and adds it to
// the event loop for collection on the specified interval returning
// a channel for the subscriber to receive on.
func (s *Collector) Collect(c *container.Container) chan interface{} {
	s.cond.L.Lock()
	defer s.cond.L.Unlock()

	publisher, exists := s.publishers[c]
	if !exists {
		publisher = pubsub.NewPublisher(100*time.Millisecond, 1024)
		s.publishers[c] = publisher
	}

	s.cond.Broadcast()
	return publisher.Subscribe()
}

// StopCollection closes the channels for all subscribers and removes
// the container from metrics collection.
func (s *Collector) StopCollection(c *container.Container) {
	s.m.Lock()
	if publisher, exists := s.publishers[c]; exists {
		publisher.Close()
		delete(s.publishers, c)
	}
	s.m.Unlock()
}

// Unsubscribe removes a specific subscriber from receiving updates for a container's stats.
func (s *Collector) Unsubscribe(c *container.Container, ch chan interface{}) {
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

// Run starts the collectors and will indefinitely collect stats from the supervisor
func (s *Collector) Run() {
	type publishersPair struct {
		container *container.Container
		publisher *pubsub.Publisher
	}
	// we cannot determine the capacity here.
	// it will grow enough in first iteration
	var pairs []publishersPair

	for {
		s.cond.L.Lock()
		for len(s.publishers) == 0 {
			s.cond.Wait()
		}

		// it does not make sense in the first iteration,
		// but saves allocations in further iterations
		pairs = pairs[:0]

		for container, publisher := range s.publishers {
			// copy pointers here to release the lock ASAP
			pairs = append(pairs, publishersPair{container, publisher})
		}

		s.cond.L.Unlock()

		for _, pair := range pairs {
			stats, err := s.supervisor.GetContainerStats(pair.container)
			if err != nil {
				stats = &containertypes.StatsResponse{
					Name: pair.container.Name,
					ID:   pair.container.ID,
				}
			}
			pair.publisher.Publish(*stats)
		}

		time.Sleep(s.interval)
	}
}
