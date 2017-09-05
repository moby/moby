// +build linux

//
// projectquota.go - implements XFS project quota controls
// for setting quota limits on a newly created directory.
// It currently supports the legacy XFS specific ioctls.
//
// TODO: use generic quota control ioctl FS_IOC_FS{GET,SET}XATTR
//       for both xfs/ext4 for kernel version >= v4.5
//

package quota

/*
#include <stdlib.h>
#include <dirent.h>
#include <linux/fs.h>
#include <linux/quota.h>
#include <linux/dqblk_xfs.h>

#ifndef FS_XFLAG_PROJINHERIT
struct fsxattr {
	__u32		fsx_xflags;
	__u32		fsx_extsize;
	__u32		fsx_nextents;
	__u32		fsx_projid;
	unsigned char	fsx_pad[12];
};
#define FS_XFLAG_PROJINHERIT	0x00000200
#endif
#ifndef FS_IOC_FSGETXATTR
#define FS_IOC_FSGETXATTR		_IOR ('X', 31, struct fsxattr)
#endif
#ifndef FS_IOC_FSSETXATTR
#define FS_IOC_FSSETXATTR		_IOW ('X', 32, struct fsxattr)
#endif

#ifndef PRJQUOTA
#define PRJQUOTA	2
#endif
#ifndef XFS_PROJ_QUOTA
#define XFS_PROJ_QUOTA	2
#endif
#ifndef Q_XSETPQLIM
#define Q_XSETPQLIM QCMD(Q_XSETQLIM, PRJQUOTA)
#endif
#ifndef Q_XGETPQUOTA
#define Q_XGETPQUOTA QCMD(Q_XGETQUOTA, PRJQUOTA)
#endif

const int Q_GETINFO_PRJQUOTA = QCMD(Q_GETINFO, PRJQUOTA);
*/
import "C"
import (
	"fmt"
	"io/ioutil"
	"path"
	"path/filepath"
	"unsafe"

	"encoding/json"
	"errors"

	"math"

	"github.com/sirupsen/logrus"
	"golang.org/x/sys/unix"
)

// ErrQuotaNotSupported indicates if were found the FS didn't have projects quotas available
var ErrQuotaNotSupported = errors.New("Filesystem does not support, or has not enabled quotas")

// ErrQuotasExhausted indicates that all project IDs are used on the filesystem
var ErrQuotasExhausted = errors.New("All project IDs have been exhausted")

var errNotOurProjectID = errors.New("Project ID set, but not by Docker")

const quotaXattr = "user.dockerprojectquota"

// currently this is empty, but we can populate it with quota metadata
type quotaStorage struct {
}

// Quota limit params - currently we only control blocks hard limit
type Quota struct {
	Size uint64
}

// Control - Context to be used by storage driver (e.g. overlay)
// who wants to apply project quotas to container dirs
type Control struct {
	backingFsBlockDev string
	nextProjectID     uint32
}

// NewControl - initialize project quota support.
// Test to make sure that quota can be set on a test dir and find
// the first project id to be used for the next container create.
//
// Returns nil (and error) if project quota is not supported.
//
// First get the project id of the home directory.
// This test will fail if the backing fs is not xfs.
//
// xfs_quota tool can be used to assign a project id to the driver home directory, e.g.:
//    echo 999:/var/lib/docker/overlay2 >> /etc/projects
//    echo docker:999 >> /etc/projid
//    xfs_quota -x -c 'project -s docker' /<xfs mount point>
//
// In that case, the home directory project id will be used as a "start offset"
// and all containers will be assigned larger project ids (e.g. >= 1000).
// This is a way to prevent xfs_quota management from conflicting with docker.
//
// Then try to create a test directory with the next project id and set a quota
// on it. If that works, continue to scan existing containers to map allocated
// project ids.
//
func NewControl(basePath string) (*Control, error) {
	//
	// Get project id of parent dir as minimal id to be used by driver
	//
	minProjectID, err := getProjectID(basePath)
	if err != nil {
		return nil, err
	}
	minProjectID++

	//
	// create backing filesystem device node
	//
	backingFsBlockDev, err := makeBackingFsDev(basePath)
	if err != nil {
		return nil, err
	}

	// check if we can call quotactl with project quotas
	// as a mechanism to determine (early) if we have support
	hasQuotaSupport, err := hasQuotaSupport(backingFsBlockDev)
	if err != nil {
		return nil, err
	}
	if !hasQuotaSupport {
		return nil, ErrQuotaNotSupported
	}

	//
	// Test if filesystem supports project quotas by trying to set
	// a quota on the first available project id
	//
	quota := Quota{
		Size: 0,
	}
	err = setProjectQuota(backingFsBlockDev, minProjectID, quota)
	if err != nil {
		return nil, err
	}

	q := Control{
		backingFsBlockDev: backingFsBlockDev,
		nextProjectID:     minProjectID + 1,
	}

	//
	// get first project id to be used for next container
	//
	err = q.findNextProjectID(basePath)
	if err != nil {
		return nil, err
	}

	logrus.Debugf("NewControl(%s): nextProjectID = %d", basePath, q.nextProjectID)
	return &q, nil
}

