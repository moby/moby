// +build linux

package zfs

/*
#cgo CFLAGS: -I/usr/include/libzfs -I/usr/include/libspl -DHAVE_IOCTL_IN_SYS_IOCTL_H
#cgo LDFLAGS: -lzfs -lnvpair -lzfs_core -luutil -lzpool
#include <locale.h>
#include <stdlib.h>
#include <dirent.h>
#include <libzfs.h>
#include <libzfs_core.h>

int add_snapshot_to_nvl(zfs_handle_t *, void *);
int destroy_check_dependent(zfs_handle_t *, void *);
int destroy_callback(zfs_handle_t *, void *);
*/
import "C"

import (
	"fmt"
	"path"
	"time"
	"syscall"
	"unsafe"
	"strings"

	"github.com/docker/docker/daemon/graphdriver"
	"github.com/docker/docker/pkg/log"
	"github.com/docker/docker/pkg/parsers"
)

type ZfsOptions struct {
	mountPath  string
	zpoolName string
}

func init() {
	graphdriver.Register("zfs", Init)
}

func Init(base string, opt []string) (graphdriver.Driver, error) {
	var err error
	options, err := parseOptions(opt)
	options.mountPath = base
	if err != nil {
		return nil, err
	}

	rootdir := path.Dir(base)

	if options.zpoolName == "" {
		err = checkRootdirFs(rootdir)
		if err != nil {
			return nil, err
		}
	}

	g_zfs := C.libzfs_init()
	if g_zfs == nil {
		return nil, fmt.Errorf("Could not init libzfs")
	}
	C.libzfs_print_on_error(g_zfs, C.B_TRUE)

	if options.zpoolName == "" {
		options.zpoolName, err = lookupZfsPool(rootdir);
		if err != nil {
			return nil, err
		}
	} else {
		var CPoolName = C.CString(options.zpoolName)
		defer free(CPoolName)
		zhp := C.zfs_open(g_zfs, CPoolName, C.ZFS_TYPE_POOL)
		if (zhp == nil) {
			return nil, fmt.Errorf("Could not open provided zfs pool: %s", options.zpoolName)
		}
		C.zfs_close(zhp)
	}

	return &Driver{
		g_zfs:   g_zfs,
		options: options,
	}, nil
}

func parseOptions(opt []string) (ZfsOptions, error) {
	var options ZfsOptions
	options.zpoolName = ""
	for _, option := range opt {
		key, val, err := parsers.ParseKeyValueOpt(option)
		if err != nil {
			return options, err
		}
		key = strings.ToLower(key)
		switch key {
		case "zfs.poolname":
			options.zpoolName = val
		default:
			return options, fmt.Errorf("Unknown option %s\n", key)
		}
	}
	return options, nil
}

func checkRootdirFs(rootdir string) error {
	var buf syscall.Statfs_t
	if err := syscall.Statfs(rootdir, &buf); err != nil {
		fmt.Errorf("Failed to access '%s': %s", rootdir, err)
	}

	if graphdriver.FsMagic(buf.Type) != graphdriver.FsMagicZfs {
		log.Debugf("[zfs] no zpool found for rootdir '%s'", rootdir)
		return graphdriver.ErrPrerequisites
	}
	log.Debugf("[zfs] no zpool found for rootdir '%s'", rootdir)
	return nil
}

var CprocMounts = C.CString("/proc/mounts")
var CopenMod = C.CString("r")

func lookupZfsPool(rootdir string) (string, error){
	var stat syscall.Stat_t
	var Cmnt C.struct_mntent
	var Cfp *C.FILE
	buf := string(make([]byte, 256, 256))
	Cbuf := C.CString(buf)
	defer free(Cbuf)

	if err := syscall.Stat(rootdir, &stat); err != nil {
		return "", fmt.Errorf("Failed to access '%s': %s", rootdir, err)
	}
	wantedDev := stat.Dev

	if Cfp = C.setmntent(CprocMounts, CopenMod); Cfp == nil {
		return "", fmt.Errorf("failed to open /proc/mounts")
	}
	defer C.endmntent(Cfp)

	for C.getmntent_r(Cfp, &Cmnt, Cbuf, 256) != nil {
		dir := C.GoString(Cmnt.mnt_dir)
		if err := syscall.Stat(dir, &stat); err != nil {
			return "", err
		}

		if (stat.Dev == wantedDev) {
			return C.GoString(Cmnt.mnt_fsname), nil
		}
	}
	// should never happen
	return "", fmt.Errorf("failed to find zfs pool in /proc/mounts")
}

