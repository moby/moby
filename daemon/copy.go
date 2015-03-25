package daemon

import (
	"fmt"
	"io"

	"github.com/docker/docker/engine"
)

func (daemon *Daemon) ContainerCopy(job *engine.Job) error {
	if len(job.Args) != 2 {
		return fmt.Errorf("Usage: %s CONTAINER RESOURCE\n", job.Name)
	}

	var (
		name     = job.Args[0]
		resource = job.Args[1]
	)

	container, err := daemon.Get(name)
	if err != nil {
		return err
	}

	data, err := container.Copy(resource)
	if err != nil {
		return err
	}
	defer data.Close()

	if _, err := io.Copy(job.Stdout, data); err != nil {
		return err
	}
	return nil
}
