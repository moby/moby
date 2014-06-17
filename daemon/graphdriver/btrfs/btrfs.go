// +build linux,amd64

package btrfs

/*
#include <stdlib.h>
#include <dirent.h>
#include <btrfs/ioctl.h>
*/
import "C"

import (
	"fmt"
	"os"
	"path"
	"syscall"
	"unsafe"

	"github.com/dotcloud/docker/archive"
	"github.com/dotcloud/docker/daemon/graphdriver"
	"github.com/dotcloud/docker/pkg/mount"
)

func init() {
	graphdriver.Register("btrfs", Init)
}

func Init(home string, options []string) (graphdriver.Driver, error) {
	rootdir := path.Dir(home)

	var buf syscall.Statfs_t
	if err := syscall.Statfs(rootdir, &buf); err != nil {
		return nil, err
	}

	if graphdriver.FsMagic(buf.Type) != graphdriver.FsMagicBtrfs {
		return nil, graphdriver.ErrPrerequisites
	}

	if err := os.MkdirAll(home, 0700); err != nil {
		return nil, err
	}

	if err := graphdriver.MakePrivate(home); err != nil {
		return nil, err
	}

	return &Driver{
		home: home,
	}, nil
}

type Driver struct {
	home string
}

func (d *Driver) String() string {
	return "btrfs"
}

func (d *Driver) Status() [][2]string {
	return nil
}

func (d *Driver) Cleanup() error {
	return mount.Unmount(d.home)
}

func free(p *C.char) {
	C.free(unsafe.Pointer(p))
}

func openDir(path string) (*C.DIR, error) {
	Cpath := C.CString(path)
	defer free(Cpath)

	dir := C.opendir(Cpath)
	if dir == nil {
		return nil, fmt.Errorf("Can't open dir")
	}
	return dir, nil
}

func closeDir(dir *C.DIR) {
	if dir != nil {
		C.closedir(dir)
	}
}

func getDirFd(dir *C.DIR) uintptr {
	return uintptr(C.dirfd(dir))
}

func subvolCreate(path, name string) error {
	dir, err := openDir(path)
	if err != nil {
		return err
	}
	defer closeDir(dir)

	var args C.struct_btrfs_ioctl_vol_args
	for i, c := range []byte(name) {
		args.name[i] = C.char(c)
	}

	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, getDirFd(dir), C.BTRFS_IOC_SUBVOL_CREATE,
		uintptr(unsafe.Pointer(&args)))
	if errno != 0 {
		return fmt.Errorf("Failed to create btrfs subvolume: %v", errno.Error())
	}
	return nil
}

func subvolSnapshot(src, dest, name string) error {
	srcDir, err := openDir(src)
	if err != nil {
		return err
	}
	defer closeDir(srcDir)

	destDir, err := openDir(dest)
	if err != nil {
		return err
	}
	defer closeDir(destDir)

	var args C.struct_btrfs_ioctl_vol_args_v2
	args.fd = C.__s64(getDirFd(srcDir))
	for i, c := range []byte(name) {
		args.name[i] = C.char(c)
	}

	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, getDirFd(destDir), C.BTRFS_IOC_SNAP_CREATE_V2,
		uintptr(unsafe.Pointer(&args)))
	if errno != 0 {
		return fmt.Errorf("Failed to create btrfs snapshot: %v", errno.Error())
	}
	return nil
}

func subvolDelete(path, name string) error {
	dir, err := openDir(path)
	if err != nil {
		return err
	}
	defer closeDir(dir)

	var args C.struct_btrfs_ioctl_vol_args
	for i, c := range []byte(name) {
		args.name[i] = C.char(c)
	}

	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, getDirFd(dir), C.BTRFS_IOC_SNAP_DESTROY,
		uintptr(unsafe.Pointer(&args)))
	if errno != 0 {
		return fmt.Errorf("Failed to destroy btrfs snapshot: %v", errno.Error())
	}
	return nil
}

func (d *Driver) subvolumesDir() string {
	return path.Join(d.home, "subvolumes")
}

func (d *Driver) subvolumesDirId(id string) string {
	return path.Join(d.subvolumesDir(), id)
}

func (d *Driver) Create(id string, parent string) error {
	subvolumes := path.Join(d.home, "subvolumes")
	if err := os.MkdirAll(subvolumes, 0700); err != nil {
		return err
	}
	if parent == "" {
		if err := subvolCreate(subvolumes, id); err != nil {
			return err
		}
	} else {
		parentDir, err := d.Get(parent, "")
		if err != nil {
			return err
		}
		if err := subvolSnapshot(parentDir, subvolumes, id); err != nil {
			return err
		}
	}
	return nil
}

func (d *Driver) CreateWithParent(newID, parentID, startID, endID string) error {
	var (
		layerData archive.Archive
		err       error
	)
	cDir, err := d.Get(endID, "")
	if err != nil {
		return fmt.Errorf("Error getting container rootfs %s from driver %s: %s", endID, d, err)
	}
	defer d.Put(endID)

	initDir, err := d.Get(startID, "")
	if err != nil {
		return fmt.Errorf("Error getting container init rootfs %s from driver %s: %s", startID, d, err)
	}
	defer d.Put(startID)

	changes, err := archive.ChangesDirs(cDir, initDir)
	if err != nil {
		return fmt.Errorf("Error getting changes between %s and %s from driver %s: %s", initDir, cDir, d, err)
	}

	layerData, err = archive.ExportChanges(cDir, changes)
	if err != nil {
		return fmt.Errorf("Error getting the archive with changes from %s from driver %s: %s", cDir, d, err)
	}

	defer layerData.Close()
	if err := d.Create(newID, parentID); err != nil {
		return fmt.Errorf("Driver %s failed to create image rootfs %s: %s", d, newID, err)
	}
	newImagePath, err := d.Get(newID, "")
	if err != nil {
		return fmt.Errorf("Error getting image rootfs %s from driver %s: %s", newID, d, err)
	}
	defer d.Put(newID)

	if err = archive.ApplyLayer(newImagePath, layerData); err != nil {
		return fmt.Errorf("Error applying changes from %s to %s from driver %s: %s", cDir, newID, d, err)
	}

	return nil
}

func (d *Driver) Remove(id string) error {
	dir := d.subvolumesDirId(id)
	if _, err := os.Stat(dir); err != nil {
		return err
	}
	if err := subvolDelete(d.subvolumesDir(), id); err != nil {
		return err
	}
	return os.RemoveAll(dir)
}

func (d *Driver) Get(id, mountLabel string) (string, error) {
	dir := d.subvolumesDirId(id)
	st, err := os.Stat(dir)
	if err != nil {
		return "", err
	}

	if !st.IsDir() {
		return "", fmt.Errorf("%s: not a directory", dir)
	}

	return dir, nil
}

func (d *Driver) Put(id string) {
	// Get() creates no runtime resources (like e.g. mounts)
	// so this doesn't need to do anything.
}

func (d *Driver) Exists(id string) bool {
	dir := d.subvolumesDirId(id)
	_, err := os.Stat(dir)
	return err == nil
}
