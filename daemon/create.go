package daemon

import (
	"fmt"
	"strings"

	"github.com/Sirupsen/logrus"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/graph/tags"
	"github.com/docker/docker/image"
	"github.com/docker/docker/pkg/parsers"
	"github.com/docker/docker/pkg/stringid"
	"github.com/docker/docker/runconfig"
	"github.com/opencontainers/runc/libcontainer/label"
)

// ContainerCreate takes configs and creates a container.
func (daemon *Daemon) ContainerCreate(name string, config *runconfig.Config, hostConfig *runconfig.HostConfig, adjustCPUShares bool) (*Container, []string, error) {
	if config == nil {
		return nil, nil, fmt.Errorf("Config cannot be empty in order to create a container")
	}

	warnings, err := daemon.verifyContainerSettings(hostConfig, config)
	daemon.adaptContainerSettings(hostConfig, adjustCPUShares)
	if err != nil {
		return nil, warnings, err
	}

	container, buildWarnings, err := daemon.Create(config, hostConfig, name)
	if err != nil {
		if daemon.Graph().IsNotExist(err, config.Image) {
			if strings.Contains(config.Image, "@") {
				return nil, warnings, fmt.Errorf("No such image: %s", config.Image)
			}
			img, tag := parsers.ParseRepositoryTag(config.Image)
			if tag == "" {
				tag = tags.DefaultTag
			}
			return nil, warnings, fmt.Errorf("No such image: %s:%s", img, tag)
		}
		return nil, warnings, err
	}

	warnings = append(warnings, buildWarnings...)

	return container, warnings, nil
}

// Create creates a new container from the given configuration with a given name.
func (daemon *Daemon) Create(config *runconfig.Config, hostConfig *runconfig.HostConfig, name string) (retC *Container, retS []string, retErr error) {
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
		if err = daemon.graph.CheckDepth(img); err != nil {
			return nil, nil, err
		}
		imgID = img.ID
	}

	if err := daemon.mergeAndVerifyConfig(config, img); err != nil {
		return nil, nil, err
	}

	if hostConfig == nil {
		hostConfig = &runconfig.HostConfig{}
	}
	if hostConfig.SecurityOpt == nil {
		hostConfig.SecurityOpt, err = daemon.generateSecurityOpt(hostConfig.IpcMode, hostConfig.PidMode)
		if err != nil {
			return nil, nil, err
		}
	}
	if container, err = daemon.newContainer(name, config, imgID); err != nil {
		return nil, nil, err
	}
	defer func() {
		if retErr != nil {
			if err := daemon.rm(container, false); err != nil {
				logrus.Errorf("Clean up Error! Cannot destroy container %s: %v", container.ID, err)
			}
		}
	}()

	if err := daemon.Register(container); err != nil {
		return nil, nil, err
	}
	if err := daemon.createRootfs(container); err != nil {
		return nil, nil, err
	}
	if err := daemon.setHostConfig(container, hostConfig); err != nil {
		return nil, nil, err
	}
	if err := container.Mount(); err != nil {
		return nil, nil, err
	}
	defer container.Unmount()

	if err := createContainerPlatformSpecificSettings(container, config, img); err != nil {
		return nil, nil, err
	}

	if err := container.toDiskLocking(); err != nil {
		logrus.Errorf("Error saving new container to disk: %v", err)
		return nil, nil, err
	}
	container.logEvent("create")
	return container, warnings, nil
}

func (daemon *Daemon) generateSecurityOpt(ipcMode runconfig.IpcMode, pidMode runconfig.PidMode) ([]string, error) {
	if ipcMode.IsHost() || pidMode.IsHost() {
		return label.DisableSecOpt(), nil
	}
	if ipcContainer := ipcMode.Container(); ipcContainer != "" {
		c, err := daemon.Get(ipcContainer)
		if err != nil {
			return nil, err
		}

		return label.DupSecOpt(c.ProcessLabel), nil
	}
	return nil, nil
}

// VolumeCreate creates a volume with the specified name, driver, and opts
// This is called directly from the remote API
func (daemon *Daemon) VolumeCreate(name, driverName string, opts map[string]string) (*types.Volume, error) {
	if name == "" {
		name = stringid.GenerateNonCryptoID()
	}

	v, err := daemon.volumes.Create(name, driverName, opts)
	if err != nil {
		return nil, err
	}
	return volumeToAPIType(v), nil
}
