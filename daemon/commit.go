package daemon

import (
	"bytes"
	"encoding/json"

	"github.com/docker/docker/engine"
	"github.com/docker/docker/image"
	"github.com/docker/docker/runconfig"
)

func (daemon *Daemon) ContainerCommit(job *engine.Job) engine.Status {
	if len(job.Args) != 1 {
		return job.Errorf("Not enough arguments. Usage: %s CONTAINER\n", job.Name)
	}
	name := job.Args[0]

	container, err := daemon.Get(name)
	if err != nil {
		return job.Error(err)
	}

	var (
		config       = container.Config
		stdoutBuffer = bytes.NewBuffer(nil)
		newConfig    runconfig.Config
	)

	buildConfigJob := daemon.eng.Job("build_config")
	buildConfigJob.Stdout.Add(stdoutBuffer)
	buildConfigJob.Setenv("changes", job.Getenv("changes"))
	// FIXME this should be remove when we remove deprecated config param
	buildConfigJob.Setenv("config", job.Getenv("config"))

	if err := buildConfigJob.Run(); err != nil {
		return job.Error(err)
	}
	if err := json.NewDecoder(stdoutBuffer).Decode(&newConfig); err != nil {
		return job.Error(err)
	}

	if err := runconfig.Merge(&newConfig, config); err != nil {
		return job.Error(err)
	}

	img, err := daemon.Commit(container, job.Getenv("repo"), job.Getenv("tag"), job.Getenv("comment"), job.Getenv("author"), job.GetenvBool("pause"), &newConfig)
	if err != nil {
		return job.Error(err)
	}
	job.Printf("%s\n", img.ID)
	return engine.StatusOK
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
		if err := daemon.repositories.Set(repository, tag, img.ID, true); err != nil {
			return img, err
		}
	}
	return img, nil
}
