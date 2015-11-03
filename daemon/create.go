package daemon

import (
	"strings"

	"github.com/Sirupsen/logrus"
	"github.com/docker/docker/api/types"
	derr "github.com/docker/docker/errors"
	"github.com/docker/docker/graph/tags"
	"github.com/docker/docker/image"
	"github.com/docker/docker/pkg/parsers"
	"github.com/docker/docker/pkg/stringid"
	"github.com/docker/docker/runconfig"
	"github.com/docker/docker/volume"
	"github.com/opencontainers/runc/libcontainer/label"
)

// ContainerCreateConfig is the parameter set to ContainerCreate()
type ContainerCreateConfig struct {
	Name            string
	Config          *runconfig.Config
	HostConfig      *runconfig.HostConfig
	AdjustCPUShares bool
}

// ContainerCreate takes configs and creates a container.
func (daemon *Daemon) ContainerCreate(params *ContainerCreateConfig) (types.ContainerCreateResponse, error) {
	if params.Config == nil {
		return types.ContainerCreateResponse{}, derr.ErrorCodeEmptyConfig
	}

	warnings, err := daemon.verifyContainerSettings(params.HostConfig, params.Config)
	if err != nil {
		return types.ContainerCreateResponse{ID: "", Warnings: warnings}, err
	}

	daemon.adaptContainerSettings(params.HostConfig, params.AdjustCPUShares)

	container, err := daemon.create(params)
	if err != nil {
		if daemon.Graph().IsNotExist(err, params.Config.Image) {
			if strings.Contains(params.Config.Image, "@") {
				return types.ContainerCreateResponse{ID: "", Warnings: warnings}, derr.ErrorCodeNoSuchImageHash.WithArgs(params.Config.Image)
			}
			img, tag := parsers.ParseRepositoryTag(params.Config.Image)
			if tag == "" {
				tag = tags.DefaultTag
			}
			return types.ContainerCreateResponse{ID: "", Warnings: warnings}, derr.ErrorCodeNoSuchImageTag.WithArgs(img, tag)
		}
		return types.ContainerCreateResponse{ID: "", Warnings: warnings}, err
	}

	return types.ContainerCreateResponse{ID: container.ID, Warnings: warnings}, nil
}

// Create creates a new container from the given configuration with a given name.
func (daemon *Daemon) create(params *ContainerCreateConfig) (retC *Container, retErr error) {
	var (
		container *Container
		img       *image.Image
		imgID     string
		err       error
	)

	if params.Config.Image != "" {
		img, err = daemon.repositories.LookupImage(params.Config.Image)
		if err != nil {
			return nil, err
		}
		if err = daemon.graph.CheckDepth(img); err != nil {
			return nil, err
		}
		imgID = img.ID
	}

	if err := daemon.mergeAndVerifyConfig(params.Config, img); err != nil {
		return nil, err
	}

	if params.HostConfig == nil {
		params.HostConfig = &runconfig.HostConfig{}
	}
	if params.HostConfig.SecurityOpt == nil {
		params.HostConfig.SecurityOpt, err = daemon.generateSecurityOpt(params.HostConfig.IpcMode, params.HostConfig.PidMode)
		if err != nil {
			return nil, err
		}
	}
	if container, err = daemon.newContainer(params.Name, params.Config, imgID); err != nil {
		return nil, err
	}
	defer func() {
		if retErr != nil {
			if err := daemon.rm(container, false); err != nil {
				logrus.Errorf("Clean up Error! Cannot destroy container %s: %v", container.ID, err)
			}
		}
	}()

	if err := daemon.Register(container); err != nil {
		return nil, err
	}
	if err := daemon.createRootfs(container); err != nil {
		return nil, err
	}
	if err := daemon.setHostConfig(container, params.HostConfig); err != nil {
		return nil, err
	}
	defer func() {
		if retErr != nil {
			if err := daemon.removeMountPoints(container, true); err != nil {
				logrus.Error(err)
			}
		}
	}()
	if err := daemon.Mount(container); err != nil {
		return nil, err
	}
	defer daemon.Unmount(container)

	if err := createContainerPlatformSpecificSettings(container, params.Config, params.HostConfig, img); err != nil {
		return nil, err
	}

	if err := container.toDiskLocking(); err != nil {
		logrus.Errorf("Error saving new container to disk: %v", err)
		return nil, err
	}
	daemon.logContainerEvent(container, "create")
	return container, nil
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

	// keep "docker run -v existing_volume:/foo --volume-driver other_driver" work
	if (driverName != "" && v.DriverName() != driverName) || (driverName == "" && v.DriverName() != volume.DefaultDriverName) {
		return nil, derr.ErrorVolumeNameTaken.WithArgs(name, v.DriverName())
	}
	return volumeToAPIType(v), nil
}
