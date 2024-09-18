package safepath

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	"github.com/containerd/log"
	"github.com/docker/docker/internal/cleanups"
	"github.com/pkg/errors"
	"golang.org/x/sys/windows"
)

// Join locks all individual components of the path which is the concatenation
// of provided path and its subpath, checks that it doesn't escape the base path
// and returns the concatenated path.
//
// The path is safe (the path target won't change) until the returned SafePath
// is Closed.
// Caller is responsible for calling the Close function which unlocks the path.
func Join(ctx context.Context, path, subpath string) (*SafePath, error) {
	base, subpart, err := evaluatePath(path, subpath)
	if err != nil {
		return nil, err
	}
	parts := strings.Split(subpart, string(os.PathSeparator))

	cleanupJobs := cleanups.Composite{}
	defer func() {
		if cErr := cleanupJobs.Call(context.WithoutCancel(ctx)); cErr != nil {
			log.G(ctx).WithError(cErr).Warn("failed to close handles after error")
		}
	}()

	fullPath := base
	for _, part := range parts {
		fullPath = filepath.Join(fullPath, part)

		handle, err := lockFile(fullPath)
		if err != nil {
			if errors.Is(err, windows.ERROR_FILE_NOT_FOUND) {
				return nil, &ErrNotAccessible{Path: fullPath, Cause: err}
			}
			return nil, errors.Wrapf(err, "failed to lock file %s", fullPath)
		}
		cleanupJobs.Add(func(context.Context) error {
			if err := windows.CloseHandle(handle); err != nil {
				return &os.PathError{Op: "CloseHandle", Path: fullPath, Err: err}
			}
			return err
		})

		realPath, err := filepath.EvalSymlinks(fullPath)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to eval symlinks of %s", fullPath)
		}

		if realPath != fullPath && !isLocalTo(realPath, base) {
			return nil, &ErrEscapesBase{Base: base, Subpath: subpart}
		}

		var info windows.ByHandleFileInformation
		if err := windows.GetFileInformationByHandle(handle, &info); err != nil {
			return nil, errors.WithStack(&os.PathError{Op: "GetFileInformationByHandle", Path: fullPath, Err: err})
		}

		if (info.FileAttributes & windows.FILE_ATTRIBUTE_REPARSE_POINT) != 0 {
			return nil, &ErrNotAccessible{Path: fullPath, Cause: err}
		}
	}

	return &SafePath{
		path:          fullPath,
		sourceBase:    base,
		sourceSubpath: subpart,
		cleanup:       cleanupJobs.Release(),
	}, nil
}

func lockFile(path string) (windows.Handle, error) {
	p, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return windows.InvalidHandle, &os.PathError{Op: "UTF16PtrFromString", Path: path, Err: err}
	}
	const flags = windows.FILE_FLAG_BACKUP_SEMANTICS | windows.FILE_FLAG_OPEN_REPARSE_POINT
	handle, err := windows.CreateFile(p, windows.GENERIC_READ, windows.FILE_SHARE_READ, nil, windows.OPEN_EXISTING, flags, 0)
	if err != nil {
		return handle, &os.PathError{Op: "CreateFile", Path: path, Err: err}
	}
	return handle, nil
}
