//go:build windows

package cim

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Microsoft/go-winio/pkg/guid"
	"github.com/Microsoft/hcsshim/internal/log"
	"github.com/Microsoft/hcsshim/internal/oc"
	cimfs "github.com/Microsoft/hcsshim/pkg/cimfs"
	"github.com/sirupsen/logrus"
	"go.opencensus.io/trace"
)

var cimMountNamespace guid.GUID = guid.GUID{Data1: 0x6827367b, Data2: 0xc388, Data3: 0x4e9b, Data4: [8]byte{0x96, 0x1c, 0x6d, 0x2c, 0x93, 0x6c}}

// MountForkedCimLayer mounts the cim at path `cimPath` and returns the mount location of
// that cim. The containerID is used to generate the volumeID for the volume at which
// this CIM is mounted.  containerID is used so that if the shim process crashes for any
// reason, the mounted cim can be correctly cleaned up during `shim delete` call.
func MountForkedCimLayer(ctx context.Context, cimPath, containerID string) (string, error) {
	volumeGUID, err := guid.NewV5(cimMountNamespace, []byte(containerID))
	if err != nil {
		return "", fmt.Errorf("generated cim mount GUID: %w", err)
	}

	vol, err := cimfs.Mount(cimPath, volumeGUID, 0)
	if err != nil {
		return "", err
	}
	return vol, nil
}

// MountBlockCIMLayer mounts the given block cim and returns the mount
// location of that cim. The containerID is used to generate the volumeID for the volume
// at which this CIM is mounted.  containerID is used so that if the shim process crashes
// for any reason, the mounted cim can be correctly cleaned up during `shim delete` call.
func MountBlockCIMLayer(ctx context.Context, layer *cimfs.BlockCIM, containerID string) (_ string, err error) {
	ctx, span := oc.StartSpan(ctx, "MountBlockCIMLayer")
	defer func() {
		oc.SetSpanStatus(span, err)
		span.End()
	}()
	span.AddAttributes(
		trace.StringAttribute("layer", layer.String()))

	var mountFlags uint32
	switch layer.Type {
	case cimfs.BlockCIMTypeDevice:
		mountFlags |= cimfs.CimMountBlockDeviceCim
	case cimfs.BlockCIMTypeSingleFile:
		mountFlags |= cimfs.CimMountSingleFileCim
	default:
		return "", fmt.Errorf("invalid BlockCIMType for merged layer: %w", os.ErrInvalid)
	}

	volumeGUID, err := guid.NewV5(cimMountNamespace, []byte(containerID))
	if err != nil {
		return "", fmt.Errorf("generated cim mount GUID: %w", err)
	}

	cimPath := filepath.Join(layer.BlockPath, layer.CimName)

	log.G(ctx).WithFields(logrus.Fields{
		"flags":  mountFlags,
		"volume": volumeGUID.String(),
	}).Debug("mounting block layer CIM")

	vol, err := cimfs.Mount(cimPath, volumeGUID, mountFlags)
	if err != nil {
		return "", err
	}
	return vol, nil
}

// MergeMountBlockCIMLayer mounts the given merged block cim and returns the mount
// location of that cim. The containerID is used to generate the volumeID for the volume
// at which this CIM is mounted.  containerID is used so that if the shim process crashes
// for any reason, the mounted cim can be correctly cleaned up during `shim delete` call.
// parentLayers MUST be in the base to topmost order. I.e base layer should be at index 0
// and immediate parent MUST be at the last index.
func MergeMountBlockCIMLayer(ctx context.Context, mergedLayer *cimfs.BlockCIM, parentLayers []*cimfs.BlockCIM, containerID string) (_ string, err error) {
	_, span := oc.StartSpan(ctx, "MergeMountBlockCIMLayer")
	defer func() {
		oc.SetSpanStatus(span, err)
		span.End()
	}()
	span.AddAttributes(
		trace.StringAttribute("merged layer", mergedLayer.String()),
		trace.StringAttribute("parent layers", fmt.Sprintf("%v", parentLayers)))

	var mountFlags uint32
	switch mergedLayer.Type {
	case cimfs.BlockCIMTypeDevice:
		mountFlags |= cimfs.CimMountBlockDeviceCim
	case cimfs.BlockCIMTypeSingleFile:
		mountFlags |= cimfs.CimMountSingleFileCim
	default:
		return "", fmt.Errorf("invalid BlockCIMType for merged layer: %w", os.ErrInvalid)
	}

	volumeGUID, err := guid.NewV5(cimMountNamespace, []byte(containerID))
	if err != nil {
		return "", fmt.Errorf("generated cim mount GUID: %w", err)
	}
	return cimfs.MountMergedBlockCIMs(mergedLayer, parentLayers, mountFlags, volumeGUID)
}

// Unmounts the cim mounted at the given volume
func UnmountCimLayer(ctx context.Context, volume string) error {
	return cimfs.Unmount(volume)
}

func CleanupContainerMounts(containerID string) error {
	volumeGUID, err := guid.NewV5(cimMountNamespace, []byte(containerID))
	if err != nil {
		return fmt.Errorf("generated cim mount GUID: %w", err)
	}

	volPath := fmt.Sprintf("\\\\?\\Volume{%s}\\", volumeGUID.String())

	log.L.WithFields(logrus.Fields{
		"volume":      volPath,
		"containerID": containerID,
	}).Debug("cleanup container CIM mounts")

	if _, err := os.Stat(volPath); err == nil {
		err = cimfs.Unmount(volPath)
		if err != nil {
			return err
		}
	}
	return nil
}

// LayerID provides a unique GUID for each mounted CIM volume.
func LayerID(vol string) (string, error) {
	// since each mounted volume has a unique GUID, just return the same GUID as ID
	if !strings.HasPrefix(vol, "\\\\?\\Volume{") || !strings.HasSuffix(vol, "}\\") {
		return "", fmt.Errorf("volume path %s is not in the expected format", vol)
	} else {
		return strings.TrimSuffix(strings.TrimPrefix(vol, "\\\\?\\Volume{"), "}\\"), nil
	}
}
