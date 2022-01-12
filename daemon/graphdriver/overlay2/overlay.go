//go:build linux
// +build linux

package overlay2 // import "github.com/docker/docker/daemon/graphdriver/overlay2"

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"github.com/docker/docker/daemon/graphdriver"
	"github.com/docker/docker/daemon/graphdriver/overlayutils"
	"github.com/docker/docker/pkg/archive"
	"github.com/docker/docker/pkg/chrootarchive"
	"github.com/docker/docker/pkg/containerfs"
	"github.com/docker/docker/pkg/directory"
	"github.com/docker/docker/pkg/fsutils"
	"github.com/docker/docker/pkg/idtools"
	"github.com/docker/docker/pkg/parsers"
	"github.com/docker/docker/pkg/system"
	"github.com/docker/docker/quota"
	units "github.com/docker/go-units"
	"github.com/moby/locker"
	"github.com/moby/sys/mount"
	"github.com/opencontainers/selinux/go-selinux/label"
	"github.com/sirupsen/logrus"
	"golang.org/x/sys/unix"
)

var (
	// untar defines the untar method
	untar = chrootarchive.UntarUncompressed
)

// This backend uses the overlay union filesystem for containers
// with diff directories for each layer.

// This version of the overlay driver requires at least kernel
// 4.0.0 in order to support mounting multiple diff directories.

// Each container/image has at least a "diff" directory and "link" file.
// If there is also a "lower" file when there are diff layers
// below as well as "merged" and "work" directories. The "diff" directory
// has the upper layer of the overlay and is used to capture any
// changes to the layer. The "lower" file contains all the lower layer
// mounts separated by ":" and ordered from uppermost to lowermost
// layers. The overlay itself is mounted in the "merged" directory,
// and the "work" dir is needed for overlay to work.

// The "link" file for each layer contains a unique string for the layer.
// Under the "l" directory at the root there will be a symbolic link
// with that unique string pointing the "diff" directory for the layer.
// The symbolic links are used to reference lower layers in the "lower"
// file and on mount. The links are used to shorten the total length
// of a layer reference without requiring changes to the layer identifier
// or root directory. Mounts are always done relative to root and
// referencing the symbolic links in order to ensure the number of
// lower directories can fit in a single page for making the mount
// syscall. A hard upper limit of 128 lower layers is enforced to ensure
// that mounts do not fail due to length.

const (
	driverName    = "overlay2"
	linkDir       = "l"
	diffDirName   = "diff"
	workDirName   = "work"
	mergedDirName = "merged"
	lowerFile     = "lower"
	maxDepth      = 128

	// idLength represents the number of random characters
	// which can be used to create the unique link identifier
	// for every layer. If this value is too long then the
	// page size limit for the mount command may be exceeded.
	// The idLength should be selected such that following equation
	// is true (512 is a buffer for label metadata).
	// ((idLength + len(linkDir) + 1) * maxDepth) <= (pageSize - 512)
	idLength = 26
)

type overlayOptions struct {
	overrideKernelCheck bool
	quota               quota.Quota
}

// Driver contains information about the home directory and the list of active
// mounts that are created using this driver.
type Driver struct {
	home          string
	uidMaps       []idtools.IDMap
	gidMaps       []idtools.IDMap
	ctr           *graphdriver.RefCounter
	quotaCtl      *quota.Control
	options       overlayOptions
	naiveDiff     graphdriver.DiffDriver
	supportsDType bool
	locker        *locker.Locker
}

var (
	logger                = logrus.WithField("storage-driver", "overlay2")
	backingFs             = "<unknown>"
	projectQuotaSupported = false

	useNaiveDiffLock sync.Once
	useNaiveDiffOnly bool

	indexOff  string
	userxattr string
)

func init() {
	graphdriver.Register(driverName, Init)
}

