package daemon

import (
	"encoding/json"
	"fmt"

	"github.com/docker/docker/engine"
)

func (daemon *Daemon) ContainerChanges(job *engine.Job) error {
	if n := len(job.Args); n != 1 {
		return fmt.Errorf("Usage: %s CONTAINER", job.Name)
	}
	name := job.Args[0]

	container, err := daemon.Get(name)
	if err != nil {
		return err
	}

	changes, err := container.Changes()
	if err != nil {
		return err
	}

	if err = json.NewEncoder(job.Stdout).Encode(changes); err != nil {
		return err
	}

	return nil
}
