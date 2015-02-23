package daemon

import (
	"encoding/json"
	"io"

	"github.com/docker/docker/engine"
)

// ContainerStatPath performs a low-level Lstat operation on file or directory
// in a container and returns the results as a JSON object to the job's stdout.
func (daemon *Daemon) ContainerStatPath(job *engine.Job) engine.Status {
	if len(job.Args) != 2 {
		return job.Errorf("Usage: %s CONTAINER PATH\n", job.Name)
	}

	var (
		name      = job.Args[0]
		path      = job.Args[1]
		container *Container
		err       error
	)

	if container, err = daemon.Get(name); err != nil {
		return job.Error(err)
	}

	stat, err := container.StatPath(path)
	if err != nil {
		return job.Error(err)
	}

	encoder := json.NewEncoder(job.Stdout)
	if err = encoder.Encode(stat); err != nil {
		return job.Error(err)
	}

	return engine.StatusOK
}

// ContainerArchivePath archives a file or directory from
// a container to the job's standard output in a Tar archive.
func (daemon *Daemon) ContainerArchivePath(job *engine.Job) engine.Status {
	if len(job.Args) != 2 {
		return job.Errorf("Usage: %s CONTAINER PATH\n", job.Name)
	}

	var (
		name      = job.Args[0]
		path      = job.Args[1]
		container *Container
		err       error
	)

	if container, err = daemon.Get(name); err != nil {
		return job.Error(err)
	}

	data, err := container.ArchivePath(path)
	if err != nil {
		return job.Error(err)
	}
	defer data.Close()

	if _, err := io.Copy(job.Stdout, data); err != nil {
		return job.Error(err)
	}

	return engine.StatusOK
}

// ContainerExtractToDir extracts a Tar archive
// read from the job's standard input to a
// destination directory in the specified container.
func (daemon *Daemon) ContainerExtractToDir(job *engine.Job) engine.Status {
	if len(job.Args) != 2 {
		return job.Errorf("Usage: %s CONTAINER DSTDIR", job.Name)
	}

	var (
		name      = job.Args[0]
		dstDir    = job.Args[1]
		container *Container
		err       error
	)

	if container, err = daemon.Get(name); err != nil {
		return job.Error(err)
	}

	if err := container.ExtractToDir(job.Stdin, dstDir); err != nil {
		return job.Error(err)
	}

	return engine.StatusOK
}
