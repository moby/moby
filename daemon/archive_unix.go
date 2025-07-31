//go:build !windows

package daemon

import (
	"context"
	"io"
	"os"
	"path/filepath"

	"github.com/moby/go-archive"
	containertypes "github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/events"
	"github.com/moby/moby/v2/daemon/container"
	volumemounts "github.com/moby/moby/v2/daemon/volume/mounts"
	"github.com/moby/moby/v2/errdefs"
	"github.com/moby/moby/v2/pkg/ioutils"
	"github.com/pkg/errors"
)

// containerStatPath stats the filesystem resource at the specified path in this
// container. Returns stat info about the resource.
func (daemon *Daemon) containerStatPath(container *container.Container, path string) (*containertypes.PathStat, error) {
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
func (daemon *Daemon) containerArchivePath(container *container.Container, path string) (content io.ReadCloser, stat *containertypes.PathStat, retErr error) {
	container.Lock()

	defer func() {
		if retErr != nil {
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
		if retErr != nil {
			_ = cfs.Close()
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

	daemon.LogContainerEvent(container, events.ActionArchivePath)

	return content, stat, nil
}

// containerExtractToDir extracts the given tar archive to the specified location in the
// filesystem of this container. The given path must be of a directory in the
// container. If it is not, the error will be an errdefs.InvalidParameter. If
// noOverwriteDirNonDir is true then it will be an error if unpacking the
// given content would cause an existing directory to be replaced with a non-
// directory and vice versa.
func (daemon *Daemon) containerExtractToDir(container *container.Container, path string, copyUIDGID, allowOverwriteDirWithFile bool, content io.Reader) error {
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

		// Check that the destination is not a read-only filesystem.
		if err := checkWritablePath(container, absPath); err != nil {
			return err
		}

		options := daemon.defaultTarCopyOptions(allowOverwriteDirWithFile)

		if copyUIDGID {
			var err error
			// tarCopyOptions will appropriately pull in the right uid/gid for the
			// user/group and will set the options.
			options, err = daemon.tarCopyOptions(container, allowOverwriteDirWithFile)
			if err != nil {
				return err
			}
		}

		return archive.Untar(content, absPath, options)
	})
	if err != nil {
		return err
	}

	daemon.LogContainerEvent(container, events.ActionExtractToDir)

	return nil
}

// checkWritablePath checks if the path is in a writable location inside the
// container. If the path is within a location mounted from a volume, it checks
// if the volume is mounted read-only. If it is not in a volume, it checks whether
// the container's rootfs is mounted read-only.
func checkWritablePath(ctr *container.Container, absPath string) error {
	parser := volumemounts.NewParser()
	for _, mnt := range ctr.MountPoints {
		if isVolumePath := parser.HasResource(mnt, absPath); isVolumePath {
			if mnt.RW {
				return nil
			}
			return errdefs.InvalidParameter(errors.New("mounted volume is marked read-only"))
		}
	}
	if ctr.HostConfig.ReadonlyRootfs {
		return errdefs.InvalidParameter(errors.New("container rootfs is marked read-only"))
	}
	return nil
}
