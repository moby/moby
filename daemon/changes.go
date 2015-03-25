package daemon

import (
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

	outs := engine.NewTable("", 0)
	changes, err := container.Changes()
	if err != nil {
		return err
	}

	for _, change := range changes {
		out := &engine.Env{}
		if err := out.Import(change); err != nil {
			return err
		}
		outs.Add(out)
	}

	if _, err := outs.WriteListTo(job.Stdout); err != nil {
		return err
	}

	return nil
}
