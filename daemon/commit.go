package daemon

import (
	"fmt"
	"runtime"

	"github.com/docker/docker/image"
	"github.com/docker/docker/runconfig"
)

type ContainerCommitConfig struct {
	Pause   bool
	Repo    string
	Tag     string
	Author  string
	Comment string
	Config  *runconfig.Config
}

// Commit creates a new filesystem image from the current state of a container.
// The image can optionally be tagged into a repository
func (daemon *Daemon) Commit(name string, c *ContainerCommitConfig) (*image.Image, error) {
	container, err := daemon.Get(name)
	if err != nil {
		return nil, err
	}

	if runtime.GOOS == "windows" {
		if container.IsRunning() {
			return nil, fmt.Errorf("Windows does not support commit of a running container")
		}
	}

	if c.Pause && !container.IsPaused() {
		container.Pause()
		defer container.Unpause()
	}

	rwTar, err := container.ExportRw()
	if err != nil {
		return nil, err
	}
	defer func() {
		if rwTar != nil {
			rwTar.Close()
		}
	}()

	// Create a new image from the container's base layers + a new layer from container changes
	var (
		containerID, parentImageID string
		containerConfig            *runconfig.Config
	)

	if container != nil {
		containerID = container.ID
		parentImageID = container.ImageID
		containerConfig = container.Config
	}

	img, err := daemon.graph.Create(rwTar, containerID, parentImageID, c.Comment, c.Author, containerConfig, c.Config)
	if err != nil {
		return nil, err
	}

	// Register the image if needed
	if c.Repo != "" {
		if err := daemon.repositories.Tag(c.Repo, c.Tag, img.ID, true); err != nil {
			return img, err
		}
	}
	container.LogEvent("commit")
	return img, nil
}
