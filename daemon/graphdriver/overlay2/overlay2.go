// +build linux

/*

overlay2 driver directory structure

  .
  ├── layers // Metadata of layers
  │   ├── 1
  │   ├── 2
  │   └── 3
  ├── diff   // Content of the layer
  │   ├── 1
  │   ├── 2
  │   └── 3
  ├── mnt    // Mount points for the rw layers to be mounted
  │   ├── 1
  │   ├── 2
  │   └── 3
  └── work   // overlayfs work directories used for temporary state
	  ├── 1
	  ├── 2
	  └── 3

*/

package overlay2

import (
	"bufio"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/docker/docker/daemon/graphdriver"
	"github.com/docker/docker/pkg/archive"
	"github.com/docker/docker/pkg/chrootarchive"
	"github.com/docker/docker/pkg/directory"
	"github.com/docker/docker/pkg/idtools"
	mountpk "github.com/docker/docker/pkg/mount"
	"github.com/docker/docker/pkg/parsers/kernel"
	"github.com/opencontainers/runc/libcontainer/label"
)

const (
	mntPath    = "mnt"
	diffPath   = "diff"
	layersPath = "layers"
	workPath   = "work"
)

var (
	allPaths    = []string{mntPath, diffPath, layersPath, workPath}
	allDirPaths = []string{mntPath, diffPath, workPath} // All paths that contain directories for the given ID (as opposed to files)
)

const driverName = "overlay2"

var backingFs = "<unknown>"

// ActiveMount contains information about the count, path and whether is mounted or not.
// This information is part of the Driver, that contains list of active mounts that are part of this overlay.
type ActiveMount struct {
	referenceCount int
	path           string
}

// Driver contains information about the root directory and the list of active mounts that are created using this driver.
type Driver struct {
	root       string
	sync.Mutex // Protects concurrent modification to active
	active     map[string]*ActiveMount
	uidMaps    []idtools.IDMap
	gidMaps    []idtools.IDMap
}

func init() {
	graphdriver.Register(driverName, Init)
}

