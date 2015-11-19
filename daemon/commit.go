package daemon

import (
	"fmt"
	"runtime"

	"github.com/docker/docker/image"
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
func (daemon *Daemon) Commit(name string, c *ContainerCommitConfig) (*image.Image, error) {
	container, err := daemon.Get(name)
	if err != nil {
		return nil, err
	}

	// It is not possible to commit a running container on Windows
	if runtime.GOOS == "windows" && container.IsRunning() {
		return nil, fmt.Errorf("Windows does not support commit of a running container")
	}

	if c.Pause && !container.isPaused() {
		daemon.containerPause(container)
		defer daemon.containerUnpause(container)
	}

	if c.MergeConfigs {
		if err := runconfig.Merge(c.Config, container.Config); err != nil {
			return nil, err
		}
	}

	rwTar, err := daemon.exportContainerRw(container)
	if err != nil {
		return nil, err
	}
	defer func() {
		if rwTar != nil {
			rwTar.Close()
		}
	}()

	// Create a new image from the container's base layers + a new layer from container changes
	img, err := daemon.graph.Create(rwTar, container.ID, container.ImageID, c.Comment, c.Author, container.Config, c.Config)
	if err != nil {
		return nil, err
	}

	// Register the image if needed
	if c.Repo != "" {
		if err := daemon.repositories.Tag(c.Repo, c.Tag, img.ID, true); err != nil {
			return img, err
		}
	}

	daemon.LogContainerEvent(container, "commit")
	return img, nil
}

func (daemon *Daemon) exportContainerRw(container *Container) (archive.Archive, error) {
	archive, err := daemon.diff(container)
	if err != nil {
		return nil, err
	}
	return ioutils.NewReadCloserWrapper(archive, func() error {
			err := archive.Close()
			return err
		}),
		nil
}
