package daemon

import (
	"io"
	"os"

	"github.com/docker/docker/engine"
	"github.com/docker/docker/pkg/archive"
)

// ContainerCopy is deprecated. Use ContainerArchivePath instead.
func (daemon *Daemon) ContainerCopy(job *engine.Job) engine.Status {
	if len(job.Args) != 2 {
		return job.Errorf("Usage: %s CONTAINER RESOURCE\n", job.Name)
	}

	var (
		name     = job.Args[0]
		resource = job.Args[1]
	)

	container, err := daemon.Get(name)
	if err != nil {
		return job.Error(err)
	}

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

// ContainerCopyAcross copies a file or directory from a source
// path in one container to a destination path in another container.
func (daemon *Daemon) ContainerCopyAcross(job *engine.Job) engine.Status {
	if len(job.Args) != 4 {
		return job.Errorf("Usage: %s SRCCONTAINER SRCPATH DSTCONTAINER DSTPATH", job.Name)
	}

	var (
		srcName, srcPath           = job.Args[0], job.Args[1]
		dstName, dstPath           = job.Args[2], job.Args[3]
		srcContainer, dstContainer *Container
		err                        error
	)

	if srcContainer, err = daemon.Get(srcName); err != nil {
		return job.Error(err)
	}

	if dstContainer, err = daemon.Get(dstName); err != nil {
		return job.Error(err)
	}

	// Get archive.CopyInfo for source path.
	srcStat, err := srcContainer.StatPath(srcPath)
	if err != nil {
		return job.Error(err)
	}

	srcInfo := archive.CopyInfo{
		Path:   srcStat.AbsPath,
		Exists: true,
		IsDir:  srcStat.Mode.IsDir(),
	}

	// Get archive.CopyInfo for destination path.
	dstInfo := archive.CopyInfo{Path: dstPath}

	// If the destination doesn't exist, we'll try the parent directory.
	dstStat, err := dstContainer.StatPath(dstPath)
	if err == nil {
		dstInfo = archive.CopyInfo{
			Path:   dstStat.AbsPath,
			Exists: true,
			IsDir:  dstStat.Mode.IsDir(),
		}
	} else if !os.IsNotExist(err) {
		return job.Error(err)
	}

	// Get the resource archive from the source container.
	srcContent, err := srcContainer.ArchivePath(srcPath)
	if err != nil {
		return job.Error(err)
	}
	defer srcContent.Close()

	// Prepare the content archive to be copied to the destination.
	dstDir, copyArchive, err := archive.PrepareArchiveCopy(srcContent, srcInfo, dstInfo)
	if err != nil {
		return job.Error(err)
	}
	defer copyArchive.Close()

	// Finally, extract it to the destination
	// directory in the destination container.
	if err = dstContainer.ExtractToDir(copyArchive, dstDir); err != nil {
		return job.Error(err)
	}

	return engine.StatusOK
}
