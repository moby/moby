// +build linux

package btrfs // import "github.com/docker/docker/daemon/graphdriver/btrfs"

/*
#include <dirent.h>
#include <fcntl.h>
#include <unistd.h>
#include <btrfs/ctree.h>

static int set_name_btrfs_ioctl_vol_args(struct btrfs_ioctl_vol_args* btrfs_struct, char* name) {
    int r = snprintf(btrfs_struct->name,
                     sizeof(btrfs_struct->name),
                     "%s", name);
    free(name);
    return r;
}

static int set_name_btrfs_ioctl_vol_args_v2(struct btrfs_ioctl_vol_args_v2* btrfs_struct, char* name) {
    int r = snprintf(btrfs_struct->name,
                     sizeof(btrfs_struct->name),
                     "%s", name);
    free(name);
    return r;
}

static inline __u16 _le16toh(__le16 i) {return le16toh(i);}
static inline __u64 _le64toh(__le64 i) {return le64toh(i);}

static inline int _openat(int dirfd, const char *pathname, int flags) {return openat(dirfd, pathname, flags);}
*/
import "C"

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"unsafe"

	"github.com/docker/docker/daemon/graphdriver"
	"github.com/docker/docker/pkg/containerfs"
	"github.com/docker/docker/pkg/idtools"
	"github.com/docker/docker/pkg/mount"
	"github.com/docker/docker/pkg/parsers"
	"github.com/docker/docker/pkg/system"
	units "github.com/docker/go-units"
	"github.com/opencontainers/selinux/go-selinux/label"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"golang.org/x/sys/unix"
)

func init() {
	graphdriver.Register("btrfs", Init)
}

type btrfsOptions struct {
	minSpace C.__u64
}

// Init returns a new BTRFS driver.
// An error is returned if BTRFS is not supported.
func Init(home string, options []string, uidMaps, gidMaps []idtools.IDMap) (graphdriver.Driver, error) {

	// Perform feature detection on /var/lib/docker/btrfs if it's an existing directory.
	// This covers situations where /var/lib/docker/btrfs is a mount, and on a different
	// filesystem than /var/lib/docker.
	// If the path does not exist, fall back to using /var/lib/docker for feature detection.
	testdir := home
	if _, err := os.Stat(testdir); os.IsNotExist(err) {
		testdir = filepath.Dir(testdir)
	}

	fsMagic, err := graphdriver.GetFSMagic(testdir)
	if err != nil {
		return nil, err
	}

	if fsMagic != graphdriver.FsMagicBtrfs {
		return nil, graphdriver.ErrPrerequisites
	}

	rootUID, rootGID, err := idtools.GetRootUIDGID(uidMaps, gidMaps)
	if err != nil {
		return nil, err
	}
	if err := idtools.MkdirAllAndChown(home, 0700, idtools.Identity{UID: rootUID, GID: rootGID}); err != nil {
		return nil, err
	}

	opt, userDiskQuota, err := parseOptions(options)
	if err != nil {
		return nil, err
	}

	// For some reason shared mount propagation between a container
	// and the host does not work for btrfs, and a remedy is to bind
	// mount graphdriver home to itself (even without changing the
	// propagation mode).
	err = mount.MakeMount(home)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to make %s a mount", home)
	}

	driver := &Driver{
		home:    home,
		uidMaps: uidMaps,
		gidMaps: gidMaps,
		options: opt,
	}

	if userDiskQuota {
		if err := driver.subvolEnableQuota(); err != nil {
			return nil, err
		}
	}

	return graphdriver.NewNaiveDiffDriver(driver, uidMaps, gidMaps), nil
}

func parseOptions(opt []string) (btrfsOptions, bool, error) {
	var options btrfsOptions
	userDiskQuota := false
	for _, option := range opt {
		key, val, err := parsers.ParseKeyValueOpt(option)
		if err != nil {
			return options, userDiskQuota, err
		}
		key = strings.ToLower(key)
		switch key {
		case "btrfs.min_space":
			minSpace, err := units.RAMInBytes(val)
			if err != nil {
				return options, userDiskQuota, err
			}
			if minSpace < 0 {
				return options, userDiskQuota, fmt.Errorf("btrfs: min_space can't be negative but got: %d", minSpace)
			}
			userDiskQuota = true
			options.minSpace = C.__u64(minSpace)
		default:
			return options, userDiskQuota, fmt.Errorf("Unknown option %s", key)
		}
	}
	return options, userDiskQuota, nil
}

