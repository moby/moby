package container // import "github.com/docker/docker/container"

import (
	"os"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/pkg/archive"
	"github.com/docker/docker/pkg/system"
)

// ResolvePath resolves the given path in the container to a resource on the
// host. Returns a resolved path (absolute path to the resource on the host),
// the absolute path to the resource relative to the container's rootfs, and
// an error if the path points to outside the container's rootfs.
func (container *Container) ResolvePath(path string) (resolvedPath, absPath string, err error) {
	// Check if a drive letter supplied, it must be the system drive. No-op except on Windows
	path, err = system.CheckSystemDriveAndRemoveDriveLetter(path, container.BaseFS)
	if err != nil {
		return "", "", err
	}

	// Consider the given path as an absolute path in the container.
	absPath = archive.PreserveTrailingDotOrSeparator(
		container.BaseFS.Join(string(container.BaseFS.Separator()), path),
		path,
		container.BaseFS.Separator())

	// Split the absPath into its Directory and Base components. We will
	// resolve the dir in the scope of the container then append the base.
	dirPath, basePath := container.BaseFS.Split(absPath)

	resolvedDirPath, err := container.GetResourcePath(dirPath)
	if err != nil {
		return "", "", err
	}

	// resolvedDirPath will have been cleaned (no trailing path separators) so
	// we can manually join it with the base path element.
	resolvedPath = resolvedDirPath + string(container.BaseFS.Separator()) + basePath
	return resolvedPath, absPath, nil
}

// StatPath is the unexported version of StatPath. Locks and mounts should
// be acquired before calling this method and the given path should be fully
// resolved to a path on the host corresponding to the given absolute path
// inside the container.
func (container *Container) StatPath(resolvedPath, absPath string) (stat *types.ContainerPathStat, err error) {
	driver := container.BaseFS

	lstat, err := driver.Lstat(resolvedPath)
	if err != nil {
		return nil, err
	}

	var linkTarget string
	if lstat.Mode()&os.ModeSymlink != 0 {
		// Fully evaluate the symlink in the scope of the container rootfs.
		hostPath, err := container.GetResourcePath(absPath)
		if err != nil {
			return nil, err
		}

		linkTarget, err = driver.Rel(driver.Path(), hostPath)
		if err != nil {
			return nil, err
		}

		// Make it an absolute path.
		linkTarget = driver.Join(string(driver.Separator()), linkTarget)
	}

	return &types.ContainerPathStat{
		Name:       driver.Base(absPath),
		Size:       lstat.Size(),
		Mode:       lstat.Mode(),
		Mtime:      lstat.ModTime(),
		LinkTarget: linkTarget,
	}, nil
}