func free(p *C.char) {
	C.free(unsafe.Pointer(p))
}

type Driver struct {
	g_zfs   *C.libzfs_handle_t
	options ZfsOptions
}

func (d *Driver) String() string {
	log.Debugf("d->String()")
	return "zfs"
}

func (d *Driver) Cleanup() error {
	log.Debugf("d->Cleanup()")
	C.libzfs_fini(d.g_zfs)
	return nil
}

func (d *Driver) Status() [][2]string {
	log.Debugf("d->Status()")
	return nil
}

func volumeCreate(zfs *C.libzfs_handle_t, id, mountpoint string) error {
	var props *C.nvlist_t
	c_id := C.CString(id)
	defer free(c_id)
	c_mountpoint := C.CString(mountpoint)
	defer free(c_mountpoint)

	if C.nvlist_alloc(&props, C.NV_UNIQUE_NAME, 0) != 0 {
		return fmt.Errorf("OOM couldn't allocate memory for props")
	}
	defer C.nvlist_free(props)

	C.nvlist_add_string(props, C.zfs_prop_to_name(C.ZFS_PROP_MOUNTPOINT), c_mountpoint)

	if C.zfs_create(zfs, c_id, C.ZFS_TYPE_FILESYSTEM, props) != 0 {
		return fmt.Errorf("Couldn't create zfs %s", id)
	}

	zhp := C.zfs_open(zfs, c_id, C.ZFS_TYPE_DATASET)
	if zhp == nil {
		return fmt.Errorf("Couldn't open fs")
	}
	defer C.zfs_close(zhp)

	if C.zfs_mount(zhp, nil, 0) != 0 {
		return fmt.Errorf("Unable to mount fs")
	}
	return nil
}

func volumeSnapshot(zfs *C.libzfs_handle_t, id string) (string, string, error) {
	var props *C.nvlist_t
	var nvl *C.nvlist_t

	if C.nvlist_alloc(&props, C.NV_UNIQUE_NAME, 0) != 0 {
		return "", "", fmt.Errorf("Couldn't allocate memory for snapshot properties")
	}
	defer C.nvlist_free(props)

	if C.nvlist_alloc(&nvl, C.NV_UNIQUE_NAME, 0) != 0 {
		return "", "", fmt.Errorf("Couldn't allocate memory for snapshot list")
	}
	defer C.nvlist_free(nvl)

	snapshotName := fmt.Sprintf("%d", time.Now().Nanosecond())
	snapshotPath := id + "@" + snapshotName
	c_snapshotPath := C.CString(snapshotPath)
	defer free(c_snapshotPath)

	C.fnvlist_add_boolean(nvl, c_snapshotPath)

	if C.zfs_snapshot_nvl(zfs, nvl, props) != 0 {
		return "", "", fmt.Errorf("Error snapshoting %s", id)
	}

	return snapshotPath, snapshotName, nil
}

func volumeClone(zfs *C.libzfs_handle_t, snapshot, id, mountpoint string) (*C.zfs_handle_t, error) {
	c_snapshot := C.CString(snapshot)
	defer free(c_snapshot)
	c_id := C.CString(id)
	defer free(c_id)
	c_mountpoint := C.CString(mountpoint)
	defer free(c_mountpoint)

	var props *C.nvlist_t
	if C.nvlist_alloc(&props, C.NV_UNIQUE_NAME, 0) != 0 {
		return nil, fmt.Errorf("Couldn't allocate memory for snapshot properties")
	}
	defer C.nvlist_free(props)

	C.nvlist_add_string(props, C.zfs_prop_to_name(C.ZFS_PROP_MOUNTPOINT), c_mountpoint)

	zhp := C.zfs_open(zfs, c_snapshot, C.ZFS_TYPE_SNAPSHOT)
	if zhp == nil {
		return nil, fmt.Errorf("Couldn't open snapshot %s", snapshot)
	}
	defer C.zfs_close(zhp)

	if C.zfs_clone(zhp, c_id, props) != 0 {
		return nil, fmt.Errorf("Couldn't clone snapshot")
	}

	clone := C.zfs_open(zfs, c_id, C.ZFS_TYPE_DATASET)
	if clone == nil {
		return nil, fmt.Errorf("Couldn't open clone")
	}
	// No defer here, we're returning clone. It's caller responsibility to close the handle

	if C.zfs_mount(clone, nil, 0) != 0 {
		return nil, fmt.Errorf("Unable to mount clone")
	}

	return clone, nil
}