// Driver contains information about the filesystem mounted.
type Driver struct {
	// root of the file system
	home    string
	uidMaps []idtools.IDMap
	gidMaps []idtools.IDMap
	options btrfsOptions
}

// String prints the name of the driver (btrfs).
func (d *Driver) String() string {
	return "btrfs"
}

// Status returns current driver information in a two dimensional string array.
// Output contains "Build Version" and "Library Version" of the btrfs libraries used.
// Version information can be used to check compatibility with your kernel.
func (d *Driver) Status() [][2]string {
	status := [][2]string{}
	if bv := btrfsBuildVersion(); bv != "-" {
		status = append(status, [2]string{"Build Version", bv})
	}
	if lv := btrfsLibVersion(); lv != -1 {
		status = append(status, [2]string{"Library Version", fmt.Sprintf("%d", lv)})
	}
	return status
}

// GetMetadata returns empty metadata for this driver.
func (d *Driver) GetMetadata(id string) (map[string]string, error) {
	return nil, nil
}

// Cleanup unmounts the home directory.
func (d *Driver) Cleanup() error {
	umountErr := mount.Unmount(d.home)
	if umountErr != nil {
		return umountErr
	}

	return nil
}

func free(p *C.char) {
	C.free(unsafe.Pointer(p))
}

func openDir(dirpath string) (*C.DIR, error) {
	if len(dirpath) >= C.PATH_MAX {
		// Remove // and . to more safely slice the path
		dirpath = path.Clean(dirpath)
	}

	if len(dirpath) < C.PATH_MAX {
		Cdirpath := C.CString(dirpath)
		dir, err := C.opendir(Cdirpath)
		free(Cdirpath)
		if dir == nil {
			return nil, fmt.Errorf("Can't open dir %s: %v", dirpath, err)
		}
		return dir, nil
	}

	// Path too long to fit in one ioctl so we will move in a small steps
	head := path.Dir(dirpath[:C.PATH_MAX-1])
	tail := dirpath[len(head)+1:]

	closefd := func(fd C.int) {
		if r, err := C.close(fd); r < 0 {
			logrus.Errorf("Failed to close %v: %v", fd, err)
		}
	}
	var oflags C.int = (C.O_RDONLY | C.O_NDELAY | C.O_DIRECTORY | C.O_CLOEXEC)

	tmpCstr := C.CString(head)
	headfd, err := C._openat(C.AT_FDCWD, tmpCstr, oflags)
	free(tmpCstr)
	if headfd < 0 {
		return nil, fmt.Errorf("Can't open dir %s: %v", head, err)
	}

	relfd := headfd
	for len(tail) > 0 {
		if len(tail) >= C.PATH_MAX {
			head = path.Dir(tail[:C.PATH_MAX-1])
			tail = tail[len(head)+1:]
		} else {
			head = tail
			tail = ""
		}
		tmpCstr = C.CString(head)
		headfd, err = C._openat(relfd, tmpCstr, oflags)
		free(tmpCstr)
		closefd(relfd)
		if headfd < 0 {
			return nil, fmt.Errorf("Can't open dir %s: %v", head, err)
		}
		relfd = headfd
	}

	dir, err := C.fdopendir(headfd)
	if dir == nil {
		closefd(headfd)
		return nil, fmt.Errorf("Can't get DIR from fd %d for dir %s: %v", headfd, dirpath, err)
	}
	return dir, nil
}

func closeDir(dir *C.DIR) {
	if r, err := C.closedir(dir); r < 0 {
		logrus.Errorf("Failed to closedir: %v", err)
	}
}

func getDirFd(dir *C.DIR) uintptr {
	return uintptr(C.dirfd(dir))
}

