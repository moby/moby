// Package local provides the default implementation for volumes. It
// is used to mount data volume containers and directories local to
// the host server.
package local // import "github.com/docker/docker/volume/local"

import (
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/docker/docker/errdefs"
	"github.com/pkg/errors"
)

type optsConfig struct{}

// scopedPath verifies that the path where the volume is located
// is under Docker's root and the valid local paths.
func (r *Root) scopedPath(realPath string) bool {
	if strings.HasPrefix(realPath, filepath.Join(r.scope, volumesPathName)) && realPath != filepath.Join(r.scope, volumesPathName) {
		return true
	}
	return false
}

func setOpts(v *localVolume, opts map[string]string) error {
	if len(opts) > 0 {
		return errdefs.InvalidParameter(errors.New("options are not supported on this platform"))
	}
	return nil
}

func (v *localVolume) needsMount() bool {
	return false
}

func (v *localVolume) mount() error {
	return nil
}
func (v *localVolume) unmount() error {
	return nil
}

func unmount(_ string) {}

func (v *localVolume) postMount() error {
	return nil
}

func (v *localVolume) CreatedAt() (time.Time, error) {
	fileInfo, err := os.Stat(v.path)
	if err != nil {
		return time.Time{}, err
	}
	ft := fileInfo.Sys().(*syscall.Win32FileAttributeData).CreationTime
	return time.Unix(0, ft.Nanoseconds()), nil
}
