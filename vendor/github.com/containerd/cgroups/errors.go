package cgroups

import (
	"errors"
	"os"
)

var (
	ErrInvalidPid               = errors.New("cgroups: pid must be greater than 0")
	ErrMountPointNotExist       = errors.New("cgroups: cgroup mountpoint does not exist")
	ErrInvalidFormat            = errors.New("cgroups: parsing file with invalid format failed")
	ErrFreezerNotSupported      = errors.New("cgroups: freezer cgroup not supported on this system")
	ErrMemoryNotSupported       = errors.New("cgroups: memory cgroup not supported on this system")
	ErrCgroupDeleted            = errors.New("cgroups: cgroup deleted")
	ErrNoCgroupMountDestination = errors.New("cgroups: cannot found cgroup mount destination")
)

// ErrorHandler is a function that handles and acts on errors
type ErrorHandler func(err error) error

// IgnoreNotExist ignores any errors that are for not existing files
func IgnoreNotExist(err error) error {
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

func errPassthrough(err error) error {
	return err
}