func subvolSnapshot(src, dest, name string) error {
	destDir, err := openDir(dest)
	if err != nil {
		return err
	}
	defer closeDir(destDir)

	var args C.struct_btrfs_ioctl_vol_args_v2

	var cmd uintptr = C.BTRFS_IOC_SUBVOL_CREATE_V2
	if src != "" {
		cmd = C.BTRFS_IOC_SNAP_CREATE_V2

		srcDir, err := openDir(src)
		if err != nil {
			return err
		}
		defer closeDir(srcDir)

		args.fd = C.__s64(getDirFd(srcDir))
	}

	if r, err := C.set_name_btrfs_ioctl_vol_args_v2(&args, C.CString(name)); r < 0 {
		return fmt.Errorf("Failed to copy subvolume name %s to args struct: %v", name, err)
	}

	_, _, errno := unix.Syscall(unix.SYS_IOCTL, getDirFd(destDir), cmd, uintptr(unsafe.Pointer(&args)))
	if errno != 0 {
		return fmt.Errorf("Failed to create btrfs subvolume: %v", errno.Error())
	}

	return nil
}

func btrfsIoctlSearchArgsInc(args *C.struct_btrfs_ioctl_search_args) bool {
	/* the objectid, type, offset together make up the btrfs key,
	 * which is considered a single 136byte integer when
	 * comparing. This call increases the counter by one, dealing
	 * with the overflow between the overflows */
	if args.key.min_offset < ^C.__u64(0) {
		args.key.min_offset++
		return false
	}

	if args.key.min_type < C.__u32(^C.__u8(0)) {
		args.key.min_type++
		args.key.min_offset = 0
		return false
	}

	if args.key.min_objectid < ^C.__u64(0) {
		args.key.min_objectid++
		args.key.min_offset = 0
		args.key.min_type = 0
		return false
	}

	return true
}

func btrfsIoctlSearchArgsCompare(args *C.struct_btrfs_ioctl_search_args) bool {
	/* Compare min and max. Return true if min <= max */
	if args.key.min_objectid < args.key.max_objectid {
		return true
	}
	if args.key.min_objectid > args.key.max_objectid {
		return false
	}

	if args.key.min_type < args.key.max_type {
		return true
	}
	if args.key.min_type > args.key.max_type {
		return false
	}

	if args.key.min_offset < args.key.max_offset {
		return true
	}
	if args.key.min_offset > args.key.max_offset {
		return false
	}

	return true
}

