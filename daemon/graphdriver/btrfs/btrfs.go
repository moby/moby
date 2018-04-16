//go:build linux
// +build linux

package btrfs // import "github.com/docker/docker/daemon/graphdriver/btrfs"

/*
#include <dirent.h>
#include <fcntl.h>
#include <unistd.h>
#include <btrfs/ctree.h>

static int copy_GoString_to_CString(char *dst, size_t dst_size, const _GoString_ src) {
    const size_t src_size = _GoStringLen(src);
    if (src_size >= dst_size) {
        errno = ENAMETOOLONG;
        return -1;
    }
    const char *psrc = _GoStringPtr(src);
    memcpy(dst, psrc, src_size);
    dst[src_size] = '\0';
    return src_size;
}

#define GOSTR_TO_CSTR(src, dst) copy_GoString_to_CString(dst, sizeof(dst), src)

static inline int set_name_btrfs_ioctl_vol_args(struct btrfs_ioctl_vol_args* btrfs_struct, const _GoString_ name) {
    return GOSTR_TO_CSTR(name, btrfs_struct->name);
}

static inline int set_name_btrfs_ioctl_vol_args_v2(struct btrfs_ioctl_vol_args_v2* btrfs_struct, const _GoString_ name) {
    return GOSTR_TO_CSTR(name, btrfs_struct->name);
}

static inline __u16 _le16toh(__le16 i) {return le16toh(i);}
static inline __u64 _le64toh(__le64 i) {return le64toh(i);}

static inline int _openat(int dirfd, const char *pathname, int flags) {return openat(dirfd, pathname, flags);}
*/
import "C"

import (
	"fmt"
	"math"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"unsafe"

	"github.com/containerd/containerd/pkg/userns"
	"github.com/docker/docker/daemon/graphdriver"
	"github.com/docker/docker/pkg/containerfs"
	"github.com/docker/docker/pkg/idtools"
	"github.com/docker/docker/pkg/parsers"
	units "github.com/docker/go-units"
	"github.com/moby/sys/mount"
	"github.com/opencontainers/selinux/go-selinux/label"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"golang.org/x/sys/unix"
)

func init() {
	graphdriver.Register("btrfs", Init)
}

type btrfsOptions struct {
	minSpace uint64
	size     uint64
}

// Init returns a new BTRFS driver.
// An error is returned if BTRFS is not supported.
func Init(home string, options []string, idMap idtools.IdentityMapping) (graphdriver.Driver, error) {
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

	currentID := idtools.CurrentIdentity()
	dirID := idtools.Identity{
		UID: currentID.UID,
		GID: idMap.RootPair().GID,
	}

	if err := idtools.MkdirAllAndChown(home, 0710, dirID); err != nil {
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
		idMap:   idMap,
		options: opt,
	}

	if userDiskQuota {
		if err := driver.enableQuota(); err != nil {
			return nil, err
		}
	}

	return graphdriver.NewNaiveDiffDriver(driver, driver.idMap), nil
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
			userDiskQuota = true
			options.minSpace = uint64(minSpace)
		default:
			return options, userDiskQuota, fmt.Errorf("Unknown option %s", key)
		}
	}
	return options, userDiskQuota, nil
}

// Driver contains information about the filesystem mounted.
type Driver struct {
	// root of the file system
	home         string
	idMap        idtools.IdentityMapping
	options      btrfsOptions
	quotaEnabled bool
	once         sync.Once
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
		status = append(status, [2]string{"Library Version", strconv.Itoa(lv)})
	}
	return status
}

// GetMetadata returns empty metadata for this driver.
func (d *Driver) GetMetadata(id string) (map[string]string, error) {
	return nil, nil
}