// Init returns the native diff driver for overlay filesystem.
// If overlay filesystem is not supported on the host, the error
// graphdriver.ErrNotSupported is returned.
// If an overlay filesystem is not supported over an existing filesystem then
// the error graphdriver.ErrIncompatibleFS is returned.
func Init(home string, options []string, uidMaps, gidMaps []idtools.IDMap) (graphdriver.Driver, error) {
	opts, err := parseOptions(options)
	if err != nil {
		return nil, err
	}

	// Perform feature detection on /var/lib/docker/overlay2 if it's an existing directory.
	// This covers situations where /var/lib/docker/overlay2 is a mount, and on a different
	// filesystem than /var/lib/docker.
	// If the path does not exist, fall back to using /var/lib/docker for feature detection.
	testdir := home
	if _, err := os.Stat(testdir); os.IsNotExist(err) {
		testdir = filepath.Dir(testdir)
	}

	if err := overlayutils.SupportsOverlay(testdir, true); err != nil {
		logger.Error(err)
		return nil, graphdriver.ErrNotSupported
	}

	fsMagic, err := graphdriver.GetFSMagic(testdir)
	if err != nil {
		return nil, err
	}
	if fsName, ok := graphdriver.FsNames[fsMagic]; ok {
		backingFs = fsName
	}

	supportsDType, err := fsutils.SupportsDType(testdir)
	if err != nil {
		return nil, err
	}
	if !supportsDType {
		if !graphdriver.IsInitialized(home) {
			return nil, overlayutils.ErrDTypeNotSupported("overlay2", backingFs)
		}
		// allow running without d_type only for existing setups (#27443)
		logger.Warn(overlayutils.ErrDTypeNotSupported("overlay2", backingFs))
	}

	_, rootGID, err := idtools.GetRootUIDGID(uidMaps, gidMaps)
	if err != nil {
		return nil, err
	}

	cur := idtools.CurrentIdentity()
	dirID := idtools.Identity{
		UID: cur.UID,
		GID: rootGID,
	}
	if err := idtools.MkdirAllAndChown(home, 0710, dirID); err != nil {
		return nil, err
	}
	if err := idtools.MkdirAllAndChown(path.Join(home, linkDir), 0700, cur); err != nil {
		return nil, err
	}

	d := &Driver{
		home:          home,
		uidMaps:       uidMaps,
		gidMaps:       gidMaps,
		ctr:           graphdriver.NewRefCounter(graphdriver.NewFsChecker(graphdriver.FsMagicOverlay)),
		supportsDType: supportsDType,
		locker:        locker.New(),
		options:       *opts,
	}

	d.naiveDiff = graphdriver.NewNaiveDiffDriver(d, uidMaps, gidMaps)

	if backingFs == "xfs" {
		// Try to enable project quota support over xfs.
		if d.quotaCtl, err = quota.NewControl(home); err == nil {
			projectQuotaSupported = true
		} else if opts.quota.Size > 0 {
			return nil, fmt.Errorf("Storage option overlay2.size not supported. Filesystem does not support Project Quota: %v", err)
		}
	} else if opts.quota.Size > 0 {
		// if xfs is not the backing fs then error out if the storage-opt overlay2.size is used.
		return nil, fmt.Errorf("Storage Option overlay2.size only supported for backingFS XFS. Found %v", backingFs)
	}

	// figure out whether "index=off" option is recognized by the kernel
	_, err = os.Stat("/sys/module/overlay/parameters/index")
	switch {
	case err == nil:
		indexOff = "index=off,"
	case os.IsNotExist(err):
		// old kernel, no index -- do nothing
	default:
		logger.Warnf("Unable to detect whether overlay kernel module supports index parameter: %s", err)
	}

	needsUserXattr, err := overlayutils.NeedsUserXAttr(home)
	if err != nil {
		logger.Warnf("Unable to detect whether overlay kernel module needs \"userxattr\" parameter: %s", err)
	}
	if needsUserXattr {
		userxattr = "userxattr,"
	}

	logger.Debugf("backingFs=%s, projectQuotaSupported=%v, indexOff=%q, userxattr=%q",
		backingFs, projectQuotaSupported, indexOff, userxattr)

	return d, nil
}

func parseOptions(options []string) (*overlayOptions, error) {
	o := &overlayOptions{}
	for _, option := range options {
		key, val, err := parsers.ParseKeyValueOpt(option)
		if err != nil {
			return nil, err
		}
		key = strings.ToLower(key)
		switch key {
		case "overlay2.override_kernel_check":
			o.overrideKernelCheck, err = strconv.ParseBool(val)
			if err != nil {
				return nil, err
			}
		case "overlay2.size":
			size, err := units.RAMInBytes(val)
			if err != nil {
				return nil, err
			}
			o.quota.Size = uint64(size)
		default:
			return nil, fmt.Errorf("overlay2: unknown option %s", key)
		}
	}
	return o, nil
}