func subvolDelete(dirpath, name string, subvolID C.__u64) error {
	dir, err := openDir(dirpath)
	if err != nil {
		return err
	}
	defer closeDir(dir)

	fullPath := path.Join(dirpath, name)

	// Makes subvolume writable
	if err := subvolSetPropRO(fullPath, false); err != nil {
		return err
	}

	subvoldir, err := openDir(fullPath)
	if err != nil {
		return err
	}
	defer closeDir(subvoldir)

	if subvolID == 0 {
		// Get subvol ID
		var lkpargs C.struct_btrfs_ioctl_ino_lookup_args
		lkpargs.objectid = C.BTRFS_FIRST_FREE_OBJECTID
		_, _, errno := unix.Syscall(unix.SYS_IOCTL, getDirFd(subvoldir), C.BTRFS_IOC_INO_LOOKUP,
			uintptr(unsafe.Pointer(&lkpargs)))
		if errno != 0 {
			return fmt.Errorf("Cannot resolve ID for path %s: %v", fullPath, errno.Error())
		}
		subvolID = lkpargs.treeid
	}

	destroySnap := func() syscall.Errno {
		var args C.struct_btrfs_ioctl_vol_args
		if r, err := C.set_name_btrfs_ioctl_vol_args(&args, C.CString(name)); r < 0 {
			return err.(syscall.Errno)
		}
		_, _, errno := unix.Syscall(unix.SYS_IOCTL, getDirFd(dir), C.BTRFS_IOC_SNAP_DESTROY,
			uintptr(unsafe.Pointer(&args)))
		return errno
	}

	// First try to delete right now. Should be most common case
	if errno := destroySnap(); errno == 0 {
		// for the leaf subvolumes, the qgroup id is identical to the subvol id
		return btrfsQgroupDestroyRecursive(dir, subvolID)
	} else if errno != syscall.ENOTEMPTY {
		return fmt.Errorf("Failed to destroy btrfs subvolume %s: %v", fullPath, errno.Error())
	}
	// errno == ENOTEMPTY going to search subvols

	// map to store info about subvols relative path to which is not fit in BTRFS_IOC_INO_LOOKUP
	type subvolData struct {
		name string
		id   C.__u64
	}
	// key - dirid (inode number) of parent dir
	distantDescendants := make(map[C.__u64][]subvolData)

	// Init search key
	var args C.struct_btrfs_ioctl_search_args
	args.key.tree_id = C.BTRFS_ROOT_TREE_OBJECTID
	args.key.min_type = C.BTRFS_ROOT_BACKREF_KEY
	args.key.max_type = C.BTRFS_ROOT_BACKREF_KEY
	args.key.min_objectid = C.BTRFS_FIRST_FREE_OBJECTID
	args.key.max_objectid = C.BTRFS_LAST_FREE_OBJECTID
	args.key.min_transid = 0
	args.key.max_transid = ^C.__u64(0)
	args.key.min_offset = subvolID
	args.key.max_offset = subvolID

	// while search key is converging
	for btrfsIoctlSearchArgsCompare(&args) {
		// Search subvols
		args.key.nr_items = 256
		_, _, errno := unix.Syscall(unix.SYS_IOCTL, getDirFd(dir), C.BTRFS_IOC_TREE_SEARCH,
			uintptr(unsafe.Pointer(&args)))
		if errno != 0 {
			return fmt.Errorf("Failed to search subvols for %s: %v", dirpath, errno.Error())
		}
		if args.key.nr_items <= 0 {
			break
		}
		// Parse search results
		var sh *C.struct_btrfs_ioctl_search_header
		for i, shPtr := C.__u32(0), unsafe.Pointer(&args.buf); i < args.key.nr_items; i, shPtr = i+1, unsafe.Pointer(uintptr(shPtr)+C.sizeof_struct_btrfs_ioctl_search_header+uintptr(sh.len)) {
			sh = (*C.struct_btrfs_ioctl_search_header)(shPtr)

			// Make sure we start the next search at least from this entry
			args.key.min_objectid = sh.objectid
			args.key.min_type = sh._type
			args.key.min_offset = sh.offset

			if sh._type != C.BTRFS_ROOT_BACKREF_KEY {
				continue
			}
			if sh.offset != subvolID {
				continue
			}

			// We found some
			ref := (*C.struct_btrfs_root_ref)(unsafe.Pointer(uintptr(shPtr) + C.sizeof_struct_btrfs_ioctl_search_header))
			childName := C.GoStringN((*C.char)(unsafe.Pointer(uintptr(unsafe.Pointer(ref))+C.sizeof_struct_btrfs_root_ref)), C.int(C._le16toh(ref.name_len)))
			dirid := C._le64toh(ref.dirid)

			// Search relative path to subvol parent dir
			var inoArgs C.struct_btrfs_ioctl_ino_lookup_args
			inoArgs.treeid = subvolID
			inoArgs.objectid = dirid
			_, _, errno := unix.Syscall(unix.SYS_IOCTL, getDirFd(dir), C.BTRFS_IOC_INO_LOOKUP,
				uintptr(unsafe.Pointer(&inoArgs)))
			if errno == syscall.ENAMETOOLONG {
				// Path too long so save it for future walk
				distantDescendants[dirid] = append(distantDescendants[dirid], subvolData{childName, sh.objectid})
			} else if errno != 0 {
				return fmt.Errorf("Failed to search relative path from %s to %s: %v", name, childName, errno.Error())
			} else {
				relPath := C.GoString((*C.char)(unsafe.Pointer(&inoArgs.name)))
				if err := subvolDelete(path.Join(fullPath, relPath), childName, sh.objectid); err != nil {
					return err
				}
			}
		}
		// Increase search key by one, to read the next item, if we can.
		if btrfsIoctlSearchArgsInc(&args) {
			break
		}
	}

	if len(distantDescendants) > 0 {
		var walk func(*C.DIR, string) error
		walk = func(pdir *C.DIR, ppath string) error {
			ent, errno := C.readdir(pdir)
			for ; ent != nil; ent, errno = C.readdir(pdir) {
				if ent.d_type == C.DT_DIR {
					dname := C.GoString((*C.char)(unsafe.Pointer(&ent.d_name)))
					// Check if dir contain subvols
					if childs, ok := distantDescendants[C.__u64(ent.d_ino)]; ok {
						dpath := path.Join(ppath, dname)
						for _, child := range childs {
							if err := subvolDelete(dpath, child.name, child.id); err != nil {
								return fmt.Errorf("Failed to delete btrfs subvolume %+v: %v", child, err)
							}
						}
						delete(distantDescendants, C.__u64(ent.d_ino))
					}
					// Go deeper
					if dname != "." && dname != ".." {
						var oflags C.int = (C.O_RDONLY | C.O_NDELAY | C.O_DIRECTORY | C.O_CLOEXEC | C.O_NOFOLLOW)
						dirfd, err := C._openat(C.dirfd(pdir), (*C.char)(unsafe.Pointer(&ent.d_name)), oflags)
						if dirfd < 0 {
							logrus.Errorf("Can't open dir %s: %v", dname, err)
							continue
						}
						cdir, err := C.fdopendir(dirfd)
						if cdir == nil {
							C.close(dirfd)
							logrus.Errorf("Can't get DIR from fd %d for %s: %v", dirfd, dname, err)
							continue
						}
						defer closeDir(cdir)

						if err := walk(cdir, path.Join(ppath, dname)); err != nil {
							return err
						}
					}
				}
			}
			if errno != nil {
				logrus.Errorf("Error while walk at %s: %v", ppath, errno)
			}
			return nil
		}

		if err := walk(subvoldir, fullPath); err != nil {
			return err
		}
		if len(distantDescendants) > 0 {
			return fmt.Errorf("Child btrfs subvolumes %+v was not reached from %s", distantDescendants, fullPath)
		}
	}

	// For now all descendants should be destroyed and we can try one more time
	if errno := destroySnap(); errno != 0 {
		return fmt.Errorf("Failed to destroy btrfs subvolume %s: %v", fullPath, errno.Error())
	}
	return btrfsQgroupDestroyRecursive(dir, subvolID)
}