// SetQuota - assign a unique project id to directory and set the quota limits
// for that project id
func (q *Control) SetQuota(targetPath string, quota Quota) error {
	projectID, err := getProjectID(targetPath)
	if err != nil {
		return err
	}

	if projectID == 0 {
		projectID, err = q.getNextProjectID()
		if err != nil {
			return err
		}
		//
		// assign project id to new container directory
		//
		err = setProjectID(targetPath, projectID)
		if err != nil {
			return err
		}

		// Add requisite metadata to mark it as owned by Docker
		buf, err := json.Marshal(quotaStorage{})
		if err != nil {
			return err
		}
		err = unix.Setxattr(targetPath, quotaXattr, buf, 0)
		if err != nil {
			return err
		}

		q.nextProjectID = projectID + 1
	} else {
		dockerSet, err := isDockerSetProjectID(targetPath)
		if err != nil {
			return err
		}
		if !dockerSet {
			return errNotOurProjectID
		}
	}

	//
	// set the quota limit for the container's project id
	//
	logrus.Debugf("SetQuota(%s, %d): projectID=%d", targetPath, quota.Size, projectID)
	return setProjectQuota(q.backingFsBlockDev, projectID, quota)
}

func (q *Control) getNextProjectID() (uint32, error) {
	// Verify that this project ID has no limit set on it before using it
	checkedProjectIDs := 0
	projectIDToCheck := q.nextProjectID
	// TODO: Have a mechanism by which to determine whether 32-bit project IDs are safe
	for checkedProjectIDs < math.MaxUint16 {
		lim, err := getLimit(projectIDToCheck, q.backingFsBlockDev)
		// Alternatively, if there is no current files owned by this project ID, let's recycle it
		if err == unix.ESRCH || err == unix.ENOENT || lim.dIcount == 0 {
			return projectIDToCheck, nil
		}
		if err != nil {
			return 0, err
		}
		projectIDToCheck = (projectIDToCheck + 1) % math.MaxUint16
		checkedProjectIDs++
	}
	return 0, ErrQuotasExhausted
}

// setProjectQuota - set the quota for project id on xfs block device
func setProjectQuota(backingFsBlockDev string, projectID uint32, quota Quota) error {
	var d C.fs_disk_quota_t
	d.d_version = C.FS_DQUOT_VERSION
	d.d_id = C.__u32(projectID)
	d.d_flags = C.XFS_PROJ_QUOTA

	d.d_fieldmask = C.FS_DQ_BHARD | C.FS_DQ_BSOFT
	d.d_blk_hardlimit = C.__u64(quota.Size / 512)
	d.d_blk_softlimit = d.d_blk_hardlimit

	var cs = C.CString(backingFsBlockDev)
	defer C.free(unsafe.Pointer(cs))

	_, _, errno := unix.Syscall6(unix.SYS_QUOTACTL, C.Q_XSETPQLIM,
		uintptr(unsafe.Pointer(cs)), uintptr(d.d_id),
		uintptr(unsafe.Pointer(&d)), 0, 0)
	if errno != 0 {
		return fmt.Errorf("Failed to set quota limit for projid %d on %s: %v",
			projectID, backingFsBlockDev, errno.Error())
	}

	return nil
}

// GetQuota - get the quota limits of a directory that was configured with SetQuota
func (q *Control) GetQuota(targetPath string, quota *Quota) error {
	projectID, err := getProjectID(targetPath)
	if err != nil {
		return err
	}
	if projectID == 0 {
		return fmt.Errorf("quota not found for path : %s", targetPath)
	}

	dockerSet, err := isDockerSetProjectID(targetPath)
	if err != nil {
		return err
	}
	if !dockerSet {
		return errNotOurProjectID
	}

	lim, err := getLimit(projectID, q.backingFsBlockDev)
	quota.Size = lim.dBlkHardlimit * 512
	if err != nil {
		return fmt.Errorf("Failed to get quota limit for projid %d on %s: %v",
			projectID, q.backingFsBlockDev, err.Error())
	}

	return nil
}

// GetProjectID - Get the project ID for a given path
func (q *Control) GetProjectID(targetPath string) (uint32, error) {
	projectID, err := getProjectID(targetPath)
	if err != nil {
		return 0, err
	}
	if projectID == 0 {
		return 0, fmt.Errorf("quota not found for path : %s", targetPath)
	}

	dockerSet, err := isDockerSetProjectID(targetPath)
	if err != nil {
		return 0, err
	}
	if !dockerSet {
		return 0, errNotOurProjectID
	}

	return projectID, nil
}

type limit struct {
	dBlkHardlimit uint64 // absolute limit on disk blks
	dBlkSoftlimit uint64 // preferred limit on disk blks
	dInoHardlimit uint64 // maximum # allocated inodes
	dInoSoftlimit uint64 // preferred inode limit
	dBcount       uint64 // # disk blocks owned by the user
	dIcount       uint64 // # inodes owned by the user
}

