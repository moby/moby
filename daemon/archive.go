package daemon // import "github.com/docker/docker/daemon"

import (
	"io"
	"os"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/errdefs"
)

// ContainerStatPath stats the filesystem resource at the specified path in the
// container identified by the given name.
func (daemon *Daemon) ContainerStatPath(name string, path string) (stat *container.PathStat, err error) {
	ctr, err := daemon.GetContainer(name)
	if err != nil {
		return nil, err
	}

	stat, err = daemon.containerStatPath(ctr, path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, containerFileNotFound{path, name}
		}
		// TODO(thaJeztah): check if daemon.containerStatPath returns any errors that are not typed; if not, then return as-is
		if errdefs.IsInvalidParameter(err) {
			return nil, err
		}
		return nil, errdefs.System(err)
	}
	return stat, nil
}

// ContainerArchivePath creates an archive of the filesystem resource at the
// specified path in the container identified by the given name. Returns a
// tar archive of the resource and whether it was a directory or a single file.
func (daemon *Daemon) ContainerArchivePath(name string, path string) (content io.ReadCloser, stat *container.PathStat, err error) {
	ctr, err := daemon.GetContainer(name)
	if err != nil {
		return nil, nil, err
	}

	content, stat, err = daemon.containerArchivePath(ctr, path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil, containerFileNotFound{path, name}
		}
		// TODO(thaJeztah): check if daemon.containerArchivePath returns any errors that are not typed; if not, then return as-is
		if errdefs.IsInvalidParameter(err) {
			return nil, nil, err
		}
		return nil, nil, errdefs.System(err)
	}
	return content, stat, nil
}

// ContainerExtractToDir extracts the given archive to the specified location
// in the filesystem of the container identified by the given name. The given
// path must be of a directory in the container. If it is not, the error will
// be an errdefs.InvalidParameter. If noOverwriteDirNonDir is true then it will
// be an error if unpacking the given content would cause an existing directory
// to be replaced with a non-directory and vice versa.
func (daemon *Daemon) ContainerExtractToDir(name, path string, copyUIDGID, noOverwriteDirNonDir bool, content io.Reader) error {
	ctr, err := daemon.GetContainer(name)
	if err != nil {
		return err
	}

	err = daemon.containerExtractToDir(ctr, path, copyUIDGID, noOverwriteDirNonDir, content)
	if err != nil {
		if os.IsNotExist(err) {
			return containerFileNotFound{path, name}
		}
		// TODO(thaJeztah): check if daemon.containerExtractToDir returns any errors that are not typed; if not, then return as-is
		if errdefs.IsInvalidParameter(err) {
			return err
		}
		return errdefs.System(err)
	}
	return nil
}
