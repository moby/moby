//go:build windows
// +build windows

package cimfs

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Microsoft/go-winio/pkg/guid"
	"github.com/Microsoft/hcsshim/internal/winapi"
	winapitypes "github.com/Microsoft/hcsshim/internal/winapi/types"
	"github.com/pkg/errors"
	"golang.org/x/sys/windows"
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

const (
	VolumePathFormat = "\\\\?\\Volume{%s}\\"
)

// Mount mounts the given cim at a volume with given GUID. Returns the full volume
// path if mount is successful.
func Mount(cimPath string, volumeGUID guid.GUID, mountFlags uint32) (string, error) {
	if err := winapi.CimMountImage(filepath.Dir(cimPath), filepath.Base(cimPath), mountFlags, &volumeGUID); err != nil {
		return "", &MountError{Cim: cimPath, Op: "Mount", VolumeGUID: volumeGUID, Err: err}
	}
	return fmt.Sprintf(VolumePathFormat, volumeGUID.String()), nil
}

// Unmount unmounts the cim at mounted at path `volumePath`.
func Unmount(volumePath string) error {
	// The path is expected to be in the \\?\Volume{GUID}\ format
	if volumePath[len(volumePath)-1] != '\\' {
		volumePath += "\\"
	}

	if !strings.HasPrefix(volumePath, "\\\\?\\Volume{") || !strings.HasSuffix(volumePath, "}\\") {
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

// MountMergedBlockCIMs mounts the given merged BlockCIM (usually created with
// `MergeBlockCIMs`) at a volume with given GUID. The `sourceCIMs` MUST be identical
// to the `sourceCIMs` passed to `MergeBlockCIMs` when creating this merged CIM.
func MountMergedBlockCIMs(mergedCIM *BlockCIM, sourceCIMs []*BlockCIM, mountFlags uint32, volumeGUID guid.GUID) (string, error) {
	if !IsMergedCimMountSupported() {
		return "", fmt.Errorf("merged CIMs aren't supported on this OS version")
	} else if len(sourceCIMs) < 2 {
		return "", fmt.Errorf("need at least 2 source CIMs, got %d: %w", len(sourceCIMs), os.ErrInvalid)
	}

	switch mergedCIM.Type {
	case BlockCIMTypeDevice:
		mountFlags |= CimMountBlockDeviceCim
	case BlockCIMTypeSingleFile:
		mountFlags |= CimMountSingleFileCim
	default:
		return "", fmt.Errorf("invalid block CIM type `%d`", mergedCIM.Type)
	}

	for _, sCIM := range sourceCIMs {
		if sCIM.Type != mergedCIM.Type {
			return "", fmt.Errorf("source CIM (%s) type doesn't match with merged CIM type: %w", sCIM.String(), os.ErrInvalid)
		}
	}

	// win32 mount merged CIM API expects an array of all CIMs. 0th entry in the array
	// should be the merged CIM. All remaining entries should be the source CIM paths
	// in the same order that was used while creating the merged CIM.
	allcims := append([]*BlockCIM{mergedCIM}, sourceCIMs...)
	cimsToMerge := []winapitypes.CimFsImagePath{}
	for _, bcim := range allcims {
		// Trailing backslashes cause problems-remove those
		imageDir, err := windows.UTF16PtrFromString(strings.TrimRight(bcim.BlockPath, `\`))
		if err != nil {
			return "", fmt.Errorf("convert string to utf16: %w", err)
		}
		cimName, err := windows.UTF16PtrFromString(bcim.CimName)
		if err != nil {
			return "", fmt.Errorf("convert string to utf16: %w", err)
		}

		cimsToMerge = append(cimsToMerge, winapitypes.CimFsImagePath{
			ImageDir:  imageDir,
			ImageName: cimName,
		})
	}

	if err := winapi.CimMergeMountImage(uint32(len(cimsToMerge)), &cimsToMerge[0], mountFlags, &volumeGUID); err != nil {
		return "", &MountError{Cim: filepath.Join(mergedCIM.BlockPath, mergedCIM.CimName), Op: "MountMerged", Err: err}
	}
	return fmt.Sprintf(VolumePathFormat, volumeGUID.String()), nil
}

// Mounts a verified block CIM with the provided root hash. The root hash is usually
// returned when the CIM is sealed or the root hash can be queried from a block CIM.
// Every read on the mounted volume will be verified to match against the provided root
// hash if it doesn't, the read will fail.  The CIM MUST have been created with the
// verified creation flag.
func MountVerifiedBlockCIM(bCIM *BlockCIM, mountFlags uint32, volumeGUID guid.GUID, rootHash []byte) (string, error) {
	if len(rootHash) != cimHashSize {
		return "", fmt.Errorf("unexpected root hash size %d, expected size is %d", len(rootHash), cimHashSize)
	}

	// The CimMountVerifiedCim flag should only be used when using the regular mount
	// CIM API. That flag is required to tell that API that this is a verified
	// CIM. This API doesn't need that flag as it is already assumed that the CIM is
	// verified.
	switch bCIM.Type {
	case BlockCIMTypeDevice:
		mountFlags |= CimMountBlockDeviceCim
	case BlockCIMTypeSingleFile:
		mountFlags |= CimMountSingleFileCim
	default:
		return "", fmt.Errorf("invalid block CIM type `%d`: %w", bCIM.Type, os.ErrInvalid)
	}

	if err := winapi.CimMountVerifiedImage(bCIM.BlockPath, bCIM.CimName, mountFlags, &volumeGUID, cimHashSize, &rootHash[0]); err != nil {
		return "", &MountError{Cim: bCIM.String(), Op: "MountVerifiedCIM", Err: err}
	}
	return fmt.Sprintf("\\\\?\\Volume{%s}\\", volumeGUID.String()), nil
}

// MountMergedVerifiedBlockCIMs mounts the given merged verified BlockCIM (usually created
// with `MergeBlockCIMs`) at a volume with given GUID, with the given root hash. The
// `sourceCIMs` MUST be identical to the `sourceCIMs` passed to `MergeBlockCIMs` when
// creating this merged CIM. The root hash is usually returned when the CIM is sealed or
// the root hash can be queried from a block CIM. In case of merged CIMs, the root hash of
// the merged CIM should be passed here. Every read on the mounted volume will be verified
// to match against the provided root hash if it doesn't, the read will fail.  The source
// CIMs and the merged CIM MUST have been created with the verified creation flag.
func MountMergedVerifiedBlockCIMs(mergedCIM *BlockCIM, sourceCIMs []*BlockCIM, mountFlags uint32, volumeGUID guid.GUID, rootHash []byte) (string, error) {
	if !IsVerifiedCimMountSupported() {
		return "", fmt.Errorf("verified CIMs aren't supported on this OS version")
	} else if len(sourceCIMs) < 2 {
		return "", fmt.Errorf("need at least 2 source CIMs, got %d: %w", len(sourceCIMs), os.ErrInvalid)
	} else if len(rootHash) != cimHashSize {
		return "", fmt.Errorf("unexpected root hash size %d, expected size is %d", len(rootHash), cimHashSize)
	}

	switch mergedCIM.Type {
	case BlockCIMTypeDevice:
		mountFlags |= CimMountBlockDeviceCim
	case BlockCIMTypeSingleFile:
		mountFlags |= CimMountSingleFileCim
	default:
		return "", fmt.Errorf("invalid block CIM type `%d`", mergedCIM.Type)
	}

	for _, sCIM := range sourceCIMs {
		if sCIM.Type != mergedCIM.Type {
			return "", fmt.Errorf("source CIM (%s) type doesn't match with merged CIM type: %w", sCIM.String(), os.ErrInvalid)
		}
	}

	// win32 mount merged CIM API expects an array of all CIMs. 0th entry in the array
	// should be the merged CIM. All remaining entries should be the source CIM paths
	// in the same order that was used while creating the merged CIM.
	allcims := append([]*BlockCIM{mergedCIM}, sourceCIMs...)
	cimsToMerge := []winapitypes.CimFsImagePath{}
	for _, bcim := range allcims {
		// Trailing backslashes cause problems-remove those
		imageDir, err := windows.UTF16PtrFromString(strings.TrimRight(bcim.BlockPath, `\`))
		if err != nil {
			return "", fmt.Errorf("convert string to utf16: %w", err)
		}
		cimName, err := windows.UTF16PtrFromString(bcim.CimName)
		if err != nil {
			return "", fmt.Errorf("convert string to utf16: %w", err)
		}

		cimsToMerge = append(cimsToMerge, winapitypes.CimFsImagePath{
			ImageDir:  imageDir,
			ImageName: cimName,
		})
	}

	if err := winapi.CimMergeMountVerifiedImage(uint32(len(cimsToMerge)), &cimsToMerge[0], mountFlags, &volumeGUID, cimHashSize, &rootHash[0]); err != nil {
		return "", &MountError{Cim: filepath.Join(mergedCIM.BlockPath, mergedCIM.CimName), Op: "MountMergedVerified", Err: err}
	}
	return fmt.Sprintf(VolumePathFormat, volumeGUID.String()), nil
}
