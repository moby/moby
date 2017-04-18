// +build linux

package sharedloop

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"

	"github.com/Sirupsen/logrus"

	"github.com/docker/distribution/digest"
	"github.com/docker/docker/daemon/graphdriver"
	"github.com/docker/docker/daemon/graphdriver/overlayutils"
	"github.com/docker/docker/daemon/graphdriver/quota"
	"github.com/docker/docker/pkg/archive"
	"github.com/docker/docker/pkg/chrootarchive"
	"github.com/docker/docker/pkg/fsutils"
	"github.com/docker/docker/pkg/idtools"
	"github.com/docker/docker/pkg/loopback"
	"github.com/docker/docker/pkg/mount"
	"github.com/docker/docker/pkg/parsers"
	"github.com/docker/docker/pkg/parsers/kernel"
	"github.com/docker/go-units"

	"github.com/opencontainers/runc/libcontainer/label"
)

var (
	// untar defines the untar method
	untar = chrootarchive.UntarUncompressed
)

// This backend uses the overlay union filesystem for containers
// with diff directories for each layer stored in loopback file images.
// The loopback files are either xfs or ext4 images, where xfs is chosen
// if supported by the host.

// This driver makes use of kernel capabilities in 4.0.0 or layer for
// mounting multiple diff directories. This requirement is sharedloop with
// the overlay2 driver.

// There are two (configurable) locations where images are stored.
// The first is "loopback_root" which could be a directory on sharedloop
// storage and mounted by many nodes running Docker Engine. If during
// an image pull, this directory cannot be opened for writing (permission
// denied, read-only filesystem), then the graphdriver will fallback to
// "loopback_fallback". This directory could be on node-local storage
// or another directory on sharedloop storage. This location should only be
// used by a single Docker Engine or the user should be aware of file
// consistency implications if on a sharedloop mount. This graphdriver does
// not implement its own locking mechanisms.

// Each container/image has loopback image stored either in "loopback_root"
// or "loopback_fallback" where if the driver finds the image in
// "loopback_root", it will use that image and not check
// "loopback_fallback". The images are mounted on-demand (read-write for
// ApplyDiff and read-only for Get) with mountpoints in the conventional
// docker root directory tree. The image will have at least a "diff"
// directory and "link" file. If there is also a "lower" file when there
// are diff layers below as well as "merged" and "work" directories.
// The "diff" directory has the upper layer of the overlay and is used to
// capture any changes to the layer. The "lower" file contains all the lower
// layer mounts separated by ":" and ordered from uppermost to lowermost
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
	driverName = "sharedloop"
	linkDir    = "l"
	lowerFile  = "lower"
	maxDepth   = 128

	// idLength represents the number of random characters
	// which can be used to create the unique link identifer
	// for every layer. If this value is too long then the
	// page size limit for the mount command may be exceeded.
	// The idLength should be selected such that following equation
	// is true (512 is a buffer for label metadata).
	// ((idLength + len(linkDir) + 1) * maxDepth) <= (pageSize - 512)
	idLength = 26
)

type repository map[string]digest.Digest

type store struct {
	mu sync.RWMutex
	// jsonPath is the path to the file where the serialized tag data is
	// stored.
	jsonPath string
	// Repositories is a map of repositories, indexed by name.
	Repositories map[string]repository
}

type overlayOptions struct {
	overrideKernelCheck bool
	quota               quota.Quota
	loopbackRoot        string
	loopbackFallback    string
	defaultStoreDir     string
}

// Driver contains information about the home directory and the list of active mounts that are created using this driver.
type Driver struct {
	loopbackRoot     string
	loopbackFallback string
	defaultStoreDir  string
	filesystem       string
	mkfsArgs         []string
	home             string
	uidMaps          []idtools.IDMap
	gidMaps          []idtools.IDMap
	ctr              *graphdriver.RefCounter
	quotaCtl         *quota.Control
	options          overlayOptions
	naiveDiff        graphdriver.DiffDriver
	supportsDType    bool
}

var (
	backingFs             = "<unknown>"
	projectQuotaSupported = false
	loopbackRoot          = "/var/lib/docker/loopback/root"
	loopbackFallback      = "/var/lib/docker/loopback/fallback"
	defaultStoreDir       = "/var/lib/docker/image"

	mkfsArgs         []string
	useNaiveDiffLock sync.Once
	useNaiveDiffOnly bool
)

func init() {
	graphdriver.Register(driverName, Init)
}