// Init checks for compatibility and creates an instance of the driver
func Init(root string, options []string, uidMaps, gidMaps []idtools.IDMap) (graphdriver.Driver, error) {

	if err := supportsOverlay(); err != nil {
		return nil, graphdriver.ErrNotSupported
	}

	version, err := kernel.GetKernelVersion()
	if err != nil {
		return nil, err
	}

	// require a version of overlay that supports multiple ro layers
	if kernel.CompareKernelVersion(*version, kernel.VersionInfo{3, 19, 0, ""}) == -1 {
		return nil, graphdriver.ErrNotSupported
	}

	fsMagic, err := graphdriver.GetFSMagic(root)
	if err != nil {
		return nil, err
	}
	if fsName, ok := graphdriver.FsNames[fsMagic]; ok {
		backingFs = fsName
	}

	// check if they are running over btrfs or aufs
	switch fsMagic {
	case graphdriver.FsMagicBtrfs:
		logrus.Error("'overlay' is not supported over btrfs.")
		return nil, graphdriver.ErrIncompatibleFS
	case graphdriver.FsMagicAufs:
		logrus.Error("'overlay' is not supported over aufs.")
		return nil, graphdriver.ErrIncompatibleFS
	case graphdriver.FsMagicZfs:
		logrus.Error("'overlay' is not supported over zfs.")
		return nil, graphdriver.ErrIncompatibleFS
	}

	rootUID, rootGID, err := idtools.GetRootUIDGID(uidMaps, gidMaps)
	if err != nil {
		return nil, err
	}
	// Create the driver root dir
	if err := idtools.MkdirAllAs(root, 0755, rootUID, rootGID); err != nil && !os.IsExist(err) {
		return nil, err
	}

	// Populate the dir structure
	for _, p := range allPaths {
		if err := idtools.MkdirAllAs(path.Join(root, p), 0755, rootUID, rootGID); err != nil {
			return nil, err
		}
	}

	return &Driver{
		root:    root,
		active:  make(map[string]*ActiveMount),
		uidMaps: uidMaps,
		gidMaps: gidMaps,
	}, nil
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

// String returns a string representation of this driver.
func (d *Driver) String() string {
	return driverName
}

// GetMetadata returns a set of key-value pairs which give low level information
// about the image/container driver is managing.
func (d *Driver) GetMetadata(id string) (map[string]string, error) {
	metadata := make(map[string]string)

	metadata["mntPath"] = d.dir(mntPath, id)
	metadata["diffPath"] = d.dir(diffPath, id)
	metadata["layersPath"] = d.dir(layersPath, id)
	metadata["workPath"] = d.dir(workPath, id)
	ids, _ := d.getParentIds(id)
	metadata["layers"] = strings.Join(ids, ",")
	active, mounted := d.active[id]
	if mounted {
		metadata["referenceCount"] = fmt.Sprintf("%d", active.referenceCount)
	}

	return metadata, nil
}

// Read the layers file for the current id and return all the
// layers represented by new lines in the file
//
// If there are no lines in the file then the id has no parent
// and an empty slice is returned.
func (d *Driver) getParentIds(id string) ([]string, error) {
	f, err := os.Open(d.dir(layersPath, id))
	if err != nil {
		return nil, err
	}
	defer f.Close()

	out := []string{}
	s := bufio.NewScanner(f)

	for s.Scan() {
		if t := s.Text(); t != "" {
			out = append(out, s.Text())
		}
	}
	return out, s.Err()
}

// Create creates 4 dirs for each id: mnt, layers, work and diff
// mnt and work are not used until Get is called, but we create them here anyway to
// avoid having to create them multiple times
func (d *Driver) Create(id, parent string) error {
	if err := d.createDirsFor(id); err != nil {
		return err
	}
	// Write the layers metadata (the stack of parents)
	f, err := os.Create(d.dir(layersPath, id))
	if err != nil {
		return err
	}
	defer f.Close()

	if parent != "" {
		ids, err := d.getParentIds(parent)
		if err != nil {
			return err
		}

		if _, err := fmt.Fprintln(f, parent); err != nil {
			return err
		}
		for _, i := range ids {
			if _, err := fmt.Fprintln(f, i); err != nil {
				return err
			}
		}
	}
	d.active[id] = &ActiveMount{}
	return nil
}

// even though the work directory is relevant only for mounted containers, we create it anyway
func (d *Driver) createDirsFor(id string) error {
	rootUID, rootGID, err := idtools.GetRootUIDGID(d.uidMaps, d.gidMaps)
	if err != nil {
		return err
	}
	for _, p := range allDirPaths {
		if err := idtools.MkdirAllAs(d.dir(p, id), 0755, rootUID, rootGID); err != nil {
			return err
		}
	}
	return nil
}

// Remove will unmount and remove the given id.
// XXX: can this be called even though there are active Get requests? If so, we need to properly Put it first. (to remove intermediate mounts)
func (d *Driver) Remove(id string) error {
	// Protect the d.active from concurrent access
	d.Lock()
	defer d.Unlock()

	m := d.active[id]
	if m != nil {
		// XXX: what does this case mean? When does this happen?
		if m.referenceCount > 0 {
			return nil
		}
		// Make sure the dir is umounted first
		if err := d.unmount(m.path); err != nil {
			return err
		}
	}
	tmpDirs := []string{
		mntPath,
		diffPath,
		workPath,
	}

	// XXX: why? maybe we should just remove things and not care like the overlay driver does
	// Atomically remove each directory in turn by first moving it out of the
	// way (so that docker doesn't find it anymore) before doing removal of
	// the whole tree.
	for _, p := range tmpDirs {
		realPath := d.dir(p, id)
		tmpPath := d.dir(p, fmt.Sprintf("%s-removing", id))
		if err := os.Rename(realPath, tmpPath); err != nil && !os.IsNotExist(err) {
			return err
		}
		defer os.RemoveAll(tmpPath)
	}
	// Remove the layers file for the id
	if err := os.Remove(d.dir(layersPath, id)); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// Changes produces a list of changes between the specified layer
// and its parent layer. If parent is "", then all changes will be ADD changes.
func (d *Driver) Changes(id, parent string) ([]archive.Change, error) {
	layerFs, err := d.Get(id, "")
	if err != nil {
		return nil, err
	}
	defer d.Put(id)

	parentFs := ""

	if parent != "" {
		parentFs, err = d.Get(parent, "")
		if err != nil {
			return nil, err
		}
		defer d.Put(parent)
	}

	return archive.ChangesDirs(layerFs, parentFs)
}

// Get creates and mounts the required file system for the given id and returns the mount path.
func (d *Driver) Get(id string, mountLabel string) (string, error) {
	ids, err := d.getParentIds(id)
	if err != nil {
		if !os.IsNotExist(err) {
			return "", err
		}
		ids = []string{}
	}

	// Protect the d.active from concurrent access
	d.Lock()
	defer d.Unlock()

	m := d.active[id]
	if m == nil {
		m = &ActiveMount{}
		d.active[id] = m
	}

	// If a dir does not have a parent ( no layers )do not try to mount
	// just return the diff path to the data
	m.path = d.dir(diffPath, id)
	if len(ids) > 0 {
		m.path = d.dir(mntPath, id)
		if m.referenceCount == 0 {
			if err := d.mountID(id, mountLabel); err != nil {
				return "", err
			}
		}
	}
	m.referenceCount++
	return m.path, nil
}

// maxStack comes from OVL_MAX_STACK in the linux kernel's overlayfs implementation
// It's the maximum number of levels in one overlay mount
const maxStack = 500

// Max length of mount options in system call to mount
// This is equal to one page. The page size is always 4KB (XXX: I think?)
// There is a null terminator at the end, so we subtract one
const maxMountOptsLen = 4095

func (d *Driver) mountro(mountPath string, layers []string, mountLabel string) error {
	logrus.Debugf("mounting ro %v %v %v", mountPath, layers, mountLabel)
	mntOpts := label.FormatMountLabel(fmt.Sprintf("lowerdir=%s", strings.Join(layers, ":")), mountLabel)
	logrus.Debugf("mount opts length %d", len(mntOpts))
	if len(mntOpts) > maxMountOptsLen {
		logrus.Debugf("mount opts too long %d", len(mntOpts))
		return mountOptsTooLong(fmt.Sprintf("can't mount overlay: mount opts too long: %d", len(mntOpts)))
	}
	if err := syscall.Mount("overlay", mountPath, "overlay", 0, mntOpts); err != nil {
		return fmt.Errorf("error creating overlay mount to %s: %v", mountPath, err)
	}
	return nil
}

// this returns the path to intermediate mount `i` for id `id`
func (d *Driver) formatIntermediateMountPath(id string, i int) string {
	return d.dir(mntPath, fmt.Sprintf("%s-%02d", id, i))
}

func (d *Driver) mountPartMaxLength(id, mountLabel string) int {
	upperDir := d.dir(diffPath, id)
	workDir := d.dir(workPath, id)

	extraStringsLength := len(label.FormatMountLabel(fmt.Sprintf("lowerdir=%s:,upperdir=%s,workdir=%s", d.formatIntermediateMountPath(id, 0), upperDir, workDir), mountLabel))

	return maxMountOptsLen - extraStringsLength
}

type mountOptsTooLong string

func (m mountOptsTooLong) Error() string {
	return string(m)
}

var count = 0

func (d *Driver) mountrw(id string, layers []string, mountLabel string) error {
	logrus.Debugf("mounting rw %v %v %v", id, layers, mountLabel)
	upperDir := d.dir(diffPath, id)
	workDir := d.dir(workPath, id)
	mergedDir := d.dir(mntPath, id)
	lowerDirs := strings.Join(layers, ":")

	mntOpts := label.FormatMountLabel(fmt.Sprintf("lowerdir=%s,upperdir=%s,workdir=%s", lowerDirs, upperDir, workDir), mountLabel)
	logrus.Debugf("mount opt length %d", len(mntOpts))
	if len(mntOpts) > maxMountOptsLen {
		logrus.Debugf("mount opts too long %d", len(mntOpts))
		return mountOptsTooLong(fmt.Sprintf("can't mount overlay: mount opts too long: %d", len(mntOpts)))
	}

	if err := syscall.Mount("overlay", mergedDir, "overlay", 0, mntOpts); err != nil {
		logrus.Debugf("error creating overlay mount %v", err)
		return fmt.Errorf("error creating overlay mount to %s: %v", mergedDir, err)
	}

	rootUID, rootGID, err := idtools.GetRootUIDGID(d.uidMaps, d.gidMaps)
	if err != nil {
		return err
	}

	// chown "workdir/work" to the remapped root UID/GID. Overlay fs inside a
	// user namespace requires this to move a directory from lower to upper.
	if err := os.Chown(workDir, rootUID, rootGID); err != nil {
		return err
	}
	logrus.Debugf("success %s", mergedDir)
	if len(mntOpts) == 1362 {
		count++
		if count == 2 {
			time.Sleep(10 * time.Minute)
		}
	}
	return nil
}

// Warning: we are limited to 100 intermediate mount points. This should allow for
// a very large number of layers (10000+ depending on filename length), but not infinite
// this limit is because we assume that intermediate mounts have fixed length paths for
// simplicity. The intermediate mount numbers are limited to 2 digits.
func (d *Driver) mountID(id string, mountLabel string) error {
	mergedDir := d.dir(mntPath, id)

	// If the id is mounted or we get an error return
	if mounted, err := mountpk.Mounted(mergedDir); err != nil || mounted {
		return err
	}

	// the layers are in order from highest to lowest; same as the overlay options order
	layers, err := d.getParentLayerPaths(id)
	if err != nil {
		return err
	}

	return d.tryMountRW(id, layers, mountLabel, 0)
}

func (d *Driver) tryMountRW(id string, layers []string, mountLabel string, level int) error {
	logrus.Debugf("mounting level %d", level)
	// first we try to fit the mount options in a single page
	err := d.mountrw(id, layers, mountLabel)
	// if this worked, we are done
	if err == nil {
		return nil
	}
	// if there was an error that was not because the mountOpts were too long, we failed
	if _, ok := err.(mountOptsTooLong); !ok {
		return err
	}

	// in this case we can't fit all directories in one mount, so we split it into two parts
	// one that we can mount as read-only and one that we'll try to mount as read-write again

	// pick as many layers as we can to put in the RO mount
	maxLen := d.mountPartMaxLength(id, mountLabel)
	lenSum := 0
	numROLayers := 0
	for i := len(layers) - 1; i >= 0; i-- {
		curr := layers[i]
		// count the : separator only after the first layer
		if lenSum != 0 {
			lenSum++
		}
		lenSum += len(curr)
		// if we exceed the length, stop adding layers
		if lenSum > maxLen {
			break
		}
		// there is a maximum number of layers defined by overlayfs
		if numROLayers > maxStack {
			break
		}
		numROLayers++
	}
	firstROLayerI := len(layers) - 1 - numROLayers
	roLayers := layers[firstROLayerI:]

	// mount the RO mount

	rootUID, rootGID, err := idtools.GetRootUIDGID(d.uidMaps, d.gidMaps)
	if err != nil {
		return err
	}

	// first we need to create this directory
	mountPath := d.formatIntermediateMountPath(id, level)
	if err := idtools.MkdirAllAs(mountPath, 0755, rootUID, rootGID); err != nil {
		return err
	}

	if err := d.mountro(mountPath, roLayers, mountLabel); err != nil {
		return err
	}

	// now we can try to create the RW mount again with this mount at the bottom of the stack
	rwLayers := append(layers[:firstROLayerI], d.formatIntermediateMountPath(id, level))
	return d.tryMountRW(id, rwLayers, mountLabel, level+1)
}

// Put unmounts and updates list of active mounts.
func (d *Driver) Put(id string) error {
	// Protect the d.active from concurrent access
	d.Lock()
	defer d.Unlock()

	m := d.active[id]
	if m == nil {
		// but it might be still here
		if d.Exists(id) {
			if err := d.unmount(id); err != nil {
				logrus.Debugf("Failed to unmount %s overlay: %v", id, err)
			}
		}
		return nil
	}
	if count := m.referenceCount; count > 1 {
		m.referenceCount = count - 1
	} else {
		ids, _ := d.getParentIds(id)
		// We only mounted if there are any parents
		if ids != nil && len(ids) > 0 {
			d.unmount(id)
		}
		delete(d.active, id)
	}
	return nil
}

func (d *Driver) getParentLayerPaths(id string) ([]string, error) {
	parentIds, err := d.getParentIds(id)
	if err != nil {
		return nil, err
	}
	layers := make([]string, len(parentIds))

	// Get the diff paths for all the parent ids
	for i, p := range parentIds {
		layers[i] = d.dir(diffPath, p)
	}
	return layers, nil
}

func (d *Driver) unmount(id string) error {
	logrus.Debugf("unmount %s", id)
	// first unmount the top mount
	if err := d.unmountPath(d.dir(mntPath, id)); err != nil {
		return err
	}

	// we need to figure out what intermediate mounts exist and unmount them as well
	// we do this by guessing until we reach one that doesn't exist
	for i := 0; true; i++ {
		dir := d.formatIntermediateMountPath(id, i)
		if _, err := os.Lstat(dir); err != nil {
			break
		}
		logrus.Debugf("unmount and remove intermediate %s", dir)
		d.unmountPath(dir)
		// we don't want to keep around intermediate dirs
		os.Remove(dir)
	}

	return nil
}

func (d *Driver) unmountPath(path string) error {
	if mounted, err := mountpk.Mounted(path); err != nil || !mounted {
		return err
	}
	if err := syscall.Unmount(path, 0); err != nil {
		return err
	}
	return nil
}

// Status returns current information about the filesystem such as root directory, number of directories mounted, etc.
func (d *Driver) Status() [][2]string {
	ids, _ := loadIds(path.Join(d.root, layersPath))
	return [][2]string{
		{"Root Dir", d.root},
		{"Backing Filesystem", backingFs},
		{"Layers", fmt.Sprintf("%d", len(ids))},
	}
}

// Diff produces an archive of the changes between the specified
// layer and its parent layer which may be "".
func (d *Driver) Diff(id, parent string) (archive.Archive, error) {
	// overlay2 doesn't need the parent layer to produce a diff.
	return archive.TarWithOptions(d.dir(diffPath, id), &archive.TarOptions{
		Compression:   archive.Uncompressed,
		UIDMaps:       d.uidMaps,
		GIDMaps:       d.gidMaps,
		OverlayFormat: true,
	})
}

// Cleanup performs necessary tasks to release resources
// held by the driver, e.g., unmounting all layered filesystems
// known to this driver.
func (d *Driver) Cleanup() error {
	return nil
}

// DiffSize calculates the changes between the specified id
// and its parent and returns the size in bytes of the changes
// relative to its base filesystem directory.
func (d *Driver) DiffSize(id, parent string) (size int64, err error) {
	// overlay doesn't need the parent layer to calculate the diff size.
	return directory.Size(d.dir(diffPath, id))
}

// ApplyDiff extracts the changeset from the given diff into the
// layer with the specified id and parent, returning the size of the
// new layer in bytes.
func (d *Driver) ApplyDiff(id, parent string, diff archive.Reader) (size int64, err error) {
	// overlay doesn't need the parent id to apply the diff.
	if err := chrootarchive.UntarUncompressed(diff, d.dir(diffPath, id), &archive.TarOptions{
		UIDMaps:       d.uidMaps,
		GIDMaps:       d.gidMaps,
		OverlayFormat: true,
	}); err != nil {
		return 0, err
	}

	return d.DiffSize(id, parent)
}

// Exists returns true if the given id is registered with
// this driver
func (d *Driver) Exists(id string) bool {
	if _, err := os.Lstat(d.dir(layersPath, id)); err != nil {
		return false
	}
	return true
}

// dir returns the directory for the given kind of path for the given container id
// kind can be one of layersPath, diffPath, mntPath, workPath
func (d *Driver) dir(kind, id string) string {
	return path.Join(d.root, kind, id)
}

// return the list of ids in the file at this path
func loadIds(root string) ([]string, error) {
	dirs, err := ioutil.ReadDir(root)
	if err != nil {
		return nil, err
	}
	out := []string{}
	for _, d := range dirs {
		if !d.IsDir() {
			out = append(out, d.Name())
		}
	}
	return out, nil
}
