//go:build windows

package cim

import (
	"fmt"

	"github.com/Microsoft/hcsshim/internal/winapi"
	"github.com/Microsoft/hcsshim/osversion"
	"github.com/pkg/errors"
)

// mergeHive merges the hive located at parentHivePath with the hive located at deltaHivePath and stores
// the result into the file at mergedHivePath. If a file already exists at path `mergedHivePath` then it
// throws an error.
func mergeHive(parentHivePath, deltaHivePath, mergedHivePath string) (err error) {
	var baseHive, deltaHive, mergedHive winapi.ORHKey
	if err := winapi.OROpenHive(parentHivePath, &baseHive); err != nil {
		return fmt.Errorf("failed to open base hive %s: %w", parentHivePath, err)
	}
	defer func() {
		err2 := winapi.ORCloseHive(baseHive)
		if err == nil {
			err = errors.Wrap(err2, "failed to close base hive")
		}
	}()
	if err := winapi.OROpenHive(deltaHivePath, &deltaHive); err != nil {
		return fmt.Errorf("failed to open delta hive %s: %w", deltaHivePath, err)
	}
	defer func() {
		err2 := winapi.ORCloseHive(deltaHive)
		if err == nil {
			err = errors.Wrap(err2, "failed to close delta hive")
		}
	}()
	if err := winapi.ORMergeHives([]winapi.ORHKey{baseHive, deltaHive}, &mergedHive); err != nil {
		return fmt.Errorf("failed to merge hives: %w", err)
	}
	defer func() {
		err2 := winapi.ORCloseHive(mergedHive)
		if err == nil {
			err = errors.Wrap(err2, "failed to close merged hive")
		}
	}()
	if err := winapi.ORSaveHive(mergedHive, mergedHivePath, uint32(osversion.Get().MajorVersion), uint32(osversion.Get().MinorVersion)); err != nil {
		return fmt.Errorf("failed to save hive: %w", err)
	}
	return
}
