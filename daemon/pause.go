package daemon

import (
	"fmt"

	"github.com/docker/docker/engine"
)

func (daemon *Daemon) ContainerPause(job *engine.Job) error {
	if len(job.Args) != 1 {
		return fmt.Errorf("Usage: %s CONTAINER", job.Name)
	}
	name := job.Args[0]
	container, err := daemon.Get(name)
	if err != nil {
		return err
	}
	if err := container.Pause(); err != nil {
		return fmt.Errorf("Cannot pause container %s: %s", name, err)
	}
	container.LogEvent("pause")
	return nil
}

func (daemon *Daemon) ContainerUnpause(job *engine.Job) error {
	if n := len(job.Args); n < 1 || n > 2 {
		return fmt.Errorf("Usage: %s CONTAINER", job.Name)
	}
	name := job.Args[0]
	container, err := daemon.Get(name)
	if err != nil {
		return err
	}
	if err := container.Unpause(); err != nil {
		return fmt.Errorf("Cannot unpause container %s: %s", name, err)
	}
	container.LogEvent("unpause")
	return nil
}
