package daemon

import (
	"fmt"

	"github.com/docker/docker/engine"
)

func (daemon *Daemon) ContainerRestart(job *engine.Job) error {
	if len(job.Args) != 1 {
		return fmt.Errorf("Usage: %s CONTAINER\n", job.Name)
	}
	var (
		name = job.Args[0]
		t    = 10
	)
	if job.EnvExists("t") {
		t = job.GetenvInt("t")
	}
	container, err := daemon.Get(name)
	if err != nil {
		return err
	}
	if err := container.Restart(int(t)); err != nil {
		return fmt.Errorf("Cannot restart container %s: %s\n", name, err)
	}
	container.LogEvent("restart")
	return nil
}
