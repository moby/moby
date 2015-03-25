package daemon

import (
	"fmt"
	"strconv"

	"github.com/docker/docker/engine"
)

func (daemon *Daemon) ContainerResize(job *engine.Job) error {
	if len(job.Args) != 3 {
		return fmt.Errorf("Not enough arguments. Usage: %s CONTAINER HEIGHT WIDTH\n", job.Name)
	}
	name := job.Args[0]
	height, err := strconv.Atoi(job.Args[1])
	if err != nil {
		return err
	}
	width, err := strconv.Atoi(job.Args[2])
	if err != nil {
		return err
	}
	container, err := daemon.Get(name)
	if err != nil {
		return err
	}
	if err := container.Resize(height, width); err != nil {
		return err
	}
	return nil
}

func (daemon *Daemon) ContainerExecResize(job *engine.Job) error {
	if len(job.Args) != 3 {
		return fmt.Errorf("Not enough arguments. Usage: %s EXEC HEIGHT WIDTH\n", job.Name)
	}
	name := job.Args[0]
	height, err := strconv.Atoi(job.Args[1])
	if err != nil {
		return err
	}
	width, err := strconv.Atoi(job.Args[2])
	if err != nil {
		return err
	}
	execConfig, err := daemon.getExecConfig(name)
	if err != nil {
		return err
	}
	if err := execConfig.Resize(height, width); err != nil {
		return err
	}
	return nil
}
