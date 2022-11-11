package daemon // import "github.com/docker/docker/daemon"

import (
	"io"
	"os"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/errdefs"
)

// ContainerCopy performs a deprecated operation of archiving the resource at
// the specified path in the container identified by the given name.
func (daemon *Daemon) ContainerCopy(name string, res string) (io.ReadCloser, error) {
	ctr, err := daemon.GetContainer(name)
	if err != nil {
		return nil, err
	}

	data, err := daemon.containerCopy(ctr, res)
	if err == nil {
		return data, nil
	}

	if os.IsNotExist(err) {
		return nil, containerFileNotFound{res, name}
	}
	return nil, errdefs.System(err)
}

// ContainerStatPath stats the filesystem resource at the specified path in the
// container identified by the given name.
func (daemon *Daemon) ContainerStatPath(name string, path string) (stat *types.ContainerPathStat, err error) {
	ctr, err := daemon.GetContainer(name)
	if err != nil {
		return nil, err
	}

	stat, err = daemon.containerStatPath(ctr, path)
	if err == nil {
		return stat, nil
	}

	if os.IsNotExist(err) {
		return nil, containerFileNotFound{path, name}
	}
	return nil, errdefs.System(err)
}

// ContainerArchivePath creates an archive of the filesystem resource at the
// specified path in the container identified by the given name. Returns a
// tar archive of the resource and whether it was a directory or a single file.
func (daemon *Daemon) ContainerArchivePath(name string, path string) (content io.ReadCloser, stat *types.ContainerPathStat, err error) {
	ctr, err := daemon.GetContainer(name)
	if err != nil {
		return nil, nil, err
	}

	content, stat, err = daemon.containerArchivePath(ctr, path)
	if err == nil {
		return content, stat, nil
	}

	if os.IsNotExist(err) {
		return nil, nil, containerFileNotFound{path, name}
	}
	return nil, nil, errdefs.System(err)
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
	if err == nil {
		return nil
	}

	if os.IsNotExist(err) {
		return containerFileNotFound{path, name}
	}
	return errdefs.System(err)
}