func useNaiveDiff(home string) bool {
	useNaiveDiffLock.Do(func() {
		if err := doesSupportNativeDiff(home); err != nil {
			logger.Warnf("Not using native diff for overlay2, this may cause degraded performance for building images: %v", err)
			useNaiveDiffOnly = true
		}
	})
	return useNaiveDiffOnly
}

func (d *Driver) String() string {
	return driverName
}

// Status returns current driver information in a two dimensional string array.
// Output contains "Backing Filesystem" used in this implementation.
func (d *Driver) Status() [][2]string {
	return [][2]string{
		{"Backing Filesystem", backingFs},
		{"Supports d_type", strconv.FormatBool(d.supportsDType)},
		{"Native Overlay Diff", strconv.FormatBool(!useNaiveDiff(d.home))},
		{"userxattr", strconv.FormatBool(userxattr != "")},
	}
}

// GetMetadata returns metadata about the overlay driver such as the LowerDir,
// UpperDir, WorkDir, and MergeDir used to store data.
func (d *Driver) GetMetadata(id string) (map[string]string, error) {
	dir := d.dir(id)
	if _, err := os.Stat(dir); err != nil {
		return nil, err
	}

	metadata := map[string]string{
		"WorkDir":   path.Join(dir, workDirName),
		"MergedDir": path.Join(dir, mergedDirName),
		"UpperDir":  path.Join(dir, diffDirName),
	}

	lowerDirs, err := d.getLowerDirs(id)
	if err != nil {
		return nil, err
	}
	if len(lowerDirs) > 0 {
		metadata["LowerDir"] = strings.Join(lowerDirs, ":")
	}

	return metadata, nil
}

// Cleanup any state created by overlay which should be cleaned when daemon
// is being shutdown. For now, we just have to unmount the bind mounted
// we had created.
func (d *Driver) Cleanup() error {
	return mount.RecursiveUnmount(d.home)
}

// CreateReadWrite creates a layer that is writable for use as a container
// file system.
func (d *Driver) CreateReadWrite(id, parent string, opts *graphdriver.CreateOpts) error {
	if opts == nil {
		opts = &graphdriver.CreateOpts{
			StorageOpt: make(map[string]string),
		}
	} else if opts.StorageOpt == nil {
		opts.StorageOpt = make(map[string]string)
	}

	// Merge daemon default config.
	if _, ok := opts.StorageOpt["size"]; !ok && d.options.quota.Size != 0 {
		opts.StorageOpt["size"] = strconv.FormatUint(d.options.quota.Size, 10)
	}

	if _, ok := opts.StorageOpt["size"]; ok && !projectQuotaSupported {
		return fmt.Errorf("--storage-opt is supported only for overlay over xfs with 'pquota' mount option")
	}

	return d.create(id, parent, opts)
}

// Create is used to create the upper, lower, and merge directories required for overlay fs for a given id.
// The parent filesystem is used to configure these directories for the overlay.
func (d *Driver) Create(id, parent string, opts *graphdriver.CreateOpts) (retErr error) {
	if opts != nil && len(opts.StorageOpt) != 0 {
		if _, ok := opts.StorageOpt["size"]; ok {
			return fmt.Errorf("--storage-opt size is only supported for ReadWrite Layers")
		}
	}
	return d.create(id, parent, opts)
}