// Init returns the a native diff driver for overlay filesystem.
// If overlay filesystem is not supported on the host, graphdriver.ErrNotSupported is returned as error.
// If an overlay filesystem is not supported over an existing filesystem then error graphdriver.ErrIncompatibleFS is returned.
func Init(home string, options []string, uidMaps, gidMaps []idtools.IDMap) (graphdriver.Driver, error) {
	var storeImgDirs []string
	var skipFallbackStore = false
	var needRWFallbackStore = false

	opts, err := parseOptions(options)
	if err != nil {
		return nil, err
	}

	if err := supportsOverlay(); err != nil {
		return nil, graphdriver.ErrNotSupported
	}

	// require kernel 4.0.0 to ensure multiple lower dirs are supported
	v, err := kernel.GetKernelVersion()
	if err != nil {
		return nil, err
	}
	if kernel.CompareKernelVersion(*v, kernel.VersionInfo{Kernel: 4, Major: 0, Minor: 0}) < 0 {
		if !opts.overrideKernelCheck {
			return nil, graphdriver.ErrNotSupported
		}
		logrus.Warn("Using pre-4.0.0 kernel for overlay2, mount failures may require kernel update")
	}

	fsMagic, err := graphdriver.GetFSMagic(home)
	if err != nil {
		return nil, err
	}
	if fsName, ok := graphdriver.FsNames[fsMagic]; ok {
		backingFs = fsName
	}

	// Create the driver home dir
	rootUID, rootGID, err := idtools.GetRootUIDGID(uidMaps, gidMaps)
	if err != nil {
		return nil, err
	}
	if err := idtools.MkdirAllAs(path.Join(home, linkDir), 0700, rootUID, rootGID); err != nil && !os.IsExist(err) {
		return nil, err
	}

	if err := mount.MakePrivate(home); err != nil {
		return nil, err
	}

	filesystem := determineDefaultFS()

	supportsDType, err := fsutils.SupportsDType(home)
	if err != nil {
		return nil, err
	}
	if !supportsDType {
		// not a fatal error until v1.16 (#27443)
		logrus.Warn(overlayutils.ErrDTypeNotSupported("overlay2", backingFs))
	}

	// From https://github.com/docker/docker/issues/27358
	mkfsArgs = append(mkfsArgs, "-n")
	mkfsArgs = append(mkfsArgs, "ftype=1")

	if len(opts.loopbackRoot) > 0 {
		loopbackRoot = opts.loopbackRoot
	}

	if len(opts.loopbackFallback) > 0 {
		loopbackFallback = opts.loopbackFallback
	}

	if len(opts.defaultStoreDir) > 0 {
		defaultStoreDir = opts.defaultStoreDir
	}

	d := &Driver{
		home:             home,
		loopbackRoot:     loopbackRoot,
		loopbackFallback: loopbackFallback,
		defaultStoreDir:  defaultStoreDir,
		filesystem:       filesystem,
		mkfsArgs:         mkfsArgs,
		uidMaps:          uidMaps,
		gidMaps:          gidMaps,
		ctr:              graphdriver.NewRefCounter(graphdriver.NewFsChecker(graphdriver.FsMagicOverlay)),
		supportsDType:    supportsDType,
	}

	d.naiveDiff = graphdriver.NewNaiveDiffDriver(d, uidMaps, gidMaps)

	storeImgDir := path.Join(loopbackRoot, "image-metadata")

	f, err := os.Open(path.Join(storeImgDir, "img"))
	if err == nil {
		// There's an existing store. Use it as the lower dir for overlay mount
		defer f.Close()
		storeImgDirs = append(storeImgDirs, storeImgDir)

		if tf, err := ioutil.TempFile(storeImgDir, ".touch"); err == nil {
			defer os.Remove(tf.Name())

			// We have write access to the root loopback directory, so don't
			// bother with fallback
			skipFallbackStore = true
		} else {

			// Store at root loopback directory exists, but it's not writable. We
			// need the fallback store directory to be writable
			needRWFallbackStore = true
		}
	} else if os.IsNotExist(err) {
		// Try to create image metadata stores
		if err := idtools.MkdirAllAs(storeImgDir, 0700, rootUID, rootGID); err == nil {
			if err := d.createStores(storeImgDir); err == nil {
				storeImgDirs = append(storeImgDirs, storeImgDir)

				// We have write access to the root loopback directory, so don't
				// bother with fallback
				skipFallbackStore = true
			} else {
				// Failed to create store, cleanup the dir just created
				os.Remove(storeImgDir)
			}
		}
	}

	// If store was created at root loopback path, then len(storeImgDirs) is 1, so move on to fallback
	if !skipFallbackStore {
		storeImgDir = path.Join(loopbackFallback, "image-metadata")

		f, err = os.Open(path.Join(storeImgDir, "img"))
		if err == nil {
			defer f.Close()
			// Make sure filesystem is writable
			if tf, err := ioutil.TempFile(storeImgDir, ".touch"); err == nil {
				defer os.Remove(tf.Name())
				storeImgDirs = append(storeImgDirs, storeImgDir)
			}
		} else if os.IsNotExist(err) {
			// Try to create image metadata stores
			if err := idtools.MkdirAllAs(storeImgDir, 0700, rootUID, rootGID); err == nil {
				if err := d.createStores(storeImgDir); err == nil {
					storeImgDirs = append(storeImgDirs, storeImgDir)
				} else {
					// Failed to create store, cleanup the dir just created
					os.Remove(storeImgDir)
				}
			}
		}
	}

	if len(storeImgDirs) == 0 ||
		(needRWFallbackStore && (len(storeImgDirs) == 1)) {
		return nil, fmt.Errorf("%s: neither loopback_root or loopback_fallback are writable. No usable path for image metadata", driverName)
	}

	// Merge all stores on top of default store directory using overlayfs or bind mount
	if err := d.mergeStores(storeImgDirs); err != nil {
		return nil, err
	}

	if backingFs == "xfs" {
		// Try to enable project quota support over xfs.
		if d.quotaCtl, err = quota.NewControl(home); err == nil {
			projectQuotaSupported = true
		}
	}

	logrus.Debugf("backingFs=%s,  projectQuotaSupported=%v", backingFs, projectQuotaSupported)

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
		case "sharedloop.override_kernel_check":
			o.overrideKernelCheck, err = strconv.ParseBool(val)
			if err != nil {
				return nil, err
			}

		case "sharedloop.loopback_root":
			o.loopbackRoot = val
		case "sharedloop.loopback_fallback":
			o.loopbackFallback = val
		case "sharedloop.default_storedir":
			o.defaultStoreDir = val
		default:
			return nil, fmt.Errorf("%s: Unknown option %s\n", driverName, key)
		}
	}

	return o, nil
}

func supportsOverlay() error {
	// We can try to modprobe overlay first before looking at
	// proc/filesystems for when overlay is supported
	exec.Command("modprobe", "overlay").Run()

	f, err := os.Open("/proc/filesystems")
	if err != nil {
		return err
	}
	defer f.Close()

	s := bufio.NewScanner(f)
	for s.Scan() {
		if s.Text() == "nodev\toverlay" {
			return nil
		}
	}
	logrus.Error("'overlay' not found as a supported filesystem on this host. Please ensure kernel is new enough and has overlay support loaded.")
	return graphdriver.ErrNotSupported
}

