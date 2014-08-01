package daemon

import (
	"github.com/docker/docker/engine"
	"github.com/docker/docker/image"
	"github.com/docker/docker/runconfig"
)

func (daemon *Daemon) ContainerCommit(job *engine.Job) engine.Status {
	if len(job.Args) != 1 {
		return job.Errorf("Not enough arguments. Usage: %s CONTAINER\n", job.Name)
	}
	name := job.Args[0]

	container := daemon.Get(name)
	if container == nil {
		return job.Errorf("No such container: %s", name)
	}

	var (
		config    = container.Config
		newConfig runconfig.Config
	)

	if err := job.GetenvJson("config", &newConfig); err != nil {
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
	if pause {
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
		containerID, containerImage string
		containerConfig             *runconfig.Config
	)

	if container != nil {
		containerID = container.ID
		containerImage = container.Image
		containerConfig = container.Config
	}

	img, err := daemon.graph.Create(rwTar, containerID, containerImage, comment, author, containerConfig, config)
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
