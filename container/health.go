package container

import (
	"github.com/Sirupsen/logrus"
	"github.com/docker/engine-api/types"
)

// Health holds the current container health-check state
type Health struct {
	types.Health
	stop chan struct{} // Write struct{} to stop the monitor
}

// String returns a human-readable description of the health-check state
func (s *Health) String() string {
	if s.stop == nil {
		return "no healthcheck"
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
		logrus.Debugf("OpenMonitorChannel")
		s.stop = make(chan struct{})
		return s.stop
	}
	return nil
}

// CloseMonitorChannel closes any existing monitor channel.
func (s *Health) CloseMonitorChannel() {
	if s.stop != nil {
		logrus.Debugf("CloseMonitorChannel: waiting for probe to stop")
		// This channel does not buffer. Once the write succeeds, the monitor
		// has read the stop request and will not make any further updates
		// to c.State.Health.
		s.stop <- struct{}{}
		s.stop = nil
		logrus.Debugf("CloseMonitorChannel done")
	}
}