func useNaiveDiff(home string) bool {
	useNaiveDiffLock.Do(func() {
		if err := hasOpaqueCopyUpBug(home); err != nil {
			logrus.Warnf("Not using native diff for sharedloop: %v", err)
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
		{"Loopback Root Directory", loopbackRoot},
		{"Loopback Fallback Directory", loopbackFallback},
	}
}

// GetMetadata returns meta data about the overlay driver such as
// LowerDir, UpperDir, WorkDir and MergeDir used to store data.
func (d *Driver) GetMetadata(id string) (map[string]string, error) {

	dir := d.dir(id)
	if _, err := os.Stat(dir); err != nil {
		return nil, err
	}

	metadata := map[string]string{
		"WorkDir":   path.Join(dir, "work"),
		"MergedDir": path.Join(dir, "merged"),
		"UpperDir":  path.Join(dir, "diff"),
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
// is being shutdown.
func (d *Driver) Cleanup() error {
	// All mountpoints should be unmounted immediately, so do not call unmount helpers
	// which use syscall.MNT_DETACH

	// Unmount store overlay at default location
	mount.Unmount(path.Join(d.defaultStoreDir, driverName))

	// Unmount fallback store
	mount.Unmount(path.Join(loopbackFallback, "image-metadata", "loopmount"))

	// Unmount root store
	mount.Unmount(path.Join(loopbackRoot, "image-metadata", "loopmount"))

	// Unmount root for graphdriver
	return mount.Unmount(d.home)
}

// CreateReadWrite creates a layer that is writable for use as a container
// file system.
func (d *Driver) CreateReadWrite(id, parent string, opts *graphdriver.CreateOpts) error {
	return d.Create(id, parent, opts)
}

// createParent gets called by Create when parent layer doesn't exist. This may happen
// if the /var/lib/docker/sharedloop/ directory is empty.  No problem though, as we can use
// Create to recreate the layers from the loopback devices in /var/lib/docker/loopback/.
func (d *Driver) createParent(id string, opts *graphdriver.CreateOpts) error {
	var parent string

	dir := d.dir(id)
	if _, err := os.Lstat(dir); err != nil {
		parentFile, err := d.getParentMetaFile(id)
		if err != nil {
			return err
		}

		_, err = os.Stat(parentFile)
		if err == nil {
			parentBytes, err := ioutil.ReadFile(parentFile)
			if err != nil {
				return err
			}

			parent = string(parentBytes)
			if err = d.createParent(parent, opts); err != nil {
				return fmt.Errorf("%s: unable to recursively recreate layer %s: %v\n", driverName, parent, err)
			}
		} else if os.IsExist(err) {
			// Parent file no present means we are at the end of the recursion loop
			parent = ""
		}

		// Now actually create the layer. Due to recursive code above, our parent will already have called this
		if err := d.Create(id, parent, opts); err != nil {
			return fmt.Errorf("%s: unable to create layer %s\n", driverName, parent)
		}
	}

	return nil
}

// Create is used to create the upper, lower, and merge directories required for overlay fs for a given id.
// The parent filesystem is used to configure these directories for the overlay.
func (d *Driver) Create(id, parent string, opts *graphdriver.CreateOpts) (retErr error) {
	if opts != nil && len(opts.StorageOpt) != 0 && !projectQuotaSupported {
		return fmt.Errorf("--storage-opt is supported only for overlay over xfs with 'pquota' mount option")
	}

	dir := d.dir(id)

	rootUID, rootGID, err := idtools.GetRootUIDGID(d.uidMaps, d.gidMaps)
	if err != nil {
		return err
	}
	if err := idtools.MkdirAllAs(path.Dir(dir), 0700, rootUID, rootGID); err != nil {
		return err
	}
	if err := idtools.MkdirAs(dir, 0700, rootUID, rootGID); err != nil {
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

	if err := idtools.MkdirAs(path.Join(dir, "diff"), 0755, rootUID, rootGID); err != nil {
		return err
	}

	lid := generateID(idLength)
	if err := os.Symlink(path.Join("..", id, "diff"), path.Join(d.home, linkDir, lid)); err != nil {
		return err
	}

	// Write link id to link file
	if err := ioutil.WriteFile(path.Join(dir, "link"), []byte(lid), 0644); err != nil {
		return err
	}

	// if no parent directory, done
	if parent == "" {
		return nil
	}

	parentDir := d.dir(parent)

	// Ensure parent exists
	if _, err := os.Lstat(parentDir); err != nil {
		// This parent doesn't exist, but before we can recreate it, we have to recreate all of its
		// lower dirs
		logrus.Debugf("%s: parentDir %s does not exist. Creating its ancestors first", driverName, parentDir)
		if err = d.createParent(parent, opts); err != nil {
			return err
		}
	}

	if err := idtools.MkdirAs(path.Join(dir, "work"), 0700, rootUID, rootGID); err != nil {
		return err
	}
	if err := idtools.MkdirAs(path.Join(dir, "merged"), 0700, rootUID, rootGID); err != nil {
		return err
	}

	lower, err := d.getLower(parent)
	if err != nil {
		return err
	}
	if lower != "" {
		if err := ioutil.WriteFile(path.Join(dir, lowerFile), []byte(lower), 0666); err != nil {
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

// findExistingLoopbackPath will return the path of the loopback device in order
// of preference under either directory:
//   1. loopbackRoot
//   2. loopbackFallback
// If the file exists, then it returns the path found in the preference order.
// Returns "" when neither path exists
func (d *Driver) findExistingLoopbackPath(id string) string {
	var retPath string

	// Check at loopbackRoot
	retPath = path.Join(d.loopbackRoot, id)
	_, err := os.Stat(retPath)
	if err == nil {
		return retPath
	}

	// Check at loopbackFallback
	retPath = path.Join(d.loopbackFallback, id)
	_, err = os.Stat(retPath)
	if err == nil {
		return retPath
	}

	return ""
}

// findNewLoopbackPath returns the first writable location
func (d *Driver) findNewLoopbackPath(id string) (string, error) {
	var retPath string
	var err error
	var f *os.File

	// Check at loopbackRoot
	f, err = ioutil.TempFile(d.loopbackRoot, ".touch")
	if err == nil {
		defer os.Remove(f.Name())
		return path.Join(d.loopbackRoot, id), nil
	}

	// Check at loopbackFallback
	f, err = ioutil.TempFile(d.loopbackFallback, ".touch")
	if err == nil {
		defer os.Remove(f.Name())
		return path.Join(d.loopbackFallback, id), nil
	}

	// Return empty string
	err = fmt.Errorf("%s: could not find a writeable path for loopback device storage", driverName)
	return retPath, err
}

func (d *Driver) getLower(parent string) (string, error) {
	parentDir := d.dir(parent)

	// Ensure parent exists
	if _, err := os.Lstat(parentDir); err != nil {
		return "", err
	}

	// Read Parent link file
	parentLink, err := ioutil.ReadFile(path.Join(parentDir, "link"))
	if err != nil {
		return "", err
	}
	lowers := []string{path.Join(linkDir, string(parentLink))}

	parentLower, err := ioutil.ReadFile(path.Join(parentDir, lowerFile))
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
	lowers, err := ioutil.ReadFile(path.Join(d.dir(id), lowerFile))
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

func (d *Driver) unmountAndDetachLoopIfNeeded(id string, loopFile string) (bool, error) {
	mountPath := d.getDiffPath(id)

	didUnmount, err := d.unmountLoopbackDevice(mountPath)
	if err != nil {
		return didUnmount, err
	}
	if !didUnmount {
		// Unmount was not needed because refereces > 0
		return didUnmount, nil
	}

	// Prepare to detach loop device from loopFile by first finding the
	// right device
	f, err := os.Open(loopFile)
	if err != nil {
		return didUnmount, err
	}

	entries, err := mount.GetMounts()
	if err != nil {
		return didUnmount, err
	}

	// Search the table for the loop dev attached to loopFile
	for _, e := range entries {
		if e.Mountpoint == loopFile {
			// Found it. Now get the loop dev and close it.
			loopDev := loopback.GetLoopDeviceFor(f, e.Source)
			if loopDev == nil {
				defer f.Close()
				return didUnmount, fmt.Errorf("%s: couldn't get loopback device for mount %s", driverName,
					loopFile)
			}
			logrus.Debugf("Found loop device %s attached to file %s... will detach", loopDev.Name(),
				loopFile)
			defer loopDev.Close()

			// Continue to ensure no other attachments exist
		}
	}

	f.Close()

	return didUnmount, nil
}

// Remove cleans the directories that are created for this id.
func (d *Driver) Remove(id string) error {
	dir := d.dir(id)
	lid, err := ioutil.ReadFile(path.Join(dir, "link"))
	if err == nil {
		if err := os.RemoveAll(path.Join(d.home, linkDir, string(lid))); err != nil {
			logrus.Debugf("Failed to remove link: %v", err)
		}
	}

	// Check if loopfile exists. If it doesn't we skip trying to unmount it
	loopFile := d.findExistingLoopbackPath(id)
	if loopFile != "" {
		if _, err := d.unmountAndDetachLoopIfNeeded(id, loopFile); err != nil {
			return nil
		}

		logrus.Debugf("Removing loopback file %s", loopFile)

		// It's okay if we can't remove the loopback file due to a permission error
		// or because filesystem is read-only
		err = os.Remove(loopFile)
		if err != nil && !os.IsPermission(err) && err != syscall.EROFS {
			return err
		}
	}

	logrus.Debugf("Removing dir %s", dir)
	if err := os.RemoveAll(dir); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func getIDFromLink(linkPath string) (string, error) {
	path, err := os.Readlink(linkPath)
	if err != nil {
		return "", err
	}

	suffix := "/diff"

	//  Check that we received a correctly formatted path
	if !strings.HasSuffix(path, suffix) {
		return "", fmt.Errorf("%s: path does not contain diff directory: %s\n", driverName, path)
	}

	// Get the dir without "/diff"
	absIDPath := strings.TrimSuffix(path, suffix)
	id := filepath.Base(absIDPath)
	if (id == ".") || (id == "/") {
		return "", fmt.Errorf("%s: path not in expected format: %s\n", driverName, path)
	}

	return id, nil
}

func (d *Driver) findLoopAttached(loopFile string) (*os.File, error) {
	loopDev := loopback.FindLoopDeviceForPath(loopFile)

	if loopDev != nil {
		// Found a loop dev already attached. Leave open
		return loopDev, nil
	}

	// Continue to attach and then mount loopback device
	return nil, nil
}

func (d *Driver) mountROLoopbackIfNeeded(id string, loopFile string) (string, *os.File, error) {
	diffDir := d.getDiffPath(id)

	if isMounted, err := mount.Mounted(diffDir); err != nil {
		return "", nil, err
	} else if isMounted {
		// Already mounted, our work is done here
		return diffDir, nil, nil
	}

	loopDev, err := d.findLoopAttached(loopFile)
	if err != nil {
		return "", nil, err
	} else if loopDev == nil {

		// No existing loop devices are attached to loopFile

		loopDev, err = loopback.AttachROLoopDevice(loopFile)
		if err != nil {
			return "", nil, err
		}
		logrus.Debugf("%s: read-only attached loop dev %s to image %s", driverName, loopDev.Name(), loopFile)

	}
	// Else already attached, loopDev is open

	// Mount the device
	mountRW := false
	err = d.mountLoopbackDevice(loopDev.Name(), diffDir, id, mountRW)
	if err != nil {
		return "", loopDev, err
	}

	return diffDir, loopDev, nil
}

func (d *Driver) mountLowerIds(ids []string) error {
	if ids == nil {
		return fmt.Errorf("%s: no IDs of lower layers to mount", driverName)
	}

	var mounts []string
	var loops []*os.File
	var err error

	// Cleanup mounts if there was a failure
	defer func() {
		if err != nil {
			for i, m := range mounts {
				didUnmount, err := d.unmountLoopbackDevice(m)
				if err != nil || !didUnmount {
					continue
				}

				// detach the loop device
				defer loops[i].Close()
			}
		}
	}()

	// Iterate through the array of lower ids to mount. Failures are acceptable
	// if the loopback file does not exist. In that case the layer is the top
	// ephemeral layer that belongs to the container. As such, it will not be
	// found on sharedloop storage
	for _, id := range ids {
		var loopDev *os.File

		loopFile := path.Join(d.findExistingLoopbackPath(id))

		if loopFile == "" {
			// This could be the upper RW directory, for which there is no loop file
			continue
		}

		mountDir, loopDev, err := d.mountROLoopbackIfNeeded(id, loopFile)
		if mountDir != "" {
			mounts = append(mounts, mountDir)
		}

		if loopDev != nil {
			loops = append(loops, loopDev)
		}

		if err != nil {
			return err
		}
	}

	return nil
}

func (d *Driver) mountStores(paths []string) ([]string, error) {
	if paths == nil {
		return nil, fmt.Errorf("%s: no paths of lower layers to mount", driverName)
	}

	var mounts []string
	var loops []*os.File
	var err error

	// Cleanup mounts if there was a failure
	defer func() {
		if err != nil {
			for i, m := range mounts {
				didUnmount, err := d.unmountLoopbackDevice(m)
				if err != nil || !didUnmount {
					continue
				}

				// detach the loop device
				defer loops[i].Close()
			}
		}
	}()

	// Iterate through the array of lower paths to mount. Failures are not acceptable
	for _, p := range paths {
		var mountRW bool
		var loopDev *os.File

		loopFile := path.Join(p, "img")
		loopMount := path.Join(p, "loopmount")

		if isMounted, err := mount.Mounted(loopMount); err != nil {
			return nil, err
		} else if isMounted {
			// Already mounted, our work is done here
			continue
		}

		loopDev, err = d.findLoopAttached(loopFile)
		if err != nil {
			return nil, err
		} else if loopDev == nil {
			loopDev, err = loopback.AttachLoopDevice(loopFile)
			if err == nil {
				mountRW = true

				logrus.Debugf("%s: read-write attached loop dev %s to path %s", driverName, loopDev.Name(), loopFile)

				// Create mount dir
				rootUID, rootGID, err := idtools.GetRootUIDGID(d.uidMaps, d.gidMaps)
				if err != nil {
					return nil, err
				}
				if err := idtools.MkdirAllAs(loopMount, 0700, rootUID, rootGID); err != nil && !os.IsExist(err) {
					return nil, err
				}
			} else if err != syscall.EROFS {
				// Failed to attach RDWR, so try RDONLY

				if loopDev, err = loopback.AttachROLoopDevice(loopFile); err != nil {
					return nil, err
				}
				mountRW = false

				logrus.Debugf("%s: read-only attached loop dev %s to path %s", driverName, loopDev.Name(), loopFile)
			} else {
				// Failed to attach loop device even read-only

				return nil, err
			}
		}

		// Loop dev was successfully attached. Record it in the list, so we can
		// close the device if there's a failure

		loops = append(loops, loopDev)

		// Mount the device
		err = d.mountLoopbackDevice(loopDev.Name(), loopMount, "", mountRW)
		if err != nil {
			// Failure, but before returning we need to close
			// this loopDev because we haven't increased size
			// of mounts list used to clean up other loopDevs

			defer loopDev.Close()
			return nil, err
		}

		mounts = append(mounts, loopMount)
	}

	return mounts, nil
}

func (d *Driver) getDiffPath(id string) string {
	dir := d.dir(id)

	return path.Join(dir, "diff")
}

func (d *Driver) getParentMetaFile(id string) (string, error) {
	loopFile := d.findExistingLoopbackPath(id)
	if loopFile == "" {
		return "", fmt.Errorf("%s: couldn't find loopback device for ID %s, not writing parent metadata", driverName,
			id)
	}

	parentFile := loopFile + "-parent"

	return parentFile, nil
}

func (d *Driver) writeParentMetaFile(id string, parent string) error {
	parentFile, err := d.getParentMetaFile(id)
	if err != nil {
		return err
	}

	if err := ioutil.WriteFile(parentFile, []byte(parent), 0666); err != nil {
		return err
	}

	return nil
}

// Get creates and mounts the required file system for the given id and returns the mount path.
func (d *Driver) Get(id string, mountLabel string) (s string, err error) {

	dir := d.dir(id)
	if _, err := os.Stat(dir); err != nil {
		return "", fmt.Errorf("%s: dir %s for image with id %s could not be found", driverName, dir, id)
	}

	if loopFile := path.Join(d.findExistingLoopbackPath(id)); loopFile != "" {
		// This is not the upper RW directory, so ensure loopback device is mounted
		// read-only at {dir}/diff

		diffDir, loopDev, err := d.mountROLoopbackIfNeeded(id, loopFile)
		if err != nil {
			if loopDev != nil {
				defer loopDev.Close()
			}
			return "", err
		} else if diffDir == "" && loopDev != nil {
			defer loopDev.Close()
		}

		return diffDir, nil
	}

	lowers, err2 := ioutil.ReadFile(path.Join(dir, lowerFile))
	if err2 != nil {
		// If no lower, just return diff directory
		if os.IsNotExist(err2) {
			return d.getDiffPath(id), nil
		}
		return "", err2
	}

	mergedDir := path.Join(dir, "merged")

	// If overlay already mounted, skip mounting lowers as well
	if count := d.ctr.Increment(mergedDir); count > 1 {
		return mergedDir, nil
	}

	defer func() {
		if err != nil {
			if c := d.ctr.Decrement(mergedDir); c <= 0 {
				syscall.Unmount(mergedDir, 0)
			}
		}
	}()

	workDir := path.Join(dir, "work")
	splitLowers := strings.Split(string(lowers), ":")
	absLowers := make([]string, len(splitLowers))

	lowerIds := make([]string, len(splitLowers))

	for i, s := range splitLowers {
		absLowers[i] = path.Join(d.home, s)
		lowerIds[i], err = getIDFromLink(absLowers[i])
		if err != nil {
			lowerIds = nil
		}
	}

	err = d.mountLowerIds(lowerIds)
	if err != nil {
		return "", err
	}

	opts := fmt.Sprintf("lowerdir=%s,upperdir=%s,workdir=%s", strings.Join(absLowers, ":"), path.Join(dir, "diff"), path.Join(dir, "work"))
	mountData := label.FormatMountLabel(opts, mountLabel)
	mount := syscall.Mount
	mountTarget := mergedDir

	pageSize := syscall.Getpagesize()

	// Go can return a larger page size than supported by the system
	// as of go 1.7. This will be fixed in 1.8 and this block can be
	// removed when building with 1.8.
	// See https://github.com/golang/go/commit/1b9499b06989d2831e5b156161d6c07642926ee1
	// See https://github.com/docker/docker/issues/27384
	if pageSize > 4096 {
		pageSize = 4096
	}

	// Use relative paths and mountFrom when the mount data has exceeded
	// the page size. The mount syscall fails if the mount data cannot
	// fit within a page and relative links make the mount data much
	// smaller at the expense of requiring a fork exec to chroot.
	if len(mountData) > pageSize {
		opts = fmt.Sprintf("lowerdir=%s,upperdir=%s,workdir=%s", string(lowers), path.Join(id, "diff"), path.Join(id, "work"))
		mountData = label.FormatMountLabel(opts, mountLabel)
		if len(mountData) > pageSize {
			return "", fmt.Errorf("cannot mount layer, mount label too large %d", len(mountData))
		}

		mount = func(source string, target string, mType string, flags uintptr, label string) error {
			return mountFrom(d.home, source, target, mType, flags, label)
		}
		mountTarget = path.Join(id, "merged")
	}

	if err := mount("overlay", mountTarget, "overlay", 0, mountData); err != nil {
		return "", fmt.Errorf("error creating overlay mount to %s: %v", mergedDir, err)
	}

	// chown "workdir/work" to the remapped root UID/GID. Overlay fs inside a
	// user namespace requires this to move a directory from lower to upper.
	rootUID, rootGID, err := idtools.GetRootUIDGID(d.uidMaps, d.gidMaps)
	if err != nil {
		return "", err
	}

	if err := os.Chown(path.Join(workDir, "work"), rootUID, rootGID); err != nil {
		return "", err
	}

	return mergedDir, nil
}

func (d *Driver) unmountLowerIds(ids []string) error {
	if ids == nil {
		return fmt.Errorf("%s: no IDs of lower layers to unmount", driverName)
	}

	for _, id := range ids {
		// Check if loopfile exists. If it doesn't we skip trying to unmount it
		loopFile := d.findExistingLoopbackPath(id)
		if loopFile != "" {
			if _, err := d.unmountAndDetachLoopIfNeeded(id, loopFile); err != nil {
				return nil
			}
		}
	}

	return nil
}

// Put unmounts the mount path created for the given id.
func (d *Driver) Put(id string) error {
	dir := d.dir(id)

	// Unmount overlay
	mountpoint := path.Join(dir, "merged")
	if count := d.ctr.Decrement(mountpoint); count > 0 {
		return nil
	}
	if err := syscall.Unmount(mountpoint, 0); err != nil {
		logrus.Debugf("Failed to unmount %s overlay: %s - %v", id, mountpoint, err)
	}

	// Read lowers to unmount
	lowers, err := ioutil.ReadFile(path.Join(dir, lowerFile))
	if err != nil {
		// If there were no lower layers. the return after unmounting overlay
		return nil
	}

	splitLowers := strings.Split(string(lowers), ":")
	absLowers := make([]string, len(splitLowers))

	lowerIds := make([]string, len(splitLowers))

	for i, s := range splitLowers {
		absLowers[i] = path.Join(d.home, s)
		lowerIds[i], err = getIDFromLink(absLowers[i])
		if err != nil {
			lowerIds = nil
		}
	}

	err = d.unmountLowerIds(lowerIds)
	if err != nil {
		return err
	}

	return nil
}

// Exists checks to see if the id is already mounted.
func (d *Driver) Exists(id string) bool {
	// First see if there's a directory locally in /var/lib/docker
	_, err := os.Stat(d.dir(id))
	if err != nil {
		return false
	}

	// Make sure loopback device is mounted

	isMounted, _ := mount.Mounted(d.dir(id))
	if !isMounted {
		// Only OK if dir is an upper RW layer
		loopFile := d.findExistingLoopbackPath(id)
		if loopFile == "" {
			return true
		}

		logrus.Errorf("%s: Exists called on id %s, which is present, but loop file %s not mounted", driverName, id, loopFile)
		return false
	}

	return true
}

// ensureImage creates a sparse file of <size> bytes at the specified path
// If the file already exists and new size is larger than its current size, it grows to the new size.
// Either way it returns the full path.
func (d *Driver) ensureImage(filename string, size int64) error {
	if fi, err := os.Stat(filename); err != nil {
		if !os.IsNotExist(err) {
			return err
		}
		logrus.Debugf("%s: Creating loopback file %s", driverName, filename)
		file, err := os.OpenFile(filename, os.O_RDWR|os.O_CREATE, 0600)
		if err != nil {
			return err
		}
		defer file.Close()

		if err := file.Truncate(size); err != nil {
			return err
		}
	} else {
		if fi.Size() < size {
			file, err := os.OpenFile(filename, os.O_RDWR|os.O_CREATE, 0600)
			if err != nil {
				return err
			}
			defer file.Close()
			if err := file.Truncate(size); err != nil {
				return fmt.Errorf("%s: Unable to grow loopback file %s: %v", driverName, filename, err)
			}
		} else if fi.Size() > size {
			logrus.Warnf("%s: Can't shrink loopback file %s", driverName, filename)
		}
	}
	return nil
}

// Return true only if kernel supports xfs and mkfs.xfs is available
func xfsSupported() bool {
	// Make sure mkfs.xfs is available
	if _, err := exec.LookPath("mkfs.xfs"); err != nil {
		return false
	}

	// Check if kernel supports xfs filesystem or not.
	exec.Command("modprobe", "xfs").Run()

	f, err := os.Open("/proc/filesystems")
	if err != nil {
		logrus.Warnf("%s: Could not check if xfs is supported: %v", driverName, err)
		return false
	}
	defer f.Close()

	s := bufio.NewScanner(f)
	for s.Scan() {
		if strings.HasSuffix(s.Text(), "\txfs") {
			return true
		}
	}

	if err := s.Err(); err != nil {
		logrus.Warnf("%s: Could not check if xfs is supported: %v", driverName, err)
	}
	return false
}

func determineDefaultFS() string {
	if xfsSupported() {
		return "xfs"
	}

	logrus.Warn("%s: XFS is not supported in your system. Either the kernel doesn't support it or mkfs.xfs is not in your PATH. Defaulting to ext4 filesystem", driverName)
	return "ext4"
}

func (d *Driver) createFilesystem(devname string) (err error) {
	args := []string{}
	for _, arg := range d.mkfsArgs {
		args = append(args, arg)
	}

	args = append(args, devname)

	logrus.Debugf("%s: Creating filesystem %s on device %s", driverName, d.filesystem, devname)
	defer func() {
		if err != nil {
			logrus.Errorf("%s: Error while creating filesystem %s on device %s with args %s: %v", driverName,
				d.filesystem, devname, args, err)
		} else {
			logrus.Debugf("%s: Successfully created filesystem %s on device %s", driverName, d.filesystem, devname)
		}
	}()

	switch d.filesystem {
	case "xfs":
		err = exec.Command("mkfs.xfs", args...).Run()
	case "ext4":
		err = exec.Command("mkfs.ext4", append([]string{"-E", "nodiscard,lazy_itable_init=0,lazy_journal_init=0"}, args...)...).Run()
		if err != nil {
			err = exec.Command("mkfs.ext4", append([]string{"-E", "nodiscard,lazy_itable_init=0"}, args...)...).Run()
		}
		if err != nil {
			return err
		}
		err = exec.Command("tune2fs", append([]string{"-c", "-1", "-i", "0"}, devname)...).Run()
	default:
		err = fmt.Errorf("%s: Unsupported filesystem type %s", driverName, d.filesystem)
	}
	return
}

func (d *Driver) createLoopbackID(id string, size int64) (*os.File, error) {
	var err error

	createdLoopback := false

	filename := d.findExistingLoopbackPath(id)
	if filename == "" {
		filename, err = d.findNewLoopbackPath(id)
		if err != nil {
			return nil, err
		}
		createdLoopback = true
	}

	// Create the loopback image file
	if err := d.ensureImage(filename, size); err != nil {
		logrus.Debugf("%s: Error device ensureImage (%s): %s", driverName, id, err)
		return nil, err
	}

	loopDev, err := loopback.AttachLoopDevice(filename)
	if err != nil {
		return nil, err
	}

	// Format the device with a filesystem if it was just created
	if createdLoopback {
		if err := d.createFilesystem(loopDev.Name()); err != nil {
			logrus.Debugf("%s: Error createFilesystem on %s: %s", driverName, loopDev.Name(), err)
			return nil, err
		}
	}

	return loopDev, nil
}

func (d *Driver) createLoopbackImg(path string, size int64) (*os.File, error) {
	createdLoopback := false

	_, err := os.Stat(path)
	if err != nil {
		createdLoopback = true
	}

	// Create the loopback image file
	if err := d.ensureImage(path, size); err != nil {
		logrus.Debugf("%s: Error device ensureImage (%s): %s", driverName, path, err)
		return nil, err
	}

	loopDev, err := loopback.AttachLoopDevice(path)
	if err != nil {
		return nil, err
	}

	// Format the device with a filesystem if it was just created
	if createdLoopback {
		if err := d.createFilesystem(loopDev.Name()); err != nil {
			logrus.Debugf("%s: Error createFilesystem on %s: %s", driverName, loopDev.Name(), err)
			return nil, err
		}
	}

	return loopDev, nil
}

func (d *Driver) mountLoopbackDevice(dev, path, mountLabel string, mountRW bool) error {
	options := ""

	// If already mounted, just return
	if count := d.ctr.Increment(path); count > 1 {
		return nil
	}

	if mountRW {
		options = joinMountOptions(options, "rw")
	} else {
		options = joinMountOptions(options, "ro")
	}

	if d.filesystem == "xfs" {
		// XFS needs nouuid or it can't mount filesystems with the same fs
		options = joinMountOptions(options, "nouuid")
	}

	if err := mount.Mount(dev, path, d.filesystem, options); err != nil {
		return fmt.Errorf("%s: Error mounting '%s' on '%s': %s", driverName, dev, path, err)
	}

	logrus.Debugf("%s: Mounted loopback device %s at %s, RW=%t", driverName, dev, path, mountRW)

	return nil
}

func (d *Driver) unmountLoopbackDevice(mountPath string) (bool, error) {
	// Prepare to unmount
	if count := d.ctr.Decrement(mountPath); count > 0 {
		return false, fmt.Errorf("%s: do not have exclusive access on unmount of %s", driverName, mountPath)
	}
	if err := syscall.Unmount(mountPath, syscall.MNT_DETACH); err != nil {
		return false, err
	}

	logrus.Debugf("%s: Unmounted loopback device at %s", driverName, mountPath)

	return true, nil
}

// isParent returns if the passed in parent is the direct parent of the passed in layer
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

	size = 1024 * 1024 * 1024 * 10

	// Create and attach loopback device
	loopDev, err := d.createLoopbackID(id, size)
	if err != nil {
		return 0, err
	}
	defer loopDev.Close()

	applyDir := d.getDiffPath(id)

	// Mount the device
	mountRW := true
	if err := d.mountLoopbackDevice(loopDev.Name(), applyDir, id, mountRW); err != nil {
		return 0, err
	}

	logrus.Debugf("Applying tar in %s", applyDir)
	// Overlay doesn't need the parent id to apply the diff
	if err := untar(diff, applyDir, &archive.TarOptions{
		UIDMaps:        d.uidMaps,
		GIDMaps:        d.gidMaps,
		WhiteoutFormat: archive.OverlayWhiteoutFormat,
	}); err != nil {
		return 0, err
	}

	actualSize, err := d.DiffSize(id, parent)
	if err != nil {
		return actualSize, err
	}

	didUnmount, err := d.unmountLoopbackDevice(applyDir)
	if err != nil || !didUnmount {
		return actualSize, err
	}

	if parent != "" {
		err = d.writeParentMetaFile(id, parent)
		if err != nil {
			return actualSize, err
		}
	}

	return actualSize, nil
}

// DiffSize calculates the changes between the specified id
// and its parent and returns the size in bytes of the changes
// relative to its base filesystem directory.
func (d *Driver) DiffSize(id, parent string) (size int64, err error) {
	si := syscall.Stat_t{}

	// Call stat on the loopback image
	err = syscall.Stat(d.findExistingLoopbackPath(id), &si)
	if err != nil {
		return 0, err
	}

	return si.Blocks * si.Blksize, err
}

// Diff produces an archive of the changes between the specified
// layer and its parent layer which may be "".
func (d *Driver) Diff(id, parent string) (io.ReadCloser, error) {
	if !d.isParent(id, parent) {
		return d.naiveDiff.Diff(id, parent)
	}

	diffPath := d.getDiffPath(id)
	logrus.Debugf("Tar with options on %s", diffPath)
	return archive.TarWithOptions(diffPath, &archive.TarOptions{
		Compression:    archive.Uncompressed,
		UIDMaps:        d.uidMaps,
		GIDMaps:        d.gidMaps,
		WhiteoutFormat: archive.OverlayWhiteoutFormat,
	})
}

// Changes produces a list of changes between the specified layer
// and its parent layer. If parent is "", then all changes will be ADD changes.
func (d *Driver) Changes(id, parent string) ([]archive.Change, error) {
	if !d.isParent(id, parent) {
		return d.naiveDiff.Changes(id, parent)
	}
	// Overlay doesn't have snapshots, so we need to get changes from all parent
	// layers.
	diffPath := d.getDiffPath(id)
	layers, err := d.getLowerDirs(id)
	if err != nil {
		return nil, err
	}

	return archive.OverlayChanges(layers, diffPath)
}

func stringInSlice(a string, list []string) bool {
	for _, b := range list {
		if b == a {
			return true
		}
	}
	return false
}

func (d *Driver) mergeStores(loopStoreImgDirs []string) error {
	var lowers []string
	var loopMounts []string
	var err error

	if loopStoreImgDirs == nil {
		return fmt.Errorf("%s: no IDs of lower store paths to mount", driverName)
	}

	loopMounts, err = d.mountStores(loopStoreImgDirs)
	if err != nil {
		return err
	}

	// Cleanup loopback mounts if there was a failure
	defer func() {
		if err != nil {
			for _, m := range loopMounts {
				d.unmountLoopbackDevice(m)
			}
		}
	}()

	for _, l := range loopMounts {
		lowers = append(lowers, path.Join(l, "diff"))
	}

	mountTarget := path.Join(d.defaultStoreDir, driverName)

	// Make sure mount target exists
	rootUID, rootGID, err := idtools.GetRootUIDGID(d.uidMaps, d.gidMaps)
	if err != nil {
		return err
	}
	err = idtools.MkdirAllAs(mountTarget, 0700, rootUID, rootGID)
	if err != nil {
		return err
	}

	if err != nil && !os.IsExist(err) {
		return err
	}

	mount := syscall.Mount
	if len(loopMounts) > 1 {
		// Only use overlayfs if there is more than 1 lower dir

		lowerDir := strings.Join(lowers[0:len(lowers)-1], ":")

		// the upper dir will be diff dir of last element of loopMounts
		upperDir := path.Join(loopMounts[len(loopMounts)-1], "diff")

		workDir := path.Join(loopMounts[len(loopMounts)-1], "work")
		err = idtools.MkdirAs(workDir, 0700, rootUID, rootGID)
		if err != nil && !os.IsExist(err) {
			return err
		}

		opts := fmt.Sprintf("lowerdir=%s,upperdir=%s,workdir=%s", lowerDir, upperDir, workDir)
		err = mount("overlay", mountTarget, "overlay", 0, opts)
		if err != nil {
			logrus.Errorf("%s: error creating overlay mount to %s: %v", driverName, mountTarget, err)
			return err
		}
	} else if len(loopMounts) == 1 {
		// Use a bind mount to mount the fallback loop device at the default store directory

		err = mount(lowers[0], mountTarget, "bind", syscall.MS_BIND, "")
		if err != nil {
			logrus.Errorf("%s: error creating bind mount from %s to %s: %v", driverName, lowers[0], mountTarget, err)
			return err
		}
	} else {
		// Else there were no stores mounted this time around. Could be where leftover mounts exists.
		// In any case, we must abovrt
		return err
	}

	return nil
}

func (d *Driver) createStores(storeImgDir string) error {

	loopMount := path.Join(storeImgDir, "loopmount")
	loopMountDiff := path.Join(loopMount, "diff")
	storeImg := path.Join(storeImgDir, "img")
	size := 1024 * 1024 * 1024 * 1

	// Create and attach loopback device
	loopDev, err := d.createLoopbackImg(storeImg, int64(size))
	if err != nil {
		return err
	}
	// Since at this point, loopDev has been opened
	defer loopDev.Close()

	defer func() {
		if err != nil {
			didUnmount, _ := d.unmountLoopbackDevice(loopMount)
			if didUnmount {
				// Don't leave behind corrupted stores
				os.Remove(storeImg)
			}
		}
	}()

	// Create the mountPath for loopback device
	rootUID, rootGID, err := idtools.GetRootUIDGID(d.uidMaps, d.gidMaps)
	if err != nil {
		return err
	}
	if err := idtools.MkdirAllAs(loopMount, 0700, rootUID, rootGID); err != nil && !os.IsExist(err) {
		return err
	}

	// Mount loopback device
	mountRW := true
	err = d.mountLoopbackDevice(loopDev.Name(), loopMount, "", mountRW)
	if err != nil {
		return err
	}

	// Create the diff dir within loopback image
	if err := idtools.MkdirAllAs(loopMountDiff, 0700, rootUID, rootGID); err != nil && !os.IsExist(err) {
		return err
	}

	// Failure at this point should not trigger removal, so don't set err
	didUnmount, err2 := d.unmountLoopbackDevice(loopMount)
	if err2 != nil {
		return err2
	}
	if !didUnmount {
		return fmt.Errorf("%s: failed to unmount %s", driverName, loopMount)
	}

	return nil
}
