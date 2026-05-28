package mounts

import (
	"fmt"

	"github.com/moby/moby/api/types/mount"
	"github.com/pkg/errors"
)

type errMountConfig struct {
	mount *mount.Mount
	err   error
}

func (e *errMountConfig) Error() string {
	return fmt.Sprintf("invalid mount config for type %q: %v", e.mount.Type, e.err.Error())
}

func errBindSourceDoesNotExist(path string) error {
	return errors.Errorf("bind source path does not exist: %s", path)
}

func errExtraField(name string) error {
	return errors.Errorf("field %s must not be specified", name)
}

func errMissingField(name string) error {
	return errors.Errorf("field %s must not be empty", name)
}

// validateExclusiveOptions checks if the given mount config only contains
// options for the given mount-type.
func validateExclusiveOptions(mnt *mount.Mount) error {
	if mnt.Type != mount.TypeBind && mnt.BindOptions != nil {
		return errExtraField("BindOptions")
	}
	if mnt.Type != mount.TypeVolume && mnt.VolumeOptions != nil {
		return errExtraField("VolumeOptions")
	}
	if mnt.Type != mount.TypeImage && mnt.ImageOptions != nil {
		return errExtraField("ImageOptions")
	}
	if mnt.Type != mount.TypeTmpfs && mnt.TmpfsOptions != nil {
		return errExtraField("TmpfsOptions")
	}
	if mnt.Type != mount.TypeCluster && mnt.ClusterOptions != nil {
		return errExtraField("ClusterOptions")
	}
	return nil
}
