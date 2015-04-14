package daemon

import (
	"bytes"
	"encoding/json"
	"io"

	"github.com/Sirupsen/logrus"
	"github.com/docker/docker/engine"
	"github.com/docker/docker/image"
	"github.com/docker/docker/runconfig"
)

type ContainerCommitConfig struct {
	Pause   bool
	Repo    string
	Tag     string
	Author  string
	Comment string
	Changes []string
	Config  io.ReadCloser
}

func (daemon *Daemon) ContainerCommit(name string, c *ContainerCommitConfig) (string, error) {
	container, err := daemon.Get(name)
	if err != nil {
		return "", err
	}

	var (
		subenv       engine.Env
		config       = container.Config
		stdoutBuffer = bytes.NewBuffer(nil)
		newConfig    runconfig.Config
	)

	if err := subenv.Decode(c.Config); err != nil {
		logrus.Errorf("%s", err)
	}

	buildConfigJob := daemon.eng.Job("build_config")
	buildConfigJob.Stdout.Add(stdoutBuffer)
	buildConfigJob.SetenvList("changes", c.Changes)
	// FIXME this should be remove when we remove deprecated config param
	buildConfigJob.SetenvSubEnv("config", &subenv)

	if err := buildConfigJob.Run(); err != nil {
		return "", err
	}
	if err := json.NewDecoder(stdoutBuffer).Decode(&newConfig); err != nil {
		return "", err
	}

	if err := runconfig.Merge(&newConfig, config); err != nil {
		return "", err
	}

	img, err := daemon.Commit(container, c.Repo, c.Tag, c.Comment, c.Author, c.Pause, &newConfig)
	if err != nil {
		return "", err
	}

	return img.ID, nil
}

// Commit creates a new filesystem image from the current state of a container.
// The image can optionally be tagged into a repository
func (daemon *Daemon) Commit(container *Container, repository, tag, comment, author string, pause bool, config *runconfig.Config) (*image.Image, error) {
	if pause && !container.IsPaused() {
		container.Pause()
		defer container.Unpause()
	}

	if err := container.Mount(); err != nil {
		return nil, err
	}
	defer container.Unmount()

	rwTar, err := container.ExportRw()
	if err != nil {
		return nil, err
	}
	defer rwTar.Close()

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

	img, err := daemon.graph.Create(rwTar, containerID, parentImageID, comment, author, containerConfig, config)
	if err != nil {
		return nil, err
	}

	// Register the image if needed
	if repository != "" {
		if err := daemon.repositories.Tag(repository, tag, img.ID, true); err != nil {
			return img, err
		}
	}
	return img, nil
}
