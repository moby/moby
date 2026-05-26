//go:build !linux && !freebsd && !windows

package local

import (
	"os"
	"time"

	"github.com/moby/moby/v2/errdefs"
)

type optsConfig struct{}

func (r *Root) validateOpts(opts map[string]string) error {
	if len(opts) == 0 {
		return nil
	}
	return errdefs.InvalidParameter(errdefs.PlatformNotImplemented{Feature: "local volume options"})
}

func (v *localVolume) setOpts(map[string]string) error {
	return nil
}

func (v *localVolume) needsMount() bool { return false }

func (v *localVolume) mount() error { return nil }

func (v *localVolume) unmount() error { return nil }

func (v *localVolume) postMount() error { return nil }

func (v *localVolume) restoreIfMounted() error { return nil }

func (v *localVolume) CreatedAt() (time.Time, error) {
	fileInfo, err := os.Stat(v.rootPath)
	if err != nil {
		return time.Time{}, err
	}
	return fileInfo.ModTime(), nil
}