func (d *Driver) create(id, parent string, opts *graphdriver.CreateOpts) (retErr error) {
	dir := d.dir(id)

	rootUID, rootGID, err := idtools.GetRootUIDGID(d.uidMaps, d.gidMaps)
	if err != nil {
		return err
	}
	root := idtools.Identity{UID: rootUID, GID: rootGID}
	dirID := idtools.Identity{
		UID: idtools.CurrentIdentity().UID,
		GID: rootGID,
	}

	if err := idtools.MkdirAllAndChown(path.Dir(dir), 0710, dirID); err != nil {
		return err
	}
	if err := idtools.MkdirAndChown(dir, 0710, dirID); err != nil {
		return err
	}

	defer func() {
		// Clean up on failure
		if retErr != nil {
			os.RemoveAll(dir)
		}
	}()

	if opts != nil && len(opts.StorageOpt) > 0 {
		driver := &Driver{}
		if err := d.parseStorageOpt(opts.StorageOpt, driver); err != nil {
			return err
		}

		if driver.options.quota.Size > 0 {
			// Set container disk quota limit
			if err := d.quotaCtl.SetQuota(dir, driver.options.quota); err != nil {
				return err
			}
		}
	}

	if err := idtools.MkdirAndChown(path.Join(dir, diffDirName), 0755, root); err != nil {
		return err
	}

	lid := overlayutils.GenerateID(idLength, logger)
	if err := os.Symlink(path.Join("..", id, diffDirName), path.Join(d.home, linkDir, lid)); err != nil {
		return err
	}

	// Write link id to link file
	if err := os.WriteFile(path.Join(dir, "link"), []byte(lid), 0644); err != nil {
		return err
	}

	// if no parent directory, done
	if parent == "" {
		return nil
	}

	if err := idtools.MkdirAndChown(path.Join(dir, workDirName), 0700, root); err != nil {
		return err
	}

	if err := os.WriteFile(path.Join(d.dir(parent), "committed"), []byte{}, 0600); err != nil {
		return err
	}

	lower, err := d.getLower(parent)
	if err != nil {
		return err
	}
	if lower != "" {
		if err := os.WriteFile(path.Join(dir, lowerFile), []byte(lower), 0666); err != nil {
			return err
		}
	}

	return nil
}

// Parse overlay storage options
func (d *Driver) parseStorageOpt(storageOpt map[string]string, driver *Driver) error {
	// Read size to set the disk project quota per container
	for key, val := range storageOpt {
		key := strings.ToLower(key)
		switch key {
		case "size":
			size, err := units.RAMInBytes(val)
			if err != nil {
				return err
			}
			driver.options.quota.Size = uint64(size)
		default:
			return fmt.Errorf("Unknown option %s", key)
		}
	}

	return nil
}

func (d *Driver) getLower(parent string) (string, error) {
	parentDir := d.dir(parent)

	// Ensure parent exists
	if _, err := os.Lstat(parentDir); err != nil {
		return "", err
	}

	// Read Parent link fileA
	parentLink, err := os.ReadFile(path.Join(parentDir, "link"))
	if err != nil {
		return "", err
	}
	lowers := []string{path.Join(linkDir, string(parentLink))}

	parentLower, err := os.ReadFile(path.Join(parentDir, lowerFile))
	if err == nil {
		parentLowers := strings.Split(string(parentLower), ":")
		lowers = append(lowers, parentLowers...)
	}
	if len(lowers) > maxDepth {
		return "", errors.New("max depth exceeded")
	}
	return strings.Join(lowers, ":"), nil
}

func (d *Driver) dir(id string) string {
	return path.Join(d.home, id)
}

func (d *Driver) getLowerDirs(id string) ([]string, error) {
	var lowersArray []string
	lowers, err := os.ReadFile(path.Join(d.dir(id), lowerFile))
	if err == nil {
		for _, s := range strings.Split(string(lowers), ":") {
			lp, err := os.Readlink(path.Join(d.home, s))
			if err != nil {
				return nil, err
			}
			lowersArray = append(lowersArray, path.Clean(path.Join(d.home, linkDir, lp)))
		}
	} else if !os.IsNotExist(err) {
		return nil, err
	}
	return lowersArray, nil
}

