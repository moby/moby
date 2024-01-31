//go:build windows

package cim

import (
	"context"
	"fmt"
	"os"
	"sync"

	"github.com/Microsoft/go-winio/pkg/guid"
	hcsschema "github.com/Microsoft/hcsshim/internal/hcs/schema2"
	cimfs "github.com/Microsoft/hcsshim/pkg/cimfs"
)

// a cache of cim layer to its mounted volume - The mount manager plugin currently doesn't have an option of
// querying a mounted cim to get the volume at which it is mounted, so we maintain a cache of that here
var (
	cimMounts       map[string]string = make(map[string]string)
	cimMountMapLock sync.Mutex
	// A random GUID used as a namespace for generating cim mount volume GUIDs: 6827367b-c388-4e9b-95ec-961c6d2c936c
	cimMountNamespace guid.GUID = guid.GUID{Data1: 0x6827367b, Data2: 0xc388, Data3: 0x4e9b, Data4: [8]byte{0x96, 0x1c, 0x6d, 0x2c, 0x93, 0x6c}}
)

// MountCimLayer mounts the cim at path `cimPath` and returns the mount location of that cim.  This method
// uses the `CimMountFlagCacheFiles` mount flag when mounting the cim.  The containerID is used to generated
// the volumeID for the volume at which this CIM is mounted.  containerID is used so that if the shim process
// crashes for any reason, the mounted cim can be correctly cleaned up during `shim delete` call.
func MountCimLayer(ctx context.Context, cimPath, containerID string) (string, error) {
	volumeGUID, err := guid.NewV5(cimMountNamespace, []byte(containerID))
	if err != nil {
		return "", fmt.Errorf("generated cim mount GUID: %w", err)
	}

	vol, err := cimfs.Mount(cimPath, volumeGUID, hcsschema.CimMountFlagCacheFiles)
	if err != nil {
		return "", err
	}

	cimMountMapLock.Lock()
	defer cimMountMapLock.Unlock()
	cimMounts[fmt.Sprintf("%s_%s", containerID, cimPath)] = vol

	return vol, nil
}

// Unmount unmounts the cim at mounted for given container.
func UnmountCimLayer(ctx context.Context, cimPath, containerID string) error {
	cimMountMapLock.Lock()
	defer cimMountMapLock.Unlock()
	if vol, ok := cimMounts[fmt.Sprintf("%s_%s", containerID, cimPath)]; !ok {
		return fmt.Errorf("cim %s not mounted", cimPath)
	} else {
		delete(cimMounts, fmt.Sprintf("%s_%s", containerID, cimPath))
		err := cimfs.Unmount(vol)
		if err != nil {
			return err
		}
	}
	return nil
}

// GetCimMountPath returns the volume at which a cim is mounted. If the cim is not mounted returns error
func GetCimMountPath(cimPath, containerID string) (string, error) {
	cimMountMapLock.Lock()
	defer cimMountMapLock.Unlock()

	if vol, ok := cimMounts[fmt.Sprintf("%s_%s", containerID, cimPath)]; !ok {
		return "", fmt.Errorf("cim %s not mounted", cimPath)
	} else {
		return vol, nil
	}
}

func CleanupContainerMounts(containerID string) error {
	volumeGUID, err := guid.NewV5(cimMountNamespace, []byte(containerID))
	if err != nil {
		return fmt.Errorf("generated cim mount GUID: %w", err)
	}

	volPath := fmt.Sprintf("\\\\?\\Volume{%s}\\", volumeGUID.String())
	if _, err := os.Stat(volPath); err == nil {
		err = cimfs.Unmount(volPath)
		if err != nil {
			return err
		}
	}
	return nil
}
