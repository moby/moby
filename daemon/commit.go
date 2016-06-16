package daemon

import (
	"encoding/json"
	"fmt"
	"runtime"
	"strings"
	"time"

	"github.com/docker/docker/api/types/backend"
	"github.com/docker/docker/builder/dockerfile"
	"github.com/docker/docker/container"
	"github.com/docker/docker/dockerversion"
	"github.com/docker/docker/image"
	"github.com/docker/docker/layer"
	"github.com/docker/docker/pkg/archive"
	"github.com/docker/docker/pkg/ioutils"
	"github.com/docker/docker/reference"
	containertypes "github.com/docker/engine-api/types/container"
	"github.com/docker/go-connections/nat"
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
		if userConf.ExposedPorts == nil {
			userConf.ExposedPorts = make(nat.PortSet)
		}
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
	if imageConf.Labels != nil {
		for l := range userConf.Labels {
			imageConf.Labels[l] = userConf.Labels[l]
		}
		userConf.Labels = imageConf.Labels
	}

	if len(userConf.Entrypoint) == 0 {
		if len(userConf.Cmd) == 0 {
			userConf.Cmd = imageConf.Cmd
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

// Commit creates a new filesystem image from the current state of a container.
// The image can optionally be tagged into a repository.
func (daemon *Daemon) Commit(name string, c *backend.ContainerCommitConfig) (string, error) {
	container, err := daemon.GetContainer(name)
	if err != nil {
		return "", err
	}

	// It is not possible to commit a running container on Windows
	if runtime.GOOS == "windows" && container.IsRunning() {
		return "", fmt.Errorf("Windows does not support commit of a running container")
	}

	if c.Pause && !container.IsPaused() {
		daemon.containerPause(container)
		defer daemon.containerUnpause(container)
	}

	newConfig, err := dockerfile.BuildFromConfig(c.Config, c.Changes)
	if err != nil {
		return "", err
	}

	if c.MergeConfigs {
		if err := merge(newConfig, container.Config); err != nil {
			return "", err
		}
	}

	rwTar, err := daemon.exportContainerRw(container)
	if err != nil {
		return "", err
	}
	defer func() {
		if rwTar != nil {
			rwTar.Close()
		}
	}()

	var history []image.History
	rootFS := image.NewRootFS()
	osVersion := ""
	var osFeatures []string

	if container.ImageID != "" {
		img, err := daemon.imageStore.Get(container.ImageID)
		if err != nil {
			return "", err
		}
		history = img.History
		rootFS = img.RootFS
		osVersion = img.OSVersion
		osFeatures = img.OSFeatures
	}

	l, err := daemon.layerStore.Register(rwTar, rootFS.ChainID())
	if err != nil {
		return "", err
	}
	defer layer.ReleaseAndLog(daemon.layerStore, l)

	h := image.History{
		Author:     c.Author,
		Created:    time.Now().UTC(),
		CreatedBy:  strings.Join(container.Config.Cmd, " "),
		Comment:    c.Comment,
		EmptyLayer: true,
	}

	if diffID := l.DiffID(); layer.DigestSHA256EmptyTar != diffID {
		h.EmptyLayer = false
		rootFS.Append(diffID)
	}

	history = append(history, h)

	config, err := json.Marshal(&image.Image{
		V1Image: image.V1Image{
			DockerVersion:   dockerversion.Version,
			Config:          newConfig,
			Architecture:    runtime.GOARCH,
			OS:              runtime.GOOS,
			Container:       container.ID,
			ContainerConfig: *container.Config,
			Author:          c.Author,
			Created:         h.Created,
		},
		RootFS:     rootFS,
		History:    history,
		OSFeatures: osFeatures,
		OSVersion:  osVersion,
	})

	if err != nil {
		return "", err
	}

	id, err := daemon.imageStore.Create(config)
	if err != nil {
		return "", err
	}

	if container.ImageID != "" {
		if err := daemon.imageStore.SetParent(id, container.ImageID); err != nil {
			return "", err
		}
	}

	if c.Repo != "" {
		newTag, err := reference.WithName(c.Repo) // todo: should move this to API layer
		if err != nil {
			return "", err
		}
		if c.Tag != "" {
			if newTag, err = reference.WithTag(newTag, c.Tag); err != nil {
				return "", err
			}
		}
		if err := daemon.TagImageWithReference(id, newTag); err != nil {
			return "", err
		}
	}

	attributes := map[string]string{
		"comment": c.Comment,
	}
	daemon.LogContainerEventWithAttributes(container, "commit", attributes)
	return id.String(), nil
}

func (daemon *Daemon) exportContainerRw(container *container.Container) (archive.Archive, error) {
	if err := daemon.Mount(container); err != nil {
		return nil, err
	}

	archive, err := container.RWLayer.TarStream()
	if err != nil {
		daemon.Unmount(container) // logging is already handled in the `Unmount` function
		return nil, err
	}
	return ioutils.NewReadCloserWrapper(archive, func() error {
			archive.Close()
			return container.RWLayer.Unmount()
		}),
		nil
}
