package container

import "github.com/docker/docker/api/types"

// Health holds the current container health-check state
type Health types.Health

// String returns a human-readable description of the health-check state
func (s *Health) String() string {
	switch s.Status {
	case types.Starting:
		return "health: starting"
	default: // Healthy and Unhealthy are clear on their own
		return s.Status
	}
}

// OpenHealthMonitorChannel creates and returns a new monitor channel. If there already is one,
// it returns nil.
func (container *Container) OpenHealthMonitorChannel() chan struct{} {
	return container.runState.openHealthMonitor()
}

// CloseHealthMonitorChannel closes any existing monitor channel.
func (container *Container) CloseHealthMonitorChannel() {
	container.runState.closeHealthMonitor()
}
