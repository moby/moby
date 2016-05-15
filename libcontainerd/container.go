package libcontainerd

import (
	"fmt"
	"time"

	"github.com/docker/docker/restartmanager"
)

const (
	// InitFriendlyName is the name given in the lookup map of processes
	// for the first process started in a container.
	InitFriendlyName = "init"
	configFilename   = "config.json"
)

type containerCommon struct {
	process
	restartManager restartmanager.RestartManager
	restarting     bool
	processes      map[string]*process
	labels         map[string]string
	startedAt      time.Time
}

// WithRestartManager sets the restartmanager to be used with the container.
func WithRestartManager(rm restartmanager.RestartManager) CreateOption {
	return restartManager{rm}
}

type restartManager struct {
	rm restartmanager.RestartManager
}

func (rm restartManager) Apply(p interface{}) error {
	if pr, ok := p.(*container); ok {
		pr.restartManager = rm.rm
		return nil
	}
	return fmt.Errorf("WithRestartManager option not supported for this client")
}

// WithLabels sets labels of the container.
func WithLabels(l map[string]string) CreateOption {
	return labels{l}
}

type labels struct {
	labels map[string]string
}

func (l labels) Apply(p interface{}) error {
	if pr, ok := p.(*container); ok {
		pr.labels = l.labels
		return nil
	}
	return fmt.Errorf("WithLabels option not supported for this client")
}