/* Destroys the specified qgroup, but unassigns it from all
 * its parents first. Also, it recursively destroys all
 * qgroups it is assgined to that have the same id part of the
 * qgroupid as the specified group. */
func btrfsQgroupDestroyRecursive(dir *C.DIR, qgroupID C.__u64) error {
	qgroupParents := []C.__u64{}

	// Init search key
	var args C.struct_btrfs_ioctl_search_args
	args.key.tree_id = C.BTRFS_QUOTA_TREE_OBJECTID
	args.key.min_type = C.BTRFS_QGROUP_RELATION_KEY
	args.key.max_type = C.BTRFS_QGROUP_RELATION_KEY
	args.key.min_objectid = qgroupID
	args.key.max_objectid = qgroupID
	args.key.min_transid = 0
	args.key.max_transid = ^C.__u64(0)
	args.key.min_offset = 0
	args.key.max_offset = ^C.__u64(0)

	for btrfsIoctlSearchArgsCompare(&args) {
		args.key.nr_items = 256
		_, _, errno := unix.Syscall(unix.SYS_IOCTL, getDirFd(dir), C.BTRFS_IOC_TREE_SEARCH,
			uintptr(unsafe.Pointer(&args)))
		if errno != 0 {
			if errno == syscall.ENOENT { // quota tree missing: quota is disabled
				return nil
			}
			return fmt.Errorf("Failed to search parents for qgroup %d: %v", qgroupID, errno.Error())
		}
		if args.key.nr_items <= 0 {
			break
		}
		// Parse search results
		var sh *C.struct_btrfs_ioctl_search_header
		for i, shPtr := C.__u32(0), unsafe.Pointer(&args.buf); i < args.key.nr_items; i, shPtr = i+1, unsafe.Pointer(uintptr(shPtr)+C.sizeof_struct_btrfs_ioctl_search_header+uintptr(sh.len)) {
			sh = (*C.struct_btrfs_ioctl_search_header)(shPtr)

			// Make sure we start the next search at least from this entry
			args.key.min_objectid = sh.objectid
			args.key.min_type = sh._type
			args.key.min_offset = sh.offset

			if sh._type != C.BTRFS_QGROUP_RELATION_KEY {
				continue
			}
			if sh.offset < sh.objectid {
				continue
			}
			if sh.objectid != qgroupID {
				continue
			}

			qgroupParents = append(qgroupParents, sh.offset)
		}
		// Increase search key by one, to read the next item, if we can.
		if btrfsIoctlSearchArgsInc(&args) {
			break
		}
	}

	btrfsQgroupIDSplit := func(qgroupid C.__u64) C.__u64 {
		return qgroupid & ((C.__u64(1) << C.BTRFS_QGROUP_LEVEL_SHIFT) - 1)
	}

	subvolID := btrfsQgroupIDSplit(qgroupID)

	for _, parent := range qgroupParents {
		//unassign btrfs qgroup from parent
		var aargs C.struct_btrfs_ioctl_qgroup_assign_args
		aargs.assign = C.false
		aargs.src = qgroupID
		aargs.dst = parent
		ret, _, errno := unix.Syscall(unix.SYS_IOCTL, getDirFd(dir), C.BTRFS_IOC_QGROUP_ASSIGN,
			uintptr(unsafe.Pointer(&aargs)))
		if errno != 0 {
			return fmt.Errorf("Failed to unassign qgroup %d from parent %d: %v", qgroupID, parent, errno.Error())
		}
		// If the return value is > 0, we need to request a rescan
		if ret > 0 {
			var rargs C.struct_btrfs_ioctl_quota_rescan_args
			for {
				_, _, errno := unix.Syscall(unix.SYS_IOCTL, getDirFd(dir), C.BTRFS_IOC_QUOTA_RESCAN,
					uintptr(unsafe.Pointer(&rargs)))
				if errno != 0 {
					if errno == syscall.EINPROGRESS {
						if err := btrfsIocQuotaRescanWait(dir); err != nil {
							return err
						}
						continue
					}
					return fmt.Errorf("Failed to start btrfs quota rescan: %v", errno.Error())
				}
				break
			}
			if err := btrfsIocQuotaRescanWait(dir); err != nil {
				return err
			}
		}

		ID := btrfsQgroupIDSplit(parent)
		// The parent qgroupid shares the same id part with us? If so, destroy it too.
		if ID == subvolID {
			if err := btrfsQgroupDestroyRecursive(dir, parent); err != nil {
				return fmt.Errorf("Failed to destroy parent %d of qgroup %d: %v", parent, qgroupID, err)
			}
		}
	}

	// now delete qgroup
	var qcargs C.struct_btrfs_ioctl_qgroup_create_args
	qcargs.create = C.false
	qcargs.qgroupid = qgroupID
	_, _, errno := unix.Syscall(unix.SYS_IOCTL, getDirFd(dir), C.BTRFS_IOC_QGROUP_CREATE, uintptr(unsafe.Pointer(&qcargs)))
	if errno != 0 {
		return fmt.Errorf("Failed to delete btrfs qgroup %d: %v", qgroupID, errno.Error())
	}
	return nil
}

