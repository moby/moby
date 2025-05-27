//go:build windows
// +build windows

package cimfs

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/Microsoft/go-winio/pkg/guid"
	"github.com/Microsoft/hcsshim/internal/winapi"
	"github.com/pkg/errors"
)

type MountError struct {
	Cim        string
	Op         string
	VolumeGUID guid.GUID
	Err        error
}

func (e *MountError) Error() string {
	s := "cim " + e.Op
	if e.Cim != "" {
		s += " " + e.Cim
	}
	s += " " + e.VolumeGUID.String() + ": " + e.Err.Error()
	return s
}

// Mount mounts the given cim at a volume with given GUID. Returns the full volume
// path if mount is successful.
func Mount(cimPath string, volumeGUID guid.GUID, mountFlags uint32) (string, error) {
	if err := winapi.CimMountImage(filepath.Dir(cimPath), filepath.Base(cimPath), mountFlags, &volumeGUID); err != nil {
		return "", &MountError{Cim: cimPath, Op: "Mount", VolumeGUID: volumeGUID, Err: err}
	}
	return fmt.Sprintf("\\\\?\\Volume{%s}\\", volumeGUID.String()), nil
}

// Unmount unmounts the cim at mounted at path `volumePath`.
func Unmount(volumePath string) error {
	// The path is expected to be in the \\?\Volume{GUID}\ format
	if volumePath[len(volumePath)-1] != '\\' {
		volumePath += "\\"
	}

	if !(strings.HasPrefix(volumePath, "\\\\?\\Volume{") && strings.HasSuffix(volumePath, "}\\")) {
		return errors.Errorf("volume path %s is not in the expected format", volumePath)
	}

	trimmedStr := strings.TrimPrefix(volumePath, "\\\\?\\Volume{")
	trimmedStr = strings.TrimSuffix(trimmedStr, "}\\")

	volGUID, err := guid.FromString(trimmedStr)
	if err != nil {
		return errors.Wrapf(err, "guid parsing failed for %s", trimmedStr)
	}

	if err := winapi.CimDismountImage(&volGUID); err != nil {
		return &MountError{VolumeGUID: volGUID, Op: "Unmount", Err: err}
	}

	return nil
}
