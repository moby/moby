package daemon

import (
	"strings"

	"github.com/Sirupsen/logrus"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/context"
	derr "github.com/docker/docker/errors"
	"github.com/docker/docker/graph/tags"
	"github.com/docker/docker/image"
	"github.com/docker/docker/pkg/parsers"
	"github.com/docker/docker/pkg/stringid"
	"github.com/docker/docker/runconfig"
	"github.com/opencontainers/runc/libcontainer/label"
)

// ContainerCreate takes configs and creates a container.
func (daemon *Daemon) ContainerCreate(ctx context.Context, name string, config *runconfig.Config, hostConfig *runconfig.HostConfig, adjustCPUShares bool) (*Container, []string, error) {
	if config == nil {
		return nil, nil, derr.ErrorCodeEmptyConfig
	}

	warnings, err := daemon.verifyContainerSettings(ctx, hostConfig, config)
	if err != nil {
		return nil, warnings, err
	}

	daemon.adaptContainerSettings(hostConfig, adjustCPUShares)

	container, buildWarnings, err := daemon.Create(ctx, config, hostConfig, name)
	if err != nil {
		if daemon.Graph().IsNotExist(err, config.Image) {
			if strings.Contains(config.Image, "@") {
				return nil, warnings, derr.ErrorCodeNoSuchImageHash.WithArgs(config.Image)
			}
			img, tag := parsers.ParseRepositoryTag(config.Image)
			if tag == "" {
				tag = tags.DefaultTag
			}
			return nil, warnings, derr.ErrorCodeNoSuchImageTag.WithArgs(img, tag)
		}
		return nil, warnings, err
	}

	warnings = append(warnings, buildWarnings...)

	return container, warnings, nil
}

// Create creates a new container from the given configuration with a given name.
func (daemon *Daemon) Create(ctx context.Context, config *runconfig.Config, hostConfig *runconfig.HostConfig, name string) (retC *Container, retS []string, retErr error) {
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
		hostConfig.SecurityOpt, err = daemon.generateSecurityOpt(ctx, hostConfig.IpcMode, hostConfig.PidMode)
		if err != nil {
			return nil, nil, err
		}
	}
	if container, err = daemon.newContainer(ctx, name, config, imgID); err != nil {
		return nil, nil, err
	}
	defer func() {
		if retErr != nil {
			if err := daemon.rm(ctx, container, false); err != nil {
				logrus.Errorf("Clean up Error! Cannot destroy container %s: %v", container.ID, err)
			}
		}
	}()

	if err := daemon.Register(ctx, container); err != nil {
		return nil, nil, err
	}
	if err := daemon.createRootfs(container); err != nil {
		return nil, nil, err
	}
	if err := daemon.setHostConfig(ctx, container, hostConfig); err != nil {
		return nil, nil, err
	}
	defer func() {
		if retErr != nil {
			if err := container.removeMountPoints(true); err != nil {
				logrus.Error(err)
			}
		}
	}()
	if err := container.Mount(ctx); err != nil {
		return nil, nil, err
	}
	defer container.Unmount(ctx)

	if err := createContainerPlatformSpecificSettings(container, config, hostConfig, img); err != nil {
		return nil, nil, err
	}

	if err := container.toDiskLocking(); err != nil {
		logrus.Errorf("Error saving new container to disk: %v", err)
		return nil, nil, err
	}
	container.logEvent(ctx, "create")
	return container, warnings, nil
}

func (daemon *Daemon) generateSecurityOpt(ctx context.Context, ipcMode runconfig.IpcMode, pidMode runconfig.PidMode) ([]string, error) {
	if ipcMode.IsHost() || pidMode.IsHost() {
		return label.DisableSecOpt(), nil
	}
	if ipcContainer := ipcMode.Container(); ipcContainer != "" {
		c, err := daemon.Get(ctx, ipcContainer)
		if err != nil {
			return nil, err
		}

		return label.DupSecOpt(c.ProcessLabel), nil
	}
	return nil, nil
}

// VolumeCreate creates a volume with the specified name, driver, and opts
// This is called directly from the remote API
func (daemon *Daemon) VolumeCreate(ctx context.Context, name, driverName string, opts map[string]string) (*types.Volume, error) {
	if name == "" {
		name = stringid.GenerateNonCryptoID()
	}

	v, err := daemon.volumes.Create(name, driverName, opts)
	if err != nil {
		return nil, err
	}
	return volumeToAPIType(v), nil
}
