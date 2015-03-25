package daemon

import (
	"fmt"
	"io"

	"github.com/docker/docker/engine"
)

func (daemon *Daemon) ContainerExport(job *engine.Job) error {
	if len(job.Args) != 1 {
		return fmt.Errorf("Usage: %s container_id", job.Name)
	}
	name := job.Args[0]

	container, err := daemon.Get(name)
	if err != nil {
		return err
	}

	data, err := container.Export()
	if err != nil {
		return fmt.Errorf("%s: %s", name, err)
	}
	defer data.Close()

	// Stream the entire contents of the container (basically a volatile snapshot)
	if _, err := io.Copy(job.Stdout, data); err != nil {
		return fmt.Errorf("%s: %s", name, err)
	}
	// FIXME: factor job-specific LogEvent to engine.Job.Run()
	container.LogEvent("export")
	return nil
}
