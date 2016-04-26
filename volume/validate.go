package volume

import (
	"fmt"
	"os"

	"github.com/docker/engine-api/types/container"
)

func validateMountConfig(mnt *container.MountConfig) error {
	if len(mnt.Destination) == 0 {
		return fmt.Errorf("Invalid mount spec: Destination must not be empty")
	}

	if len(mnt.Mode) > 0 {
		if !ValidMountMode(mnt.Mode) {
			return fmt.Errorf("Invalid mount spec: Mode %q is invalid", mnt.Mode)
		}
	}
	switch mnt.Type {
	case MountTypeEphemeral:
		if len(mnt.Source) > 0 {
			return fmt.Errorf("Invalid ephemeral mount spec: Source must not be specified")
		}
		if len(mnt.Name) > 0 {
			return fmt.Errorf("Invalid ephemeral mount spec: Name must not be specified")
		}
	case MountTypeHostBind:
		if len(mnt.Name) > 0 {
			return fmt.Errorf("Invalid hostbind mount spec: Name must not be specified")
		}
		if len(mnt.Source) == 0 {
			return fmt.Errorf("Invalid hostbind mount spec: Source must not be empty")
		}

		if len(mnt.Driver) > 0 {
			return fmt.Errorf("Invalid hostbind mount spec: Driver must not be specified")
		}
		// Do not allow binding to non-existent path
		if _, err := os.Stat(mnt.Source); err != nil {
			if os.IsNotExist(err) {
				return fmt.Errorf("Invalid hostbind mount spec for Source: path does not exist")
			}
			return fmt.Errorf("Invalid hostbind mount spec for Source %q: %v", mnt.Source, err)
		}
	case MountTypePersistent:
		if len(mnt.Name) == 0 {
			return fmt.Errorf("Invalid hostbind mount spec: Name must not be empty")
		}
		if len(mnt.Source) > 0 {
			return fmt.Errorf("Invalid ephemeral mount spec: Source must not be specified")
		}
	default:
		return fmt.Errorf("Invalid mount spec: mount type unknown: %q", mnt.Type)
	}
	return nil
}