func subvolSetPropRO(path string, isReadOnly bool) error {
	dir, err := openDir(path)
	if err != nil {
		return err
	}
	defer closeDir(dir)

	var oldFlags, newFlags C.__u64
	_, _, errno := unix.Syscall(unix.SYS_IOCTL, getDirFd(dir), C.BTRFS_IOC_SUBVOL_GETFLAGS,
		uintptr(unsafe.Pointer(&oldFlags)))
	if errno != 0 {
		return fmt.Errorf("Failed to get btrfs subvolume flags for %s: %v", path, errno.Error())
	}

	if isReadOnly {
		newFlags = oldFlags | C.BTRFS_SUBVOL_RDONLY
	} else {
		newFlags = oldFlags &^ C.BTRFS_SUBVOL_RDONLY
	}

	if newFlags != oldFlags {
		_, _, errno = unix.Syscall(unix.SYS_IOCTL, getDirFd(dir), C.BTRFS_IOC_SUBVOL_SETFLAGS,
			uintptr(unsafe.Pointer(&newFlags)))
		if errno != 0 {
			return fmt.Errorf("Failed to set btrfs subvolume flags for %s: %v", path, errno.Error())
		}
	}
	return nil
}

func btrfsIocQuotaRescanWait(dir *C.DIR) error {
	ret, _, errno := unix.Syscall(unix.SYS_IOCTL, getDirFd(dir), C.BTRFS_IOC_QUOTA_RESCAN_WAIT, uintptr(0))
	if ret < 0 {
		return fmt.Errorf("Failed to wait for btrfs quota rescan: %v", errno.Error())
	}
	return nil
}

