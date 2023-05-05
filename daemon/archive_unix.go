//go:build !windows

package daemon // import "github.com/docker/docker/daemon"

import (
	"context"
	"io"
	"os"
	"path/filepath"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/container"
	"github.com/docker/docker/errdefs"
	"github.com/docker/docker/pkg/archive"
	"github.com/docker/docker/pkg/ioutils"
	volumemounts "github.com/docker/docker/volume/mounts"
	"github.com/pkg/errors"
)

// containerStatPath stats the filesystem resource at the specified path in this
// container. Returns stat info about the resource.
func (daemon *Daemon) containerStatPath(container *container.Container, path string) (stat *types.ContainerPathStat, err error) {
	container.Lock()
	defer container.Unlock()

	cfs, err := daemon.openContainerFS(container)
	if err != nil {
		return nil, err
	}
	defer cfs.Close()

	return cfs.Stat(context.TODO(), path)
}

// containerArchivePath creates an archive of the filesystem resource at the specified
// path in this container. Returns a tar archive of the resource and stat info
// about the resource.
func (daemon *Daemon) containerArchivePath(container *container.Container, path string) (content io.ReadCloser, stat *types.ContainerPathStat, err error) {
	container.Lock()

	defer func() {
		if err != nil {
			// Wait to unlock the container until the archive is fully read
			// (see the ReadCloseWrapper func below) or if there is an error
			// before that occurs.
			container.Unlock()
		}
	}()

	cfs, err := daemon.openContainerFS(container)
	if err != nil {
		return nil, nil, err
	}

	defer func() {
		if err != nil {
			cfs.Close()
		}
	}()

	absPath := archive.PreserveTrailingDotOrSeparator(filepath.Join("/", path), path)

	stat, err = cfs.Stat(context.TODO(), absPath)
	if err != nil {
		return nil, nil, err
	}

	sourceDir, sourceBase := absPath, "."
	if stat.Mode&os.ModeDir == 0 { // not dir
		sourceDir, sourceBase = filepath.Split(absPath)
	}
	opts := archive.TarResourceRebaseOpts(sourceBase, filepath.Base(absPath))

	tb, err := archive.NewTarballer(sourceDir, opts)
	if err != nil {
		return nil, nil, err
	}

	cfs.GoInFS(context.TODO(), tb.Do)
	data := tb.Reader()
	content = ioutils.NewReadCloserWrapper(data, func() error {
		err := data.Close()
		_ = cfs.Close()
		container.Unlock()
		return err
	})

	daemon.LogContainerEvent(container, "archive-path")

	return content, stat, nil
}

// containerExtractToDir extracts the given tar archive to the specified location in the
// filesystem of this container. The given path must be of a directory in the
// container. If it is not, the error will be an errdefs.InvalidParameter. If
// noOverwriteDirNonDir is true then it will be an error if unpacking the
// given content would cause an existing directory to be replaced with a non-
// directory and vice versa.
func (daemon *Daemon) containerExtractToDir(container *container.Container, path string, copyUIDGID, noOverwriteDirNonDir bool, content io.Reader) (err error) {
	container.Lock()
	defer container.Unlock()

	cfs, err := daemon.openContainerFS(container)
	if err != nil {
		return err
	}
	defer cfs.Close()

	err = cfs.RunInFS(context.TODO(), func() error {
		// The destination path needs to be resolved with all symbolic links
		// followed. Note that we need to also evaluate the last path element if
		// it is a symlink. This is so that you can extract an archive to a
		// symlink that points to a directory.
		absPath, err := filepath.EvalSymlinks(filepath.Join("/", path))
		if err != nil {
			return err
		}
		absPath = archive.PreserveTrailingDotOrSeparator(absPath, path)

		stat, err := os.Lstat(absPath)
		if err != nil {
			return err
		}
		if !stat.IsDir() {
			return errdefs.InvalidParameter(errors.New("extraction point is not a directory"))
		}

		// Need to check if the path is in a volume. If it is, it cannot be in a
		// read-only volume. If it is not in a volume, the container cannot be
		// configured with a read-only rootfs.
		toVolume, err := checkIfPathIsInAVolume(container, absPath)
		if err != nil {
			return err
		}

		if !toVolume && container.HostConfig.ReadonlyRootfs {
			return errdefs.InvalidParameter(errors.New("container rootfs is marked read-only"))
		}

		options := daemon.defaultTarCopyOptions(noOverwriteDirNonDir)

		if copyUIDGID {
			var err error
			// tarCopyOptions will appropriately pull in the right uid/gid for the
			// user/group and will set the options.
			options, err = daemon.tarCopyOptions(container, noOverwriteDirNonDir)
			if err != nil {
				return err
			}
		}

		return archive.Untar(content, absPath, options)
	})
	if err != nil {
		return err
	}

	daemon.LogContainerEvent(container, "extract-to-dir")

	return nil
}

func (daemon *Daemon) containerCopy(container *container.Container, resource string) (rc io.ReadCloser, err error) {
	container.Lock()

	defer func() {
		if err != nil {
			// Wait to unlock the container until the archive is fully read
			// (see the ReadCloseWrapper func below) or if there is an error
			// before that occurs.
			container.Unlock()
		}
	}()

	cfs, err := daemon.openContainerFS(container)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err != nil {
			cfs.Close()
		}
	}()

	err = cfs.RunInFS(context.TODO(), func() error {
		_, err := os.Stat(resource)
		return err
	})
	if err != nil {
		return nil, err
	}

	tb, err := archive.NewTarballer(resource, &archive.TarOptions{
		Compression: archive.Uncompressed,
	})
	if err != nil {
		return nil, err
	}

	cfs.GoInFS(context.TODO(), tb.Do)
	archv := tb.Reader()
	reader := ioutils.NewReadCloserWrapper(archv, func() error {
		err := archv.Close()
		_ = cfs.Close()
		container.Unlock()
		return err
	})
	daemon.LogContainerEvent(container, "copy")
	return reader, nil
}

// checkIfPathIsInAVolume checks if the path is in a volume. If it is, it
// cannot be in a read-only volume. If it  is not in a volume, the container
// cannot be configured with a read-only rootfs.
func checkIfPathIsInAVolume(container *container.Container, absPath string) (bool, error) {
	var toVolume bool
	parser := volumemounts.NewParser()
	for _, mnt := range container.MountPoints {
		if toVolume = parser.HasResource(mnt, absPath); toVolume {
			if mnt.RW {
				break
			}
			return false, errdefs.InvalidParameter(errors.New("mounted volume is marked read-only"))
		}
	}
	return toVolume, nil
}
