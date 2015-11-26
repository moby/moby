package daemon

import (
	"encoding/json"
	"fmt"
	"runtime"
	"strings"
	"time"

	"github.com/docker/distribution/reference"
	"github.com/docker/docker/dockerversion"
	"github.com/docker/docker/image"
	"github.com/docker/docker/layer"
	"github.com/docker/docker/pkg/archive"
	"github.com/docker/docker/pkg/ioutils"
	"github.com/docker/docker/runconfig"
)

// ContainerCommitConfig contains build configs for commit operation,
// and is used when making a commit with the current state of the container.
type ContainerCommitConfig struct {
	Pause   bool
	Repo    string
	Tag     string
	Author  string
	Comment string
	// merge container config into commit config before commit
	MergeConfigs bool
	Config       *runconfig.Config
}

// Commit creates a new filesystem image from the current state of a container.
// The image can optionally be tagged into a repository.
func (daemon *Daemon) Commit(name string, c *ContainerCommitConfig) (string, error) {
	container, err := daemon.Get(name)
	if err != nil {
		return "", err
	}

	// It is not possible to commit a running container on Windows
	if runtime.GOOS == "windows" && container.IsRunning() {
		return "", fmt.Errorf("Windows does not support commit of a running container")
	}

	if c.Pause && !container.isPaused() {
		daemon.containerPause(container)
		defer daemon.containerUnpause(container)
	}

	if c.MergeConfigs {
		if err := runconfig.Merge(c.Config, container.Config); err != nil {
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

	if container.ImageID != "" {
		img, err := daemon.imageStore.Get(container.ImageID)
		if err != nil {
			return "", err
		}
		history = img.History
		rootFS = img.RootFS
	}

	l, err := daemon.layerStore.Register(rwTar, rootFS.ChainID())
	if err != nil {
		return "", err
	}
	defer layer.ReleaseAndLog(daemon.layerStore, l)

	h := image.History{
		Author:     c.Author,
		Created:    time.Now().UTC(),
		CreatedBy:  strings.Join(container.Config.Cmd.Slice(), " "),
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
			Config:          c.Config,
			Architecture:    runtime.GOARCH,
			OS:              runtime.GOOS,
			Container:       container.ID,
			ContainerConfig: *container.Config,
			Author:          c.Author,
			Created:         h.Created,
		},
		RootFS:  rootFS,
		History: history,
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
		if err := daemon.TagImage(newTag, id.String(), true); err != nil {
			return "", err
		}
	}

	daemon.LogContainerEvent(container, "commit")
	return id.String(), nil
}

func (daemon *Daemon) exportContainerRw(container *Container) (archive.Archive, error) {
	if err := daemon.Mount(container); err != nil {
		return nil, err
	}

	archive, err := container.rwlayer.TarStream()
	if err != nil {
		return nil, err
	}
	return ioutils.NewReadCloserWrapper(archive, func() error {
			archive.Close()
			return daemon.layerStore.Unmount(container.ID)
		}),
		nil
}
