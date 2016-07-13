package container

import "fmt"

var (
	// ErrImageRequired returned if a task is missing the image definition.
	ErrImageRequired = fmt.Errorf("dockerexec: image required")

	// ErrContainerDestroyed returned when a container is prematurely destroyed
	// during a wait call.
	ErrContainerDestroyed = fmt.Errorf("dockerexec: container destroyed")

	// ErrContainerUnhealthy returned if controller detects the health check failure
	ErrContainerUnhealthy = fmt.Errorf("dockerexec: unhealthy container")
)
