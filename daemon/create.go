package daemon

import (
	"fmt"
	"strings"

	"github.com/docker/docker/engine"
	"github.com/docker/docker/graph"
	"github.com/docker/docker/image"
	"github.com/docker/docker/pkg/parsers"
	"github.com/docker/docker/runconfig"
	"github.com/docker/libcontainer/label"
)

func (daemon *Daemon) ContainerCreate(job *engine.Job) engine.Status {
	var name string
	if len(job.Args) == 1 {
		name = job.Args[0]
	} else if len(job.Args) > 1 {
		return job.Errorf("Usage: %s", job.Name)
	}

	config := runconfig.ContainerConfigFromJob(job)
	hostConfig := runconfig.ContainerHostConfigFromJob(job)

	if len(hostConfig.LxcConf) > 0 && !strings.Contains(daemon.ExecutionDriver().Name(), "lxc") {
		return job.Errorf("Cannot use --lxc-conf with execdriver: %s", daemon.ExecutionDriver().Name())
	}
	if hostConfig.Memory != 0 && hostConfig.Memory < 4194304 {
		return job.Errorf("Minimum memory limit allowed is 4MB")
	}
	if hostConfig.Memory > 0 && !daemon.SystemConfig().MemoryLimit {
		job.Errorf("Your kernel does not support memory limit capabilities. Limitation discarded.\n")
		hostConfig.Memory = 0
	}
	if hostConfig.Memory > 0 && hostConfig.MemorySwap != -1 && !daemon.SystemConfig().SwapLimit {
		job.Errorf("Your kernel does not support swap limit capabilities. Limitation discarded.\n")
		hostConfig.MemorySwap = -1
	}
	if hostConfig.Memory > 0 && hostConfig.MemorySwap > 0 && hostConfig.MemorySwap < hostConfig.Memory {
		return job.Errorf("Minimum memoryswap limit should be larger than memory limit, see usage.\n")
	}
	if hostConfig.Memory == 0 && hostConfig.MemorySwap > 0 {
		return job.Errorf("You should always set the Memory limit when using Memoryswap limit, see usage.\n")
	}

	container, buildWarnings, err := daemon.Create(config, hostConfig, name)
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

	job.Printf("%s\n", container.ID)

	for _, warning := range buildWarnings {
		job.Errorf("%s\n", warning)
	}

	return engine.StatusOK
}

// Create creates a new container from the given configuration with a given name.
func (daemon *Daemon) Create(config *runconfig.Config, hostConfig *runconfig.HostConfig, name string) (*Container, []string, error) {
	var (
		container *Container
		warnings  []string
		img       *image.Image
		imgID     string
		err       error
	)

	if config.Image != "" {
		img, err = daemon.repositories.LookupImage(config.Image)
		if err != nil {
			return nil, nil, err
		}
		if err = img.CheckDepth(); err != nil {
			return nil, nil, err
		}
		imgID = img.ID
	}

	if warnings, err = daemon.mergeAndVerifyConfig(config, img); err != nil {
		return nil, nil, err
	}
	if hostConfig == nil {
		hostConfig = &runconfig.HostConfig{}
	}
	if hostConfig.SecurityOpt == nil {
		hostConfig.SecurityOpt, err = daemon.GenerateSecurityOpt(hostConfig.IpcMode, hostConfig.PidMode)
		if err != nil {
			return nil, nil, err
		}
	}
	if container, err = daemon.newContainer(name, config, imgID); err != nil {
		return nil, nil, err
	}
	if err := daemon.Register(container); err != nil {
		return nil, nil, err
	}
	if err := daemon.createRootfs(container); err != nil {
		return nil, nil, err
	}
	if hostConfig != nil {
		if err := daemon.setHostConfig(container, hostConfig); err != nil {
			return nil, nil, err
		}
	}
	if err := container.Mount(); err != nil {
		return nil, nil, err
	}
	defer container.Unmount()
	if err := container.prepareVolumes(); err != nil {
		return nil, nil, err
	}
	if err := container.ToDisk(); err != nil {
		return nil, nil, err
	}
	return container, warnings, nil
}

func (daemon *Daemon) GenerateSecurityOpt(ipcMode runconfig.IpcMode, pidMode runconfig.PidMode) ([]string, error) {
	if ipcMode.IsHost() || pidMode.IsHost() {
		return label.DisableSecOpt(), nil
	}
	if ipcContainer := ipcMode.Container(); ipcContainer != "" {
		c, err := daemon.Get(ipcContainer)
		if err != nil {
			return nil, err
		}
		if !c.IsRunning() {
			return nil, fmt.Errorf("cannot join IPC of a non running container: %s", ipcContainer)
		}

		return label.DupSecOpt(c.ProcessLabel), nil
	}
	return nil, nil
}
