package daemon // import "github.com/docker/docker/daemon"

import (
	"encoding/json"
	"fmt"
	"io"
	"runtime"
	"strings"
	"time"

	"github.com/docker/docker/api/types/backend"
	containertypes "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/builder/dockerfile"
	"github.com/docker/docker/errdefs"
	"github.com/docker/docker/image"
	"github.com/docker/docker/layer"
	"github.com/docker/docker/pkg/ioutils"
	"github.com/docker/docker/pkg/system"
	"github.com/pkg/errors"
)

// merge merges two Config, the image container configuration (defaults values),
// and the user container configuration, either passed by the API or generated
// by the cli.
// It will mutate the specified user configuration (userConf) with the image
// configuration where the user configuration is incomplete.
func merge(userConf, imageConf *containertypes.Config) error {
	if userConf.User == "" {
		userConf.User = imageConf.User
	}
	if len(userConf.ExposedPorts) == 0 {
		userConf.ExposedPorts = imageConf.ExposedPorts
	} else if imageConf.ExposedPorts != nil {
		for port := range imageConf.ExposedPorts {
			if _, exists := userConf.ExposedPorts[port]; !exists {
				userConf.ExposedPorts[port] = struct{}{}
			}
		}
	}

	if len(userConf.Env) == 0 {
		userConf.Env = imageConf.Env
	} else {
		for _, imageEnv := range imageConf.Env {
			found := false
			imageEnvKey := strings.Split(imageEnv, "=")[0]
			for _, userEnv := range userConf.Env {
				userEnvKey := strings.Split(userEnv, "=")[0]
				if runtime.GOOS == "windows" {
					// Case insensitive environment variables on Windows
					imageEnvKey = strings.ToUpper(imageEnvKey)
					userEnvKey = strings.ToUpper(userEnvKey)
				}
				if imageEnvKey == userEnvKey {
					found = true
					break
				}
			}
			if !found {
				userConf.Env = append(userConf.Env, imageEnv)
			}
		}
	}

	if userConf.Labels == nil {
		userConf.Labels = map[string]string{}
	}
	for l, v := range imageConf.Labels {
		if _, ok := userConf.Labels[l]; !ok {
			userConf.Labels[l] = v
		}
	}

	if len(userConf.Entrypoint) == 0 {
		if len(userConf.Cmd) == 0 {
			userConf.Cmd = imageConf.Cmd
			userConf.ArgsEscaped = imageConf.ArgsEscaped
		}

		if userConf.Entrypoint == nil {
			userConf.Entrypoint = imageConf.Entrypoint
		}
	}
	if imageConf.Healthcheck != nil {
		if userConf.Healthcheck == nil {
			userConf.Healthcheck = imageConf.Healthcheck
		} else {
			if len(userConf.Healthcheck.Test) == 0 {
				userConf.Healthcheck.Test = imageConf.Healthcheck.Test
			}
			if userConf.Healthcheck.Interval == 0 {
				userConf.Healthcheck.Interval = imageConf.Healthcheck.Interval
			}
			if userConf.Healthcheck.Timeout == 0 {
				userConf.Healthcheck.Timeout = imageConf.Healthcheck.Timeout
			}
			if userConf.Healthcheck.StartPeriod == 0 {
				userConf.Healthcheck.StartPeriod = imageConf.Healthcheck.StartPeriod
			}
			if userConf.Healthcheck.Retries == 0 {
				userConf.Healthcheck.Retries = imageConf.Healthcheck.Retries
			}
		}
	}

	if userConf.WorkingDir == "" {
		userConf.WorkingDir = imageConf.WorkingDir
	}
	if len(userConf.Volumes) == 0 {
		userConf.Volumes = imageConf.Volumes
	} else {
		for k, v := range imageConf.Volumes {
			userConf.Volumes[k] = v
		}
	}

	if userConf.StopSignal == "" {
		userConf.StopSignal = imageConf.StopSignal
	}
	return nil
}