func (d *Driver) subvolEnableQuota() error {
	sdir := d.subvolumesDir()
	dir, err := openDir(sdir)
	if err != nil {
		return err
	}
	defer closeDir(dir)

	var args C.struct_btrfs_ioctl_quota_ctl_args
	args.cmd = C.BTRFS_QUOTA_CTL_ENABLE
	_, _, errno := unix.Syscall(unix.SYS_IOCTL, getDirFd(dir), C.BTRFS_IOC_QUOTA_CTL,
		uintptr(unsafe.Pointer(&args)))
	if errno != 0 {
		return fmt.Errorf("Failed to enable btrfs quota for %s: %v", sdir, errno.Error())
	}
	// If quotas was switched on then initial rescan is triggered so wait for it.
	return btrfsIocQuotaRescanWait(dir)
}

func subvolLimitQgroup(path string, size C.__u64) error {
	dir, err := openDir(path)
	if err != nil {
		return err
	}
	defer closeDir(dir)

	var args C.struct_btrfs_ioctl_qgroup_limit_args
	args.lim.max_referenced = size
	args.lim.flags = C.BTRFS_QGROUP_LIMIT_MAX_RFER
	args.qgroupid = 0 //To take the subvol as qgroup
	_, _, errno := unix.Syscall(unix.SYS_IOCTL, getDirFd(dir), C.BTRFS_IOC_QGROUP_LIMIT,
		uintptr(unsafe.Pointer(&args)))
	if errno != 0 {
		return fmt.Errorf("Failed to limit qgroup for %s: %v", path, errno.Error())
	}

	return nil
}

func (d *Driver) subvolumesDir() string {
	return path.Join(d.home, "subvolumes")
}

func (d *Driver) subvolumesDirID(id string) string {
	return path.Join(d.subvolumesDir(), id)
}

func (d *Driver) quotasDir() string {
	return path.Join(d.home, "quotas")
}

func (d *Driver) quotasDirID(id string) string {
	return path.Join(d.quotasDir(), id)
}

// CreateReadWrite creates a layer that is writable for use as a container
// file system.
func (d *Driver) CreateReadWrite(id, parent string, opts *graphdriver.CreateOpts) error {
	return d.Create(id, parent, opts)
}