func getLimit(projectID uint32, backingFsBlockDev string) (limit, error) {
	//
	// get the quota limit for the container's project id
	//
	var d C.fs_disk_quota_t

	var cs = C.CString(backingFsBlockDev)
	defer C.free(unsafe.Pointer(cs))

	_, _, errno := unix.Syscall6(unix.SYS_QUOTACTL, C.Q_XGETPQUOTA,
		uintptr(unsafe.Pointer(cs)), uintptr(C.__u32(projectID)),
		uintptr(unsafe.Pointer(&d)), 0, 0)
	if errno != 0 {
		return limit{}, errno
	}
	lim := limit{
		dBlkHardlimit: uint64(d.d_blk_hardlimit),
		dBlkSoftlimit: uint64(d.d_blk_softlimit),
		dInoHardlimit: uint64(d.d_ino_hardlimit),
		dInoSoftlimit: uint64(d.d_ino_softlimit),
		dBcount:       uint64(d.d_bcount),
		dIcount:       uint64(d.d_icount),
	}
	return lim, nil
}

// getProjectID - get the project id of path on xfs
func getProjectID(targetPath string) (uint32, error) {
	dir, err := openDir(targetPath)
	if err != nil {
		return 0, err
	}
	defer closeDir(dir)

	var fsx C.struct_fsxattr
	_, _, errno := unix.Syscall(unix.SYS_IOCTL, getDirFd(dir), C.FS_IOC_FSGETXATTR,
		uintptr(unsafe.Pointer(&fsx)))
	if errno != 0 {
		return 0, fmt.Errorf("Failed to get projid for %s: %v", targetPath, errno.Error())
	}

	return uint32(fsx.fsx_projid), nil
}

func isDockerSetProjectID(targetPath string) (bool, error) {
	buf := make([]byte, 4096)
	_, err := unix.Getxattr(targetPath, quotaXattr, buf)
	if err == unix.ENODATA {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

// setProjectID - set the project id of path on xfs
func setProjectID(targetPath string, projectID uint32) error {
	dir, err := openDir(targetPath)
	if err != nil {
		return err
	}
	defer closeDir(dir)

	var fsx C.struct_fsxattr
	_, _, errno := unix.Syscall(unix.SYS_IOCTL, getDirFd(dir), C.FS_IOC_FSGETXATTR,
		uintptr(unsafe.Pointer(&fsx)))
	if errno != 0 {
		return fmt.Errorf("Failed to get projid for %s: %v", targetPath, errno.Error())
	}

	if fsx.fsx_projid != 0 {
		dockerSet, err := isDockerSetProjectID(targetPath)
		if err != nil {
			return err
		}
		if !dockerSet {
			return errNotOurProjectID
		}
	}

	fsx.fsx_projid = C.__u32(projectID)
	fsx.fsx_xflags |= C.FS_XFLAG_PROJINHERIT
	_, _, errno = unix.Syscall(unix.SYS_IOCTL, getDirFd(dir), C.FS_IOC_FSSETXATTR,
		uintptr(unsafe.Pointer(&fsx)))
	if errno != 0 {
		return fmt.Errorf("Failed to set projid for %s: %v", targetPath, errno.Error())
	}

	return nil
}

// findNextProjectID - find the next project id to be used for containers
// by scanning driver home directory to find used project ids
func (q *Control) findNextProjectID(home string) error {
	files, err := ioutil.ReadDir(home)
	if err != nil {
		return fmt.Errorf("read directory failed : %s", home)
	}
	for _, file := range files {
		if !file.IsDir() {
			continue
		}
		path := filepath.Join(home, file.Name())
		projid, err := getProjectID(path)
		if err != nil {
			return err
		}
		if q.nextProjectID <= projid {
			q.nextProjectID = projid + 1
		}
	}

	return nil
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

// Get the backing block device of the driver home directory
// and create a block device node under the home directory
// to be used by quotactl commands
func makeBackingFsDev(home string) (string, error) {
	var stat unix.Stat_t
	if err := unix.Stat(home, &stat); err != nil {
		return "", err
	}

	backingFsBlockDev := path.Join(home, "backingFsBlockDev")
	// Re-create just in case someone copied the home directory over to a new device
	unix.Unlink(backingFsBlockDev)
	if err := unix.Mknod(backingFsBlockDev, unix.S_IFBLK|0600, int(stat.Dev)); err != nil {
		return "", fmt.Errorf("Failed to mknod %s: %v", backingFsBlockDev, err)
	}

	return backingFsBlockDev, nil
}

func hasQuotaSupport(backingFsBlockDev string) (bool, error) {
	var cs = C.CString(backingFsBlockDev)
	defer free(cs)
	var dqinfo C.struct_dqinfo

	_, _, errno := unix.Syscall6(unix.SYS_QUOTACTL, uintptr(C.Q_GETINFO_PRJQUOTA), uintptr(unsafe.Pointer(cs)), 0, uintptr(unsafe.Pointer(&dqinfo)), 0, 0)
	if errno == 0 {
		return true, nil
	}

	switch errno {
	// For this class of errors, return them directly to the user
	case unix.EFAULT:
	case unix.ENOENT:
	case unix.ENOTBLK:
	case unix.EPERM:
	default:
		return false, nil
	}

	return false, errno
}