//export add_snapshot_to_nvl
func add_snapshot_to_nvl(zhp *C.zfs_handle_t, data unsafe.Pointer) C.int {
	var nvl *C.nvlist_t
	nvl = (*C.nvlist_t)(data)

	C.fnvlist_add_boolean(nvl, C.zfs_get_name(zhp))
	C.zfs_close(zhp)

	return 0
}

func volumeSnapshotDelete(zfs *C.libzfs_handle_t, parent string, snapshotName string) error {
	c_parent := C.CString(parent)
	defer free(c_parent)
	c_snapshotName := C.CString(snapshotName)
	defer free(c_snapshotName)

	var nvl *C.nvlist_t

	nvl = C.fnvlist_alloc()
	defer C.fnvlist_free(nvl)

	zhp := C.zfs_open(zfs, c_parent, C.ZFS_TYPE_FILESYSTEM)
	if zhp == nil {
		return fmt.Errorf("Couldn't find snapshot for deletion")
	}
	defer C.zfs_close(zhp)

	C.zfs_iter_snapspec(zhp, c_snapshotName,
		(C.zfs_iter_f)(unsafe.Pointer(C.add_snapshot_to_nvl)),
		(unsafe.Pointer)(unsafe.Pointer(nvl)))
	C.zfs_destroy_snaps_nvl(zfs, nvl, C.B_TRUE)

	return nil
}

func volumeCloneFrom(zfs *C.libzfs_handle_t, id, parent, mountPoint string) error {
	var err error
	// Snapshot parent
	snapshotPath, snapshotName, err := volumeSnapshot(zfs, parent)
	if err != nil {
		return err
	}

	// Clone from parent
	clone, err := volumeClone(zfs, snapshotPath, id, mountPoint)
	if err != nil {
		return err
	}
	defer C.zfs_close(clone)

	// Remove snapshot
	err = volumeSnapshotDelete(zfs, parent, snapshotName)
	if err != nil {
		return err
	}

	return nil
}

func (d *Driver) ZfsPath(id string) string {
	log.Debugf("d->ZfsPath(%s)", id)
	return d.options.zpoolName + "/" + id
}

func (d *Driver) Create(id string, parent string) error {
	log.Debugf("d->Create(%s, %s)", id, parent)
	mountPoint := path.Join(d.options.mountPath, "graph", id)
	if parent == "" {
		return volumeCreate(d.g_zfs, d.ZfsPath(id), mountPoint)
	} else {
		return volumeCloneFrom(d.g_zfs, d.ZfsPath(id), d.ZfsPath(parent), mountPoint)
	}
}

type destroy_cbdata struct {
	cb_target       *C.zfs_handle_t
	cb_zfs          *C.libzfs_handle_t
	cb_first        bool
	cb_error        bool
	cb_batchedsnaps *C.nvlist_t
}

//export destroy_check_dependent
func destroy_check_dependent(zhp *C.zfs_handle_t, data unsafe.Pointer) C.int {
	defer C.zfs_close(zhp)

	var cb *destroy_cbdata
	cb = (*destroy_cbdata)(data)

	var tname = C.GoString(C.zfs_get_name(cb.cb_target))
	var name = C.GoString(C.zfs_get_name(zhp))
	// Do not free those char* (zfs internals)

	if name[:len(tname)] == tname &&
		name[len(tname)] == '@' {
		// Element has snapshot, we will delete snapshots
	} else if name[:len(tname)] == tname &&
		name[len(tname)] == '/' {
		// Element has childrens
		cb.cb_error = true
	} else {
		// Element has clones
		cb.cb_error = true
	}

	return 0
}

//export destroy_callback
func destroy_callback(zhp *C.zfs_handle_t, data unsafe.Pointer) C.int {
	defer C.zfs_close(zhp)

	var cb *destroy_cbdata
	cb = (*destroy_cbdata)(data)
	c_name := C.zfs_get_name(zhp)
	// Do not free c_name, it's from zfs internal structs

	if C.zfs_get_type(zhp) == C.ZFS_TYPE_SNAPSHOT {
		C.fnvlist_add_boolean(cb.cb_batchedsnaps, c_name)
	} else {
		var err = C.zfs_destroy_snaps_nvl(cb.cb_zfs, cb.cb_batchedsnaps, C.B_FALSE)

		C.fnvlist_free(cb.cb_batchedsnaps)
		cb.cb_batchedsnaps = C.fnvlist_alloc()

		if err != 0 ||
			C.zfs_unmount(zhp, nil, 0) != 0 ||
			C.zfs_destroy(zhp, C.B_FALSE) != 0 {
			return -1
		}
	}

	return 0
}

