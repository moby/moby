package daemon

import (
	"io"

	"github.com/docker/docker/engine"
)

// ContainerCopy is deprecated. Use ContainerCopyFrom instead.
func (daemon *Daemon) ContainerCopy(job *engine.Job) engine.Status {
	if len(job.Args) != 2 {
		return job.Errorf("Usage: %s CONTAINER RESOURCE\n", job.Name)
	}

	var (
		name     = job.Args[0]
		resource = job.Args[1]
	)

	if container := daemon.Get(name); container != nil {

		data, err := container.Copy(resource)
		if err != nil {
			return job.Error(err)
		}
		defer data.Close()

		if _, err := io.Copy(job.Stdout, data); err != nil {
			return job.Error(err)
		}
		return engine.StatusOK
	}
	return job.Errorf("No such container: %s", name)
}

// ContainerCopyFrom copies a file or directory from a container to the job's
// standard output in a Tar archive.
func (daemon *Daemon) ContainerCopyFrom(job *engine.Job) engine.Status {
	if len(job.Args) != 2 {
		return job.Errorf("Usage: %s CONTAINER PATH\n", job.Name)
	}

	var (
		name       = job.Args[0]
		sourcePath = job.Args[1]
		container  *Container
	)

	if container = daemon.Get(name); container == nil {
		return job.Errorf("no such container: %s", name)
	}

	data, err := container.CopyFrom(sourcePath)
	if err != nil {
		return job.Error(err)
	}
	defer data.Close()

	if _, err := io.Copy(job.Stdout, data); err != nil {
		return job.Error(err)
	}

	return engine.StatusOK
}

// ContainerCopyTo copies a file or directory from the archive read from the
// job's standard input to a destination path in the specified container.
func (daemon *Daemon) ContainerCopyTo(job *engine.Job) engine.Status {
	if len(job.Args) != 2 {
		return job.Errorf("Usage: %s CONTAINER DSTPATH", job.Name)
	}

	var (
		name      = job.Args[0]
		dstPath   = job.Args[1]
		container *Container
	)

	if container = daemon.Get(name); container == nil {
		return job.Errorf("no such container: %s", name)
	}

	if err := container.CopyTo(job.Stdin, dstPath); err != nil {
		return job.Error(err)
	}

	return engine.StatusOK
}

// ContainerCopyAcross copies a file or directory from a source path in one
// container to a destination path in another container.
func (daemon *Daemon) ContainerCopyAcross(job *engine.Job) engine.Status {
	if len(job.Args) != 4 {
		return job.Errorf("Usage: %s SRCCONTAINER SRCPATH DSTCONTAINER DSTPATH", job.Name)
	}

	var (
		srcName, srcPath           = job.Args[0], job.Args[1]
		dstName, dstPath           = job.Args[2], job.Args[3]
		srcContainer, dstContainer *Container
	)

	if srcContainer = daemon.Get(srcName); srcContainer == nil {
		return job.Errorf("no such container: %s", srcName)
	}

	if dstContainer = daemon.Get(dstName); dstContainer == nil {
		return job.Errorf("no such container: %s", dstName)
	}

	data, err := srcContainer.CopyFrom(srcPath)
	if err != nil {
		return job.Error(err)
	}
	defer data.Close()

	if err = dstContainer.CopyTo(data, dstPath); err != nil {
		return job.Error(err)
	}

	return engine.StatusOK
}
