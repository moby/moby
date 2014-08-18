// +build linux

package ceph

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
)

type RbdMappingInfo struct {
	Pool     string `json:"pool"`
	Name     string `json:"name"`
	Snapshot string `json:"snap"`
	Device   string `json:"device"`
}

const (
	RadosConfigFile     = "/etc/ceph/ceph.conf"
	RadosSysfsDevices   = "/sys/devices/rbd"
	RbdDevicePath       = "/dev/rbd"
	DockerBaseImageSize = 10 * 1024 * 1024 * 1024
	DockerCloneSnapshot = "docker-clone-snapshot"
)

func connectToRadosCluster(client, pool string) (Rados, RadosIoCtx, error) {
	rados, err := radosCreate(client)
	if err != nil {
		return nil, nil, err
	}

	if err := radosConfReadFile(rados, RadosConfigFile); err != nil {
		return nil, nil, err
	}

	if err := radosConnect(rados); err != nil {
		return nil, nil, err
	}

	ioctx, err := radosIoCtxCreate(rados, pool)
	if err != nil {
		radosDisconnect(rados)
		return nil, nil, err
	}

	return rados, ioctx, nil
}

func createDockerSnapshot(ioctx RadosIoCtx, id string) error {
	img, err := rbdOpen(ioctx, id)
	if err != nil {
		return err
	}

	defer rbdClose(img)

	if err := rbdSnapshotCreate(img, DockerCloneSnapshot); err != nil {
		return err
	}

	return nil
}

func cloneImage(ioctx RadosIoCtx, id, parent string) error {
	img, err := rbdOpenSnapshot(ioctx, parent, DockerCloneSnapshot)
	if err != nil {
		if err != RbdNotFoundError {
			return err
		}

		if err := createDockerSnapshot(ioctx, parent); err != nil {
			return err
		}

		img, err = rbdOpenSnapshot(ioctx, parent, DockerCloneSnapshot)
		if err != nil {
			return err
		}
	}

	defer rbdClose(img)

	if err := rbdSnapshotProtect(img, DockerCloneSnapshot); err != nil {
		if err != RbdBusyError {
			return err
		}
	}

	if err := rbdClone(ioctx, parent, DockerCloneSnapshot, id); err != nil {
		return err
	}

	return nil
}

func createImage(ioctx RadosIoCtx, id, parent string) error {
	if err := cloneImage(ioctx, id, parent); err != nil {
		return err
	}

	return nil
}

func imageExists(ioctx RadosIoCtx, id string) (bool, error) {
	img, err := rbdOpen(ioctx, id)
	if err != nil {
		if err != RbdNotFoundError {
			return false, err
		}

		return false, nil
	}

	defer rbdClose(img)

	return true, nil
}

func mapImageToRbdDevice(pool, id string) (string, error) {
	out, err := exec.Command("rbd", "--pool", pool, "map", id).Output()
	if err != nil {
		return "", err
	}

	rbdDevice := strings.TrimRight(string(out), "\n")

	if rbdDevice != "" {
		return rbdDevice, nil
	}

	// Older rbd binaries are not printing the device on mapping so
	// we have to discover it with showmapped.
	out, err = exec.Command("rbd", "showmapped", "--format", "json").Output()
	if err != nil {
		return "", err
	}

	mappingInfo := map[string]*RbdMappingInfo{}
	json.Unmarshal(out, &mappingInfo)

	for _, info := range mappingInfo {
		if info.Pool == pool && info.Name == id {
			return info.Device, nil
		}
	}

	return "", fmt.Errorf("Unable to map image %s\n", id)
}

func unmapImageFromRbdDevice(rbdDevice string) error {
	if err := exec.Command("rbd", "unmap", rbdDevice).Run(); err != nil {
		return err
	}

	return nil
}

func createBaseImageIfNeeded(ioctx RadosIoCtx, pool, id string) error {
	if img, err := rbdOpenSnapshot(ioctx, id, DockerCloneSnapshot); err == nil {
		defer rbdClose(img)
		return nil // Base image already present
	} else if err != RbdNotFoundError {
		return err
	}

	// Removing half-baked base image if the snapshot was missing
	if err := deleteImage(ioctx, id); err != nil {
		return err
	}

	if err := rbdCreate(ioctx, id,
		DockerBaseImageSize); err != nil {
		return err
	}

	rbdDevice, err := mapImageToRbdDevice(pool, id)
	if err != nil {
		return err
	}

	defer unmapImageFromRbdDevice(rbdDevice)

	if err := exec.Command("mkfs.ext4", rbdDevice).Run(); err != nil {
		return err
	}

	if err := createDockerSnapshot(ioctx, id); err != nil {
		return err
	}

	return nil
}

func deleteDockerSnapshotIfPresent(ioctx RadosIoCtx, id string) error {
	img, err := rbdOpenSnapshot(ioctx, id, DockerCloneSnapshot)
	if err != nil {
		if err == RbdNotFoundError {
			return nil
		}

		return err
	}

	defer rbdClose(img)

	if err := rbdSnapshotUnprotect(img, DockerCloneSnapshot); err != nil {
		if err == RbdIvalidArgError {
			return nil
		}

		return err
	}

	if err := rbdSnapshotRemove(img, DockerCloneSnapshot); err != nil {
		if err != RbdNotFoundError {
			return err
		}
	}

	return nil
}

func deleteImage(ioctx RadosIoCtx, id string) error {
	if err := deleteDockerSnapshotIfPresent(ioctx, id); err != nil {
		return err
	}

	if err := rbdRemove(ioctx, id); err != nil {
		if err != RbdNotFoundError {
			return err
		}
	}

	return nil
}