func (d *Driver) Remove(id string) error {
	log.Debugf("d->Remove(%s)", id)
	// execute:
	//   zfs destroy -d id
	// remove head, children will be removed once dereferenced

	var cb destroy_cbdata
	c_fullpath := C.CString(d.ZfsPath(id))
	defer free(c_fullpath)
	cb.cb_error = false
	cb.cb_zfs = d.g_zfs

	// Open zfs dataset
	zhp := C.zfs_open(d.g_zfs, c_fullpath, C.ZFS_TYPE_DATASET)
	if zhp == nil {
		return fmt.Errorf("Couldn't locate %s", id)
	}
	// No close zhp, destroy callback take care of this
	cb.cb_target = zhp

	// Ensure no clone is present
	cb.cb_first = true
	if C.zfs_iter_dependents(zhp,
		C.B_TRUE,
		(C.zfs_iter_f)(unsafe.Pointer(C.destroy_check_dependent)),
		(unsafe.Pointer)(unsafe.Pointer(&cb))) != 0 {
		C.zfs_close(zhp)
		return fmt.Errorf("Error scanning childrens of %s", id)
	}
	if cb.cb_error != false {
		return fmt.Errorf("cannot destroy %s: filesystem has children", id)
	}

	// Delete snapshots
	cb.cb_batchedsnaps = C.fnvlist_alloc()
	defer C.fnvlist_free(cb.cb_batchedsnaps)
	if C.zfs_iter_dependents(zhp,
		C.B_FALSE,
		(C.zfs_iter_f)(unsafe.Pointer(C.destroy_callback)),
		(unsafe.Pointer)(unsafe.Pointer(&cb))) != 0 {
		C.zfs_close(zhp)
		return fmt.Errorf("cannot destroy %s: filesystem has children", id)
	}

	var errdestroy = destroy_callback(zhp, (unsafe.Pointer)(unsafe.Pointer(&cb)))
	if errdestroy == 0 {
		errdestroy = C.zfs_destroy_snaps_nvl(d.g_zfs, cb.cb_batchedsnaps, C.B_FALSE)
	}
	if errdestroy != 0 {
		return fmt.Errorf("cannot destroy %s: filesystem has children", id)
	}
	zhp = nil

	//zhp has been closed by destroy_callback
	return nil
}

func zfs_read_mountpoint(zhp *C.zfs_handle_t) (string, error) {
	var sourcetype C.zprop_source_t
	buf := make([]byte, C.ZFS_MAXPROPLEN)
	source := make([]byte, C.ZFS_MAXNAMELEN)

	if C.zfs_prop_get(zhp, C.ZFS_PROP_MOUNTPOINT,
		(*C.char)(unsafe.Pointer(&buf[0])), C.ZFS_MAXPROPLEN,
		&sourcetype,
		(*C.char)(unsafe.Pointer(&source[0])), C.ZFS_MAXNAMELEN,
		C.B_FALSE) != 0 {
		return "", fmt.Errorf("No such property mountpoint")
	}

	return C.GoString((*C.char)(unsafe.Pointer(&buf[0]))), nil
}

func (d *Driver) Get(id, mountLabel string) (string, error) {
	log.Debugf("d->Get(%s, %s)", id, mountLabel)
	c_fullpath := C.CString(d.ZfsPath(id))
	defer free(c_fullpath)

	var zhp = C.zfs_open(d.g_zfs, c_fullpath, C.ZFS_TYPE_DATASET)
	if zhp == nil {
		return "", fmt.Errorf("Couldn't locate %s", id)
	}
	defer C.zfs_close(zhp)

	mountPoint, err := zfs_read_mountpoint(zhp)
	if err != nil {
		return "", err
	}

	// Need to get back zfs get -o mountpoint
	return mountPoint, nil
}

func (d *Driver) Put(id string) {
	log.Debugf("d->Id(%s)", id)
	// FS is already mounted
}

func (d *Driver) Exists(id string) bool {
	log.Debugf("d->Exists(%s)", id)
	c_fullpath := C.CString(d.ZfsPath(id))
	defer free(c_fullpath)

	var zhp = C.zfs_open(d.g_zfs, c_fullpath, C.ZFS_TYPE_DATASET)
	if zhp == nil {
		return false
	}
	defer C.zfs_close(zhp)

	return true
}
