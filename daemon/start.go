package daemon

import (
	"fmt"
	"os"
	"strings"

	"github.com/docker/docker/engine"
	"github.com/docker/docker/runconfig"
)

func (daemon *Daemon) ContainerStart(job *engine.Job) engine.Status {
	if len(job.Args) < 1 {
		return job.Errorf("Usage: %s container_id", job.Name)
	}
	var (
		name      = job.Args[0]
		container = daemon.Get(name)
	)

	if container == nil {
		return job.Errorf("No such container: %s", name)
	}

	if container.IsRunning() {
		return job.Errorf("Container already started")
	}

	// If no environment was set, then no hostconfig was passed.
	// This is kept for backward compatibility - hostconfig should be passed when
	// creating a container, not during start.
	if len(job.Environ()) > 0 {
		hostConfig := runconfig.ContainerHostConfigFromJob(job)
		if err := daemon.setHostConfig(container, hostConfig); err != nil {
			return job.Error(err)
		}
	}
	if err := container.Start(); err != nil {
		container.LogEvent("die")
		return job.Errorf("Cannot start container %s: %s", name, err)
	}

	return engine.StatusOK
}

func (daemon *Daemon) setHostConfig(container *Container, hostConfig *runconfig.HostConfig) error {
	if err := parseSecurityOpt(container, hostConfig); err != nil {
		return err
	}
	// Validate the HostConfig binds. Make sure that:
	// the source exists
	for _, bind := range hostConfig.Binds {
		splitBind := strings.Split(bind, ":")
		source := splitBind[0]

		// ensure the source exists on the host
		_, err := os.Stat(source)
		if err != nil && os.IsNotExist(err) {
			err = os.MkdirAll(source, 0755)
			if err != nil {
				return fmt.Errorf("Could not create local directory '%s' for bind mount: %s!", source, err.Error())
			}
		}
	}
	// Register any links from the host config before starting the container
	if err := daemon.RegisterLinks(container, hostConfig); err != nil {
		return err
	}
	container.SetHostConfig(hostConfig)
	container.ToDisk()

	return nil
}