// CreateImageFromContainer creates a new image from a container. The container
// config will be updated by applying the change set to the custom config, then
// applying that config over the existing container config.
func (daemon *Daemon) CreateImageFromContainer(name string, c *backend.CreateImageConfig) (string, error) {
	start := time.Now()
	container, err := daemon.GetContainer(name)
	if err != nil {
		return "", err
	}

	// It is not possible to commit a running container on Windows
	if (runtime.GOOS == "windows") && container.IsRunning() {
		return "", errors.Errorf("%+v does not support commit of a running container", runtime.GOOS)
	}

	if container.IsDead() {
		err := fmt.Errorf("You cannot commit container %s which is Dead", container.ID)
		return "", errdefs.Conflict(err)
	}

	if container.IsRemovalInProgress() {
		err := fmt.Errorf("You cannot commit container %s which is being removed", container.ID)
		return "", errdefs.Conflict(err)
	}

	if c.Pause && !container.IsPaused() {
		daemon.containerPause(container)
		defer daemon.containerUnpause(container)
	}

	if c.Config == nil {
		c.Config = container.Config
	}
	newConfig, err := dockerfile.BuildFromConfig(c.Config, c.Changes, container.OS)
	if err != nil {
		return "", err
	}
	if err := merge(newConfig, container.Config); err != nil {
		return "", err
	}

	id, err := daemon.commitImage(backend.CommitConfig{
		Author:              c.Author,
		Comment:             c.Comment,
		Config:              newConfig,
		ContainerConfig:     container.Config,
		ContainerID:         container.ID,
		ContainerMountLabel: container.MountLabel,
		ContainerOS:         container.OS,
		ParentImageID:       string(container.ImageID),
	})
	if err != nil {
		return "", err
	}

	var imageRef string
	if c.Repo != "" {
		imageRef, err = daemon.TagImage(string(id), c.Repo, c.Tag)
		if err != nil {
			return "", err
		}
	}
	daemon.LogContainerEventWithAttributes(container, "commit", map[string]string{
		"comment":  c.Comment,
		"imageID":  id.String(),
		"imageRef": imageRef,
	})
	containerActions.WithValues("commit").UpdateSince(start)
	return id.String(), nil
}

func (daemon *Daemon) commitImage(c backend.CommitConfig) (image.ID, error) {
	layerStore, ok := daemon.layerStores[c.ContainerOS]
	if !ok {
		return "", system.ErrNotSupportedOperatingSystem
	}
	rwTar, err := exportContainerRw(layerStore, c.ContainerID, c.ContainerMountLabel)
	if err != nil {
		return "", err
	}
	defer func() {
		if rwTar != nil {
			rwTar.Close()
		}
	}()

	var parent *image.Image
	if c.ParentImageID == "" {
		parent = new(image.Image)
		parent.RootFS = image.NewRootFS()
	} else {
		parent, err = daemon.imageStore.Get(image.ID(c.ParentImageID))
		if err != nil {
			return "", err
		}
	}

	l, err := layerStore.Register(rwTar, parent.RootFS.ChainID())
	if err != nil {
		return "", err
	}
	defer layer.ReleaseAndLog(layerStore, l)

	cc := image.ChildConfig{
		ContainerID:     c.ContainerID,
		Author:          c.Author,
		Comment:         c.Comment,
		ContainerConfig: c.ContainerConfig,
		Config:          c.Config,
		DiffID:          l.DiffID(),
	}
	config, err := json.Marshal(image.NewChildImage(parent, cc, c.ContainerOS))
	if err != nil {
		return "", err
	}

	id, err := daemon.imageStore.Create(config)
	if err != nil {
		return "", err
	}

	if c.ParentImageID != "" {
		if err := daemon.imageStore.SetParent(id, image.ID(c.ParentImageID)); err != nil {
			return "", err
		}
	}
	return id, nil
}

func exportContainerRw(layerStore layer.Store, id, mountLabel string) (arch io.ReadCloser, err error) {
	rwlayer, err := layerStore.GetRWLayer(id)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err != nil {
			layerStore.ReleaseRWLayer(rwlayer)
		}
	}()

	// TODO: this mount call is not necessary as we assume that TarStream() should
	// mount the layer if needed. But the Diff() function for windows requests that
	// the layer should be mounted when calling it. So we reserve this mount call
	// until windows driver can implement Diff() interface correctly.
	_, err = rwlayer.Mount(mountLabel)
	if err != nil {
		return nil, err
	}

	archive, err := rwlayer.TarStream()
	if err != nil {
		rwlayer.Unmount()
		return nil, err
	}
	return ioutils.NewReadCloserWrapper(archive, func() error {
			archive.Close()
			err = rwlayer.Unmount()
			layerStore.ReleaseRWLayer(rwlayer)
			return err
		}),
		nil
}

// CommitBuildStep is used by the builder to create an image for each step in
// the build.
//
// This method is different from CreateImageFromContainer:
//   * it doesn't attempt to validate container state
//   * it doesn't send a commit action to metrics
//   * it doesn't log a container commit event
//
// This is a temporary shim. Should be removed when builder stops using commit.
func (daemon *Daemon) CommitBuildStep(c backend.CommitConfig) (image.ID, error) {
	container, err := daemon.GetContainer(c.ContainerID)
	if err != nil {
		return "", err
	}
	c.ContainerMountLabel = container.MountLabel
	c.ContainerOS = container.OS
	c.ParentImageID = string(container.ImageID)
	return daemon.commitImage(c)
}
