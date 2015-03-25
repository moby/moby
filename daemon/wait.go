package daemon

import (
	"fmt"
	"time"

	"github.com/docker/docker/engine"
)

func (daemon *Daemon) ContainerWait(job *engine.Job) error {
	if len(job.Args) != 1 {
		return fmt.Errorf("Usage: %s", job.Name)
	}
	name := job.Args[0]
	container, err := daemon.Get(name)
	if err != nil {
		return fmt.Errorf("%s: %v", job.Name, err)
	}
	status, _ := container.WaitStop(-1 * time.Second)
	job.Printf("%d\n", status)
	return nil
}