// Create the filesystem with given id.
func (d *Driver) Create(id, parent string, opts *graphdriver.CreateOpts) error {
	// Let's check StorageOpt first
	// units.RAMInBytes return only int64 so we can use max value as flag
	storageSize := ^C.__u64(0)
	if opts != nil {
		for key, val := range opts.StorageOpt {
			key := strings.ToLower(key)
			switch key {
			case "size":
				size, err := units.RAMInBytes(val)
				if err != nil {
					return fmt.Errorf("btrfs: Failed to parse storage size %s: %v", val, err)
				}
				if size < 0 {
					return fmt.Errorf("btrfs: storage size can't be negative but got: %d", size)
				}
				storageSize = C.__u64(size)
				if d.options.minSpace > 0 && storageSize < d.options.minSpace {
					return fmt.Errorf("btrfs: storage size cannot be less than %d byte, but got %d for %s", d.options.minSpace, storageSize, id)
				}
			default:
				return fmt.Errorf("Unknown btrfs storage option %s", key)
			}
		}
	}

	subvolumes := d.subvolumesDir()
	rootUID, rootGID, err := idtools.GetRootUIDGID(d.uidMaps, d.gidMaps)
	if err != nil {
		return err
	}
	if err := idtools.MkdirAllAndChown(subvolumes, 0700, idtools.Identity{UID: rootUID, GID: rootGID}); err != nil {
		return err
	}
	parentDir := ""
	if parent != "" {
		parentDir = d.subvolumesDirID(parent)
		st, err := os.Stat(parentDir)
		if err != nil {
			return err
		}
		if !st.IsDir() {
			return fmt.Errorf("%s: not a directory", parentDir)
		}
	}
	if err := subvolSnapshot(parentDir, subvolumes, id); err != nil {
		return err
	}

	subvolumeDir := d.subvolumesDirID(id)

	// if we have a remapped root (user namespaces enabled), change the created snapshot
	// dir ownership to match
	if rootUID != 0 || rootGID != 0 {
		if err := os.Chown(subvolumeDir, rootUID, rootGID); err != nil {
			return err
		}
	}

	if opts != nil {
		if storageSize != ^C.__u64(0) {
			if err := idtools.MkdirAllAndChown(d.quotasDir(), 0700, idtools.IDPair{UID: rootUID, GID: rootGID}); err != nil {
				return err
			}
			subvolQuota := d.quotasDirID(id)
			if err := ioutil.WriteFile(subvolQuota, []byte(fmt.Sprint(storageSize)), 0644); err != nil {
				return err
			}
			if rootUID != 0 || rootGID != 0 {
				if err := os.Chown(subvolQuota, rootUID, rootGID); err != nil {
					return err
				}
			}
		}

		if err := label.Relabel(subvolumeDir, opts.MountLabel, false); err != nil {
			return err
		}
	}

	// For now we made all init changes and can turn subvolume into readonly mode
	return subvolSetPropRO(subvolumeDir, true)
}

// Remove the filesystem with given id.
func (d *Driver) Remove(id string) error {
	dir := d.subvolumesDirID(id)
	if _, err := os.Stat(dir); err != nil {
		return err
	}
	quotasDir := d.quotasDirID(id)
	if _, err := os.Stat(quotasDir); err == nil {
		if err := os.Remove(quotasDir); err != nil {
			return err
		}
	} else if !os.IsNotExist(err) {
		return err
	}

	if err := subvolDelete(d.subvolumesDir(), id, 0); err != nil {
		return err
	}
	return system.EnsureRemoveAll(dir)
}

// Get the requested filesystem id.
func (d *Driver) Get(id, mountLabel string) (containerfs.ContainerFS, error) {
	dir := d.subvolumesDirID(id)
	st, err := os.Stat(dir)
	if err != nil {
		return nil, err
	}

	if !st.IsDir() {
		return nil, fmt.Errorf("%s: not a directory", dir)
	}

	// Makes subvolume writable
	if err := subvolSetPropRO(dir, false); err != nil {
		return nil, err
	}

	if quota, err := ioutil.ReadFile(d.quotasDirID(id)); err == nil {
		size, err := strconv.ParseUint(string(quota), 10, 64)
		if err != nil {
			return nil, fmt.Errorf("Failed to parse quota size \"%s\" for %s: %v", string(quota), dir, err)
		}
		qsize := C.__u64(size)
		if d.options.minSpace > 0 && qsize < d.options.minSpace {
			return nil, fmt.Errorf("btrfs: storage size cannot be less than %d byte, but got %d for %s", d.options.minSpace, qsize, dir)
		}
		// Apply quota from here
		if err := d.subvolEnableQuota(); err != nil {
			return nil, err
		}
		if err := subvolLimitQgroup(dir, qsize); err != nil {
			return nil, err
		}
	}

	if mountLabel != "" {
		if err := label.SetFileLabel(dir, mountLabel); err != nil {
			return nil, err
		}
	}

	return containerfs.NewLocalContainerFS(dir), nil
}

// Put returns a subvolume to read-only state.
func (d *Driver) Put(id string) error {
	// Get() creates no runtime resources (like e.g. mounts),
	// it only makes a subvolume read-write; revert it here.
	return subvolSetPropRO(d.subvolumesDirID(id), true)
}

// Exists checks if the id exists in the filesystem.
func (d *Driver) Exists(id string) bool {
	dir := d.subvolumesDirID(id)
	_, err := os.Stat(dir)
	return err == nil
}
