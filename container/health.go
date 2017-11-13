package container

import (
	"sync"

	"github.com/docker/docker/api/types"
	"github.com/sirupsen/logrus"
)

// Health holds the current container health-check state
type Health struct {
	types.Health
	stop chan struct{} // Write struct{} to stop the monitor
	mu   sync.Mutex
}

// String returns a human-readable description of the health-check state
func (s *Health) String() string {
	// This happens when the monitor has yet to be setup.
	if s.Status == "" {
		return types.Unhealthy
	}

	switch s.Status {
	case types.Starting:
		return "health: starting"
	default: // Healthy and Unhealthy are clear on their own
		return s.Status
	}
}

// OpenMonitorChannel creates and returns a new monitor channel. If there
// already is one, it returns nil.
func (s *Health) OpenMonitorChannel() chan struct{} {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.stop == nil {
		logrus.Debug("OpenMonitorChannel")
		s.stop = make(chan struct{})
		return s.stop
	}
	return nil
}

// CloseMonitorChannel closes any existing monitor channel.
func (s *Health) CloseMonitorChannel() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.stop != nil {
		logrus.Debug("CloseMonitorChannel: waiting for probe to stop")
		close(s.stop)
		s.stop = nil
		// unhealthy when the monitor has stopped for compatibility reasons
		s.Status = types.Unhealthy
		logrus.Debug("CloseMonitorChannel done")
	}
}
