// +build linux

package ceph

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"strings"
	"sync"
	"syscall"

	"github.com/docker/docker/daemon/graphdriver"
	"github.com/docker/docker/pkg/mount"
	"github.com/docker/docker/pkg/parsers"
	"github.com/docker/libcontainer/label"
)

func init() {
	graphdriver.Register("ceph", Init)
}

type Driver struct {
	home          string
	devices       map[string]*string
	devicesLock   sync.Mutex
	dataPoolName  string
	imagePrefix   string
	baseImageName string
	clientId      string
	rados         Rados
	ioctx         RadosIoCtx
}

func Init(home string, options []string) (graphdriver.Driver, error) {
	d := &Driver{
		home:          home,
		devices:       make(map[string]*string),
		dataPoolName:  "docker-data",
		imagePrefix:   "",
		baseImageName: "base-image",
		clientId:      "admin",
	}

	for _, option := range options {
		key, val, err := parsers.ParseKeyValueOpt(option)
		if err != nil {
			return nil, err
		}

		key = strings.ToLower(key)

		switch key {
		case "ceph.datapool":
			d.dataPoolName = val
		case "ceph.imageprefix":
			d.imagePrefix = val
		case "ceph.client":
			d.clientId = val
		default:
			return nil, fmt.Errorf("Unknown option %s\n", key)
		}
	}

	if rados, ioctx, err := connectToRadosCluster(d.clientId, d.dataPoolName); err != nil {
		return nil, err
	} else {
		d.rados = rados
		d.ioctx = ioctx
	}

	baseImageName := d.getImageName(d.baseImageName)

	if err := createBaseImageIfNeeded(d.ioctx, d.dataPoolName, baseImageName); err != nil {
		return nil, err
	}

	if err := os.MkdirAll(home, 0700); err != nil {
		return nil, err
	}

	if err := mount.MakePrivate(home); err != nil {
		return nil, err
	}

	return graphdriver.NaiveDiffDriver(d), nil
}

func (d *Driver) String() string {
	return "ceph"
}

func (d *Driver) Status() [][2]string {
	status := [][2]string{
		{"Pool Objects", ""},
	}
	return status
}

func (d *Driver) Cleanup() error {
	if err := mount.Unmount(d.home); err == nil {
		return err
	}

	return nil
}

func (d *Driver) getImageName(id string) string {
	return d.imagePrefix + id
}

func (d *Driver) Create(id, parent string) error {
	var parentName string;
	imageName := d.getImageName(id)

	if parent == "" {
		parentName = d.getImageName(d.baseImageName)
	} else {
		parentName = d.getImageName(parent)
	}

	if err := createImage(d.ioctx, imageName, parentName); err != nil {
		fmt.Errorf("Error creating image %s (parent %s): %s", imageName, parentName, err)
	}

	return nil
}

func (d *Driver) Remove(id string) error {
	imageName := d.getImageName(id)

	// This assumes the device has been properly Get/Put:ed and thus is unmounted
	if err := deleteImage(d.ioctx, imageName); err != nil {
		fmt.Errorf("Error deleting image %s: %s", imageName, err)
	}

	mountPoint := path.Join(d.home, "mnt", id)
	if err := os.RemoveAll(mountPoint); err != nil && !os.IsNotExist(err) {
		return err
	}

	return nil
}

func (d *Driver) Get(id, mountLabel string) (string, error) {
	mountPoint := path.Join(d.home, "mnt", id)

	if err := os.MkdirAll(mountPoint, 0755); err != nil && !os.IsExist(err) {
		return "", err
	}

	if err := d.mountImage(id, mountPoint, mountLabel); err != nil {
		return "", err
	}

	rootFileSystem := path.Join(mountPoint, "rootfs")

	if err := initRootFileSystemIfNeeded(id, rootFileSystem); err != nil {
		d.unmountImage(id, mountPoint)
		return "", err
	}

	return rootFileSystem, nil
}

func (d *Driver) Put(id string) {
	mountPoint := path.Join(d.home, "mnt", id)

	if err := d.unmountImage(id, mountPoint); err != nil {
		fmt.Errorf("Warning: error unmounting device %s: %s\n", id, err)
	}
}

func (d *Driver) Exists(id string) bool {
	imageName := d.getImageName(id)

	ex, err := imageExists(d.ioctx, imageName)
	if err != nil {
		fmt.Errorf("Warning: assuming image not present %s: %s\n", imageName, err)
		return false
	}

	return ex
}

func (d *Driver) mountImage(id, mountPoint, mountLabel string) error {
	d.devicesLock.Lock()
	defer d.devicesLock.Unlock()

	imageName := d.getImageName(id)

	if d.devices[id] != nil {
		return fmt.Errorf("Cannot mount the same image twice: %s", id)
	}

	rbdDevice, err := mapImageToRbdDevice(d.dataPoolName, imageName)
	if err != nil {
		return err
	}

	flags := uintptr(syscall.MS_MGC_VAL)
	options := label.FormatMountLabel("", mountLabel)

	if err := syscall.Mount(rbdDevice, mountPoint, "ext4", flags, options); err != nil {
		unmapImageFromRbdDevice(rbdDevice)
		return err
	}

	d.devices[id] = &rbdDevice

	return nil
}

func (d *Driver) unmountImage(id, mountPoint string) error {
	d.devicesLock.Lock()
	defer d.devicesLock.Unlock()

	rbdDevicePtr := d.devices[id]
	if rbdDevicePtr == nil {
		return fmt.Errorf("Cannot unmount image: %s", id)
	}

	// No matter what happen next, the procedure is irrecoverable
	delete(d.devices, id)

	if err := syscall.Unmount(mountPoint, 0); err != nil {
		return err
	}

	if err := unmapImageFromRbdDevice(*rbdDevicePtr); err != nil {
		return err
	}

	return nil
}

func initRootFileSystemIfNeeded(id, rootFileSystem string) error {
	if err := os.MkdirAll(rootFileSystem, 0755); err != nil && !os.IsExist(err) {
		return err
	}

	// Create an "id" file with the container/image id in it to help
	// reconscruct this in case of later problems
	idFile := path.Join(rootFileSystem, "id")
	if _, err := os.Stat(idFile); err != nil && os.IsNotExist(err) {
		if err := ioutil.WriteFile(idFile, []byte(id), 0600); err != nil {
			return err
		}
	}

	return nil
}
