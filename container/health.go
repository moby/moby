package container

import (
	"github.com/Sirupsen/logrus"
	"github.com/docker/docker/api/types"
)

// Health holds the current container health-check state
type Health struct {
	types.Health
	stop chan struct{} // Write struct{} to stop the monitor
}

// String returns a human-readable description of the health-check state
func (s *Health) String() string {
	// This happens when the container is being shutdown and the monitor has stopped
	// or the monitor has yet to be setup.
	if s.stop == nil {
		return types.Unhealthy
	}

	switch s.Status {
	case types.Starting:
		return "health: starting"
	default: // Healthy and Unhealthy are clear on their own
		return s.Status
	}
}

// OpenMonitorChannel creates and returns a new monitor channel. If there already is one,
// it returns nil.
func (s *Health) OpenMonitorChannel() chan struct{} {
	if s.stop == nil {
		logrus.Debug("OpenMonitorChannel")
		s.stop = make(chan struct{})
		return s.stop
	}
	return nil
}

// CloseMonitorChannel closes any existing monitor channel.
func (s *Health) CloseMonitorChannel() {
	if s.stop != nil {
		logrus.Debug("CloseMonitorChannel: waiting for probe to stop")
		close(s.stop)
		s.stop = nil
		logrus.Debug("CloseMonitorChannel done")
	}
}
