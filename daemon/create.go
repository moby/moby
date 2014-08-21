package daemon

import (
	"github.com/docker/docker/engine"
	"github.com/docker/docker/graph"
	"github.com/docker/docker/pkg/parsers"
	"github.com/docker/docker/runconfig"
)

func (daemon *Daemon) ContainerCreate(job *engine.Job) engine.Status {
	var name string
	if len(job.Args) == 1 {
		name = job.Args[0]
	} else if len(job.Args) > 1 {
		return job.Errorf("Usage: %s", job.Name)
	}
	config := runconfig.ContainerConfigFromJob(job)
	if config.Memory != 0 && config.Memory < 524288 {
		return job.Errorf("Minimum memory limit allowed is 512k")
	}
	if config.Memory > 0 && !daemon.SystemConfig().MemoryLimit {
		job.Errorf("Your kernel does not support memory limit capabilities. Limitation discarded.\n")
		config.Memory = 0
	}
	if config.Memory > 0 && !daemon.SystemConfig().SwapLimit {
		job.Errorf("Your kernel does not support swap limit capabilities. Limitation discarded.\n")
		config.MemorySwap = -1
	}
	container, buildWarnings, err := daemon.Create(config, name)
	if err != nil {
		if daemon.Graph().IsNotExist(err) {
			_, tag := parsers.ParseRepositoryTag(config.Image)
			if tag == "" {
				tag = graph.DEFAULTTAG
			}
			return job.Errorf("No such image: %s (tag: %s)", config.Image, tag)
		}
		return job.Error(err)
	}
	if !container.Config.NetworkDisabled && daemon.SystemConfig().IPv4ForwardingDisabled {
		job.Errorf("IPv4 forwarding is disabled.\n")
	}
	container.LogEvent("create")
	// FIXME: this is necessary because daemon.Create might return a nil container
	// with a non-nil error. This should not happen! Once it's fixed we
	// can remove this workaround.
	if container != nil {
		job.Printf("%s\n", container.ID)
	}
	for _, warning := range buildWarnings {
		job.Errorf("%s\n", warning)
	}
	return engine.StatusOK
}

// Create creates a new container from the given configuration with a given name.
func (daemon *Daemon) Create(config *runconfig.Config, name string) (*Container, []string, error) {
	var (
		container *Container
		warnings  []string
	)

	img, err := daemon.repositories.LookupImage(config.Image)
	if err != nil {
		return nil, nil, err
	}
	if err := img.CheckDepth(); err != nil {
		return nil, nil, err
	}
	if warnings, err = daemon.mergeAndVerifyConfig(config, img); err != nil {
		return nil, nil, err
	}
	if container, err = daemon.newContainer(name, config, img); err != nil {
		return nil, nil, err
	}
	if err := daemon.createRootfs(container, img); err != nil {
		return nil, nil, err
	}
	if err := container.ToDisk(); err != nil {
		return nil, nil, err
	}
	if err := daemon.Register(container); err != nil {
		return nil, nil, err
	}
	return container, warnings, nil
}
