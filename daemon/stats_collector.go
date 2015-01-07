package daemon

import (
	"sync"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/docker/docker/daemon/execdriver"
)

func newStatsCollector(interval time.Duration) *statsCollector {
	s := &statsCollector{
		interval:   interval,
		containers: make(map[string]*statsCollectorData),
	}
	s.start()
	return s
}

type statsCollectorData struct {
	c         *Container
	lastStats *execdriver.ResourceStats
	subs      []chan *execdriver.ResourceStats
}

// statsCollector manages and provides container resource stats
type statsCollector struct {
	m          sync.Mutex
	interval   time.Duration
	containers map[string]*statsCollectorData
}

func (s *statsCollector) collect(c *Container) <-chan *execdriver.ResourceStats {
	s.m.Lock()
	ch := make(chan *execdriver.ResourceStats, 1024)
	s.containers[c.ID] = &statsCollectorData{
		c: c,
		subs: []chan *execdriver.ResourceStats{
			ch,
		},
	}
	s.m.Unlock()
	return ch
}

func (s *statsCollector) stopCollection(c *Container) {
	s.m.Lock()
	delete(s.containers, c.ID)
	s.m.Unlock()
}

func (s *statsCollector) start() {
	go func() {
		for _ = range time.Tick(s.interval) {
			log.Debugf("starting collection of container stats")
			s.m.Lock()
			for id, d := range s.containers {
				stats, err := d.c.Stats()
				if err != nil {
					// TODO: @crosbymichael evict container depending on error
					log.Errorf("collecting stats for %s: %v", id, err)
					continue
				}
				for _, sub := range s.containers[id].subs {
					sub <- stats
				}
			}
			s.m.Unlock()
		}
	}()
}