// Remove cleans the directories that are created for this id.
func (d *Driver) Remove(id string) error {
	if id == "" {
		return fmt.Errorf("refusing to remove the directories: id is empty")
	}
	d.locker.Lock(id)
	defer d.locker.Unlock(id)
	dir := d.dir(id)
	lid, err := os.ReadFile(path.Join(dir, "link"))
	if err == nil {
		if len(lid) == 0 {
			logger.Errorf("refusing to remove empty link for layer %v", id)
		} else if err := os.RemoveAll(path.Join(d.home, linkDir, string(lid))); err != nil {
			logger.Debugf("Failed to remove link: %v", err)
		}
	}

	if err := system.EnsureRemoveAll(dir); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// Get creates and mounts the required file system for the given id and returns the mount path.
func (d *Driver) Get(id, mountLabel string) (_ containerfs.ContainerFS, retErr error) {
	d.locker.Lock(id)
	defer d.locker.Unlock(id)
	dir := d.dir(id)
	if _, err := os.Stat(dir); err != nil {
		return nil, err
	}

	diffDir := path.Join(dir, diffDirName)
	lowers, err := os.ReadFile(path.Join(dir, lowerFile))
	if err != nil {
		// If no lower, just return diff directory
		if os.IsNotExist(err) {
			return containerfs.NewLocalContainerFS(diffDir), nil
		}
		return nil, err
	}

	mergedDir := path.Join(dir, mergedDirName)
	if count := d.ctr.Increment(mergedDir); count > 1 {
		return containerfs.NewLocalContainerFS(mergedDir), nil
	}
	defer func() {
		if retErr != nil {
			if c := d.ctr.Decrement(mergedDir); c <= 0 {
				if mntErr := unix.Unmount(mergedDir, 0); mntErr != nil {
					logger.Errorf("error unmounting %v: %v", mergedDir, mntErr)
				}
				// Cleanup the created merged directory; see the comment in Put's rmdir
				if rmErr := unix.Rmdir(mergedDir); rmErr != nil && !os.IsNotExist(rmErr) {
					logger.Debugf("Failed to remove %s: %v: %v", id, rmErr, err)
				}
			}
		}
	}()

	workDir := path.Join(dir, workDirName)
	splitLowers := strings.Split(string(lowers), ":")
	absLowers := make([]string, len(splitLowers))
	for i, s := range splitLowers {
		absLowers[i] = path.Join(d.home, s)
	}
	var readonly bool
	if _, err := os.Stat(path.Join(dir, "committed")); err == nil {
		readonly = true
	} else if !os.IsNotExist(err) {
		return nil, err
	}

	var opts string
	if readonly {
		opts = indexOff + userxattr + "lowerdir=" + diffDir + ":" + strings.Join(absLowers, ":")
	} else {
		opts = indexOff + userxattr + "lowerdir=" + strings.Join(absLowers, ":") + ",upperdir=" + diffDir + ",workdir=" + workDir
	}

	mountData := label.FormatMountLabel(opts, mountLabel)
	mount := unix.Mount
	mountTarget := mergedDir

	rootUID, rootGID, err := idtools.GetRootUIDGID(d.uidMaps, d.gidMaps)
	if err != nil {
		return nil, err
	}
	if err := idtools.MkdirAndChown(mergedDir, 0700, idtools.Identity{UID: rootUID, GID: rootGID}); err != nil {
		return nil, err
	}

	pageSize := unix.Getpagesize()

	// Use relative paths and mountFrom when the mount data has exceeded
	// the page size. The mount syscall fails if the mount data cannot
	// fit within a page and relative links make the mount data much
	// smaller at the expense of requiring a fork exec to chroot.
	if len(mountData) > pageSize-1 {
		if readonly {
			opts = indexOff + userxattr + "lowerdir=" + path.Join(id, diffDirName) + ":" + string(lowers)
		} else {
			opts = indexOff + userxattr + "lowerdir=" + string(lowers) + ",upperdir=" + path.Join(id, diffDirName) + ",workdir=" + path.Join(id, workDirName)
		}
		mountData = label.FormatMountLabel(opts, mountLabel)
		if len(mountData) > pageSize-1 {
			return nil, fmt.Errorf("cannot mount layer, mount label too large %d", len(mountData))
		}

		mount = func(source string, target string, mType string, flags uintptr, label string) error {
			return mountFrom(d.home, source, target, mType, flags, label)
		}
		mountTarget = path.Join(id, mergedDirName)
	}

	if err := mount("overlay", mountTarget, "overlay", 0, mountData); err != nil {
		return nil, fmt.Errorf("error creating overlay mount to %s: %v", mergedDir, err)
	}

	if !readonly {
		// chown "workdir/work" to the remapped root UID/GID. Overlay fs inside a
		// user namespace requires this to move a directory from lower to upper.
		if err := os.Chown(path.Join(workDir, workDirName), rootUID, rootGID); err != nil {
			return nil, err
		}
	}

	return containerfs.NewLocalContainerFS(mergedDir), nil
}

// Put unmounts the mount path created for the give id.
// It also removes the 'merged' directory to force the kernel to unmount the
// overlay mount in other namespaces.
func (d *Driver) Put(id string) error {
	d.locker.Lock(id)
	defer d.locker.Unlock(id)
	dir := d.dir(id)
	_, err := os.ReadFile(path.Join(dir, lowerFile))
	if err != nil {
		// If no lower, no mount happened and just return directly
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	mountpoint := path.Join(dir, mergedDirName)
	if count := d.ctr.Decrement(mountpoint); count > 0 {
		return nil
	}
	if err := unix.Unmount(mountpoint, unix.MNT_DETACH); err != nil {
		logger.Debugf("Failed to unmount %s overlay: %s - %v", id, mountpoint, err)
	}
	// Remove the mountpoint here. Removing the mountpoint (in newer kernels)
	// will cause all other instances of this mount in other mount namespaces
	// to be unmounted. This is necessary to avoid cases where an overlay mount
	// that is present in another namespace will cause subsequent mounts
	// operations to fail with ebusy.  We ignore any errors here because this may
	// fail on older kernels which don't have
	// torvalds/linux@8ed936b5671bfb33d89bc60bdcc7cf0470ba52fe applied.
	if err := unix.Rmdir(mountpoint); err != nil && !os.IsNotExist(err) {
		logger.Debugf("Failed to remove %s overlay: %v", id, err)
	}
	return nil
}

// Exists checks to see if the id is already mounted.
func (d *Driver) Exists(id string) bool {
	_, err := os.Stat(d.dir(id))
	return err == nil
}

// isParent determines whether the given parent is the direct parent of the
// given layer id
func (d *Driver) isParent(id, parent string) bool {
	lowers, err := d.getLowerDirs(id)
	if err != nil {
		return false
	}
	if parent == "" && len(lowers) > 0 {
		return false
	}

	parentDir := d.dir(parent)
	var ld string
	if len(lowers) > 0 {
		ld = filepath.Dir(lowers[0])
	}
	if ld == "" && parent == "" {
		return true
	}
	return ld == parentDir
}

// ApplyDiff applies the new layer into a root
func (d *Driver) ApplyDiff(id string, parent string, diff io.Reader) (size int64, err error) {
	if useNaiveDiff(d.home) || !d.isParent(id, parent) {
		return d.naiveDiff.ApplyDiff(id, parent, diff)
	}

	// never reach here if we are running in UserNS
	applyDir := d.getDiffPath(id)

	logger.Debugf("Applying tar in %s", applyDir)
	// Overlay doesn't need the parent id to apply the diff
	if err := untar(diff, applyDir, &archive.TarOptions{
		UIDMaps:        d.uidMaps,
		GIDMaps:        d.gidMaps,
		WhiteoutFormat: archive.OverlayWhiteoutFormat,
	}); err != nil {
		return 0, err
	}

	return directory.Size(context.TODO(), applyDir)
}

func (d *Driver) getDiffPath(id string) string {
	dir := d.dir(id)

	return path.Join(dir, diffDirName)
}

// DiffSize calculates the changes between the specified id
// and its parent and returns the size in bytes of the changes
// relative to its base filesystem directory.
func (d *Driver) DiffSize(id, parent string) (size int64, err error) {
	if useNaiveDiff(d.home) || !d.isParent(id, parent) {
		return d.naiveDiff.DiffSize(id, parent)
	}
	return directory.Size(context.TODO(), d.getDiffPath(id))
}

// Diff produces an archive of the changes between the specified
// layer and its parent layer which may be "".
func (d *Driver) Diff(id, parent string) (io.ReadCloser, error) {
	if useNaiveDiff(d.home) || !d.isParent(id, parent) {
		return d.naiveDiff.Diff(id, parent)
	}

	// never reach here if we are running in UserNS
	diffPath := d.getDiffPath(id)
	logger.Debugf("Tar with options on %s", diffPath)
	return archive.TarWithOptions(diffPath, &archive.TarOptions{
		Compression:    archive.Uncompressed,
		UIDMaps:        d.uidMaps,
		GIDMaps:        d.gidMaps,
		WhiteoutFormat: archive.OverlayWhiteoutFormat,
	})
}

// Changes produces a list of changes between the specified layer and its
// parent layer. If parent is "", then all changes will be ADD changes.
func (d *Driver) Changes(id, parent string) ([]archive.Change, error) {
	return d.naiveDiff.Changes(id, parent)
}