// Cleanup unmounts the home directory.
func (d *Driver) Cleanup() error {
	if err := mount.Unmount(d.home); err != nil {
		return err
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

	if r, err := C.set_name_btrfs_ioctl_vol_args_v2(&args, name); r < 0 {
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

func subvolDelete(dirpath, name string, subvolID C.__u64, quotaEnabled bool) error {
	dir, err := openDir(dirpath)
	if err != nil {
		return err
	}
	defer closeDir(dir)

	fullPath := path.Join(dirpath, name)

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
		// TODO: qgroup can have child groups, so it removing also should be recursive
		if quotaEnabled {
			var args C.struct_btrfs_ioctl_qgroup_create_args
			// for the leaf subvolumes, the qgroup id is identical to the subvol id
			args.qgroupid = subvolID
			_, _, errno := unix.Syscall(unix.SYS_IOCTL, getDirFd(dir), C.BTRFS_IOC_QGROUP_CREATE,
				uintptr(unsafe.Pointer(&args)))
			if errno != 0 {
				logrus.Errorf("Failed to delete btrfs qgroup %v for %s: %v", subvolID, fullPath, errno.Error())
			}
		}

		var args C.struct_btrfs_ioctl_vol_args
		if r, err := C.set_name_btrfs_ioctl_vol_args(&args, name); r < 0 {
			return err.(syscall.Errno)
		}
		_, _, errno := unix.Syscall(unix.SYS_IOCTL, getDirFd(dir), C.BTRFS_IOC_SNAP_DESTROY,
			uintptr(unsafe.Pointer(&args)))
		return errno
	}

	// First try to delete right now. Should be most common case
	if errno := destroySnap(); errno == 0 {
		return nil
	} else if errno != syscall.ENOTEMPTY {
		return fmt.Errorf("Failed to destroy btrfs subvolume %s: %v", fullPath, errno.Error())
	}
	// errno == ENOTEMPTY going to search nested subvols

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

	// while search key is still valid
	for btrfsIoctlSearchArgsCompare(&args) {
		// Get list of all subvolumes that is a child of this subvolume
		args.key.nr_items = 256
		_, _, errno := unix.Syscall(unix.SYS_IOCTL, getDirFd(dir), C.BTRFS_IOC_TREE_SEARCH,
			uintptr(unsafe.Pointer(&args)))
		if errno != 0 {
			return fmt.Errorf("Failed to search subvols for %s: %v", dirpath, errno.Error())
		}
		if args.key.nr_items == 0 {
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
				if err := subvolDelete(path.Join(fullPath, relPath), childName, sh.objectid, quotaEnabled); err != nil {
					return err
				}
			}
		}
		// Increase search key by one, to read the next item, if we can.
		if btrfsIoctlSearchArgsInc(&args) {
			break
		}
	}

	// If there's any subvolumes whose path is too long, process it now
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
							if err := subvolDelete(dpath, child.name, child.id, quotaEnabled); err != nil {
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
	return nil
}

func (d *Driver) updateQuotaStatus() {
	d.once.Do(func() {
		if !d.quotaEnabled {
			// In case quotaEnabled is not set, check qgroup and update quotaEnabled as needed
			if err := qgroupStatus(d.home); err != nil {
				// quota is still not enabled
				return
			}
			d.quotaEnabled = true
		}
	})
}

func (d *Driver) enableQuota() error {
	d.updateQuotaStatus()

	if d.quotaEnabled {
		return nil
	}

	dir, err := openDir(d.home)
	if err != nil {
		return err
	}
	defer closeDir(dir)

	var args C.struct_btrfs_ioctl_quota_ctl_args
	args.cmd = C.BTRFS_QUOTA_CTL_ENABLE
	_, _, errno := unix.Syscall(unix.SYS_IOCTL, getDirFd(dir), C.BTRFS_IOC_QUOTA_CTL,
		uintptr(unsafe.Pointer(&args)))
	if errno != 0 {
		return fmt.Errorf("Failed to enable btrfs quota for %s: %v", dir, errno.Error())
	}

	d.quotaEnabled = true

	return nil
}

func (d *Driver) subvolRescanQuota() error {
	d.updateQuotaStatus()

	if !d.quotaEnabled {
		return nil
	}

	dir, err := openDir(d.home)
	if err != nil {
		return err
	}
	defer closeDir(dir)

	var args C.struct_btrfs_ioctl_quota_rescan_args
	_, _, errno := unix.Syscall(unix.SYS_IOCTL, getDirFd(dir), C.BTRFS_IOC_QUOTA_RESCAN_WAIT,
		uintptr(unsafe.Pointer(&args)))
	if errno != 0 {
		return fmt.Errorf("Failed to rescan btrfs quota for %s: %v", dir, errno.Error())
	}

	return nil
}

func subvolLimitQgroup(path string, size uint64) error {
	dir, err := openDir(path)
	if err != nil {
		return err
	}
	defer closeDir(dir)

	var args C.struct_btrfs_ioctl_qgroup_limit_args
	args.lim.max_referenced = C.__u64(size)
	args.lim.flags = C.BTRFS_QGROUP_LIMIT_MAX_RFER
	_, _, errno := unix.Syscall(unix.SYS_IOCTL, getDirFd(dir), C.BTRFS_IOC_QGROUP_LIMIT,
		uintptr(unsafe.Pointer(&args)))
	if errno != 0 {
		return fmt.Errorf("Failed to limit qgroup for %s: %v", dir, errno.Error())
	}

	return nil
}

// qgroupStatus performs a BTRFS_IOC_TREE_SEARCH on the root path
// with search key of BTRFS_QGROUP_STATUS_KEY.
// In case qgroup is enabled, the retuned key type will match BTRFS_QGROUP_STATUS_KEY.
// For more details please see https://github.com/kdave/btrfs-progs/blob/v4.9/qgroup.c#L1035
func qgroupStatus(path string) error {
	dir, err := openDir(path)
	if err != nil {
		return err
	}
	defer closeDir(dir)

	var args C.struct_btrfs_ioctl_search_args
	args.key.tree_id = C.BTRFS_QUOTA_TREE_OBJECTID
	args.key.min_type = C.BTRFS_QGROUP_STATUS_KEY
	args.key.max_type = C.BTRFS_QGROUP_STATUS_KEY
	args.key.max_objectid = C.__u64(math.MaxUint64)
	args.key.max_offset = C.__u64(math.MaxUint64)
	args.key.max_transid = C.__u64(math.MaxUint64)
	args.key.nr_items = 4096

	_, _, errno := unix.Syscall(unix.SYS_IOCTL, getDirFd(dir), C.BTRFS_IOC_TREE_SEARCH,
		uintptr(unsafe.Pointer(&args)))
	if errno != 0 {
		return fmt.Errorf("Failed to search qgroup for %s: %v", path, errno.Error())
	}
	sh := (*C.struct_btrfs_ioctl_search_header)(unsafe.Pointer(&args.buf))
	if sh._type != C.BTRFS_QGROUP_STATUS_KEY {
		return fmt.Errorf("Invalid qgroup search header type for %s: %v", path, sh._type)
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
	quotas := d.quotasDir()
	subvolumes := d.subvolumesDir()
	root := d.idMap.RootPair()

	currentID := idtools.CurrentIdentity()
	dirID := idtools.Identity{
		UID: currentID.UID,
		GID: root.GID,
	}

	if err := idtools.MkdirAllAndChown(subvolumes, 0710, dirID); err != nil {
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

	var storageOpt map[string]string
	if opts != nil {
		storageOpt = opts.StorageOpt
	}

	subvolumeDir := d.subvolumesDirID(id)
	if _, ok := storageOpt["size"]; ok {
		driver := &Driver{}
		if err := d.parseStorageOpt(storageOpt, driver); err != nil {
			return err
		}

		if err := d.setStorageSize(subvolumeDir, driver); err != nil {
			return err
		}
		if err := idtools.MkdirAllAndChown(quotas, 0700, idtools.CurrentIdentity()); err != nil {
			return err
		}
		if err := os.WriteFile(d.quotasDirID(id), []byte(fmt.Sprint(driver.options.size)), 0644); err != nil {
			return err
		}
	}

	// if we have a remapped root (user namespaces enabled), change the created snapshot
	// dir ownership to match
	if root.UID != 0 || root.GID != 0 {
		if err := root.Chown(subvolumeDir); err != nil {
			return err
		}
	}

	mountLabel := ""
	if opts != nil {
		mountLabel = opts.MountLabel
	}

	return label.Relabel(subvolumeDir, mountLabel, false)
}

// Parse btrfs storage options
func (d *Driver) parseStorageOpt(storageOpt map[string]string, driver *Driver) error {
	// Read size to change the subvolume disk quota per container
	for key, val := range storageOpt {
		key := strings.ToLower(key)
		switch key {
		case "size":
			size, err := units.RAMInBytes(val)
			if err != nil {
				return err
			}
			driver.options.size = uint64(size)
		default:
			return fmt.Errorf("Unknown option %s", key)
		}
	}

	return nil
}

// Set btrfs storage size
func (d *Driver) setStorageSize(dir string, driver *Driver) error {
	if driver.options.size == 0 {
		return fmt.Errorf("btrfs: invalid storage size: %s", units.HumanSize(float64(driver.options.size)))
	}
	if d.options.minSpace > 0 && driver.options.size < d.options.minSpace {
		return fmt.Errorf("btrfs: storage size cannot be less than %s", units.HumanSize(float64(d.options.minSpace)))
	}
	if err := d.enableQuota(); err != nil {
		return err
	}
	return subvolLimitQgroup(dir, driver.options.size)
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

	// Call updateQuotaStatus() to invoke status update
	d.updateQuotaStatus()

	if err := subvolDelete(d.subvolumesDir(), id, 0, d.quotaEnabled); err != nil {
		if d.quotaEnabled {
			// use strings.Contains() rather than errors.Is(), because subvolDelete() does not use %w yet
			if userns.RunningInUserNS() && strings.Contains(err.Error(), "operation not permitted") {
				err = errors.Wrap(err, `failed to delete subvolume without root (hint: remount btrfs on "user_subvol_rm_allowed" option, or update the kernel to >= 4.18, or change the storage driver to "fuse-overlayfs")`)
			}
			return err
		}
		// If quota is not enabled, fallback to rmdir syscall to delete subvolumes.
		// This would allow unprivileged user to delete their owned subvolumes
		// in kernel >= 4.18 without user_subvol_rm_allowed mount option.
		//
		// From https://github.com/containers/storage/pull/508/commits/831e32b6bdcb530acc4c1cb9059d3c6dba14208c
	}
	if err := containerfs.EnsureRemoveAll(dir); err != nil {
		return err
	}
	return d.subvolRescanQuota()
}

// Get the requested filesystem id.
func (d *Driver) Get(id, mountLabel string) (string, error) {
	dir := d.subvolumesDirID(id)
	st, err := os.Stat(dir)
	if err != nil {
		return "", err
	}

	if !st.IsDir() {
		return "", fmt.Errorf("%s: not a directory", dir)
	}

	if quota, err := os.ReadFile(d.quotasDirID(id)); err == nil {
		if size, err := strconv.ParseUint(string(quota), 10, 64); err == nil && size >= d.options.minSpace {
			if err := d.enableQuota(); err != nil {
				return "", err
			}
			if err := subvolLimitQgroup(dir, size); err != nil {
				return "", err
			}
		}
	}

	return dir, nil
}

// Put is not implemented for BTRFS as there is no cleanup required for the id.
func (d *Driver) Put(id string) error {
	// Get() creates no runtime resources (like e.g. mounts),
	// so this doesn't need to do anything
	return nil
}

// Exists checks if the id exists in the filesystem.
func (d *Driver) Exists(id string) bool {
	dir := d.subvolumesDirID(id)
	_, err := os.Stat(dir)
	return err == nil
}
