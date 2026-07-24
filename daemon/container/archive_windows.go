package container

import (
	"os"
	"path/filepath"

	"github.com/moby/go-archive"
	containertypes "github.com/moby/moby/api/types/container"
	"github.com/moby/moby/v2/errdefs"
	"github.com/pkg/errors"
)

// ResolvePath resolves the given path in the container to a resource on the
// host. Returns a resolved path (absolute path to the resource on the host),
// the absolute path to the resource relative to the container's rootfs, and
// an error if the path points to outside the container's rootfs.
func (container *Container) ResolvePath(path string) (resolvedPath, absPath string, _ error) {
	if container.BaseFS == "" {
		return "", "", errors.New("ResolvePath: BaseFS of container " + container.ID + " is unexpectedly empty")
	}
	// Check if a drive letter supplied, it must be the system drive. No-op except on Windows
	path, err := archive.CheckSystemDriveAndRemoveDriveLetter(path)
	if err != nil {
		return "", "", err
	}

	// Consider the given path as an absolute path in the container.
	absPath = archive.PreserveTrailingDotOrSeparator(filepath.Join(string(filepath.Separator), path), path)

	// Split the absPath into its Directory and Base components. We will
	// resolve the dir in the scope of the container then append the base.
	dirPath, basePath := filepath.Split(absPath)

	resolvedDirPath, err := container.GetResourcePath(dirPath)
	if err != nil {
		return "", "", err
	}

	// resolvedDirPath will have been cleaned (no trailing path separators) so
	// we can manually join it with the base path element.
	resolvedPath = resolvedDirPath + string(filepath.Separator) + basePath
	return resolvedPath, absPath, nil
}

// assertsDirectory reports whether the given path explicitly asserts that it is
// a directory (i.e. it ends with a path separator or a "." path component).
func assertsDirectory(path string) bool {
	if len(path) == 0 {
		return false
	}
	if path[len(path)-1] == filepath.Separator {
		return true
	}
	return filepath.Base(path) == "."
}

// StatPath is the unexported version of StatPath. Locks and mounts should
// be acquired before calling this method and the given path should be fully
// resolved to a path on the host corresponding to the given absolute path
// inside the container.
func (container *Container) StatPath(resolvedPath, absPath string) (*containertypes.PathStat, error) {
	if container.BaseFS == "" {
		return nil, errors.New("StatPath: BaseFS of container " + container.ID + " is unexpectedly empty")
	}

	lstat, err := os.Lstat(resolvedPath)
	if err != nil {
		return nil, err
	}

	// A path that asserts a directory (trailing separator or "." component)
	// must resolve to a directory. os.Lstat does not reliably enforce this
	// across Windows storage drivers: a windowsfilter volume mount
	// (\\?\Volume{GUID}\) rejects a trailing separator on a file, but a
	// containerd snapshotter directory mount does not, so the trailing
	// separator is silently ignored and a file is returned. Enforce the
	// invariant explicitly for consistent behavior. (moby/moby#47107)
	if assertsDirectory(absPath) && !lstat.IsDir() {
		return nil, errdefs.InvalidParameter(errors.Errorf("%s: not a directory", absPath))
	}

	var linkTarget string
	if lstat.Mode()&os.ModeSymlink != 0 {
		// Fully evaluate the symlink in the scope of the container rootfs.
		hostPath, err := container.GetResourcePath(absPath)
		if err != nil {
			return nil, err
		}

		linkTarget, err = filepath.Rel(container.BaseFS, hostPath)
		if err != nil {
			return nil, err
		}

		// Make it an absolute path.
		linkTarget = filepath.Join(string(filepath.Separator), linkTarget)
	}

	return &containertypes.PathStat{
		Name:       filepath.Base(absPath),
		Size:       lstat.Size(),
		Mode:       lstat.Mode(),
		Mtime:      lstat.ModTime(),
		LinkTarget: linkTarget,
	}, nil
}
