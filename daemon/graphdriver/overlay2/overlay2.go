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

/*
TODO:

* detect if overlay is supported by the kernel and it's new enough to support multiple RO layers
* detect if overlay is supported on the underlying fs
* make sure we support user namespaces correctly


*/

const (
	MntPath    = "mnt"
	DiffPath   = "diff"
	LayersPath = "layers"
	WorkPath   = "work"
)

var (
	AllPaths    = []string{MntPath, DiffPath, LayersPath, WorkPath}
	AllDirPaths = []string{MntPath, DiffPath, WorkPath} // All paths that contain directories for the given ID (as opposed to files)
)

const DriverName = "overlay2"

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
	graphdriver.Register(DriverName, Init)
}

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

	rootUID, rootGID, err := idtools.GetRootUIDGID(uidMaps, gidMaps)
	if err != nil {
		return nil, err
	}
	// Create the driver root dir
	if err := idtools.MkdirAllAs(root, 0755, rootUID, rootGID); err != nil && !os.IsExist(err) {
		return nil, err
	}

	// XXX: do we need MakePrivate?

	// Populate the dir structure
	for _, p := range AllPaths {
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

// XXX: copied from overlay driver
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

// XXX: to implement:
/*
type ProtoDriver interface {
	// String returns a string representation of this driver.
	. String() string
	// Create creates a new, empty, filesystem layer with the
	// specified id and parent. Parent may be "".
	. Create(id, parent string) error
	// Remove attempts to remove the filesystem layer with this id.
	. Remove(id string) error
	// Get returns the mountpoint for the layered filesystem referred
	// to by this id. You can optionally specify a mountLabel or "".
	// Returns the absolute path to the mounted layered filesystem.
	. Get(id, mountLabel string) (dir string, err error)
	// Put releases the system resources for the specified id,
	// e.g, unmounting layered filesystem.
	. Put(id string) error
	// Exists returns whether a filesystem layer with the specified
	// ID exists on this driver.
	. Exists(id string) bool
	// Status returns a set of key-value pairs which give low
	// level diagnostic status about this driver.
	. Status() [][2]string
	// Returns a set of key-value pairs which give low level information
	// about the image/container driver is managing.
	. GetMetadata(id string) (map[string]string, error)
	// Cleanup performs necessary tasks to release resources
	// held by the driver, e.g., unmounting all layered filesystems
	// known to this driver.
	. Cleanup() error
	// Diff produces an archive of the changes between the specified
	// layer and its parent layer which may be "".
	. Diff(id, parent string) (archive.Archive, error)
	// Changes produces a list of changes between the specified layer
	// and its parent layer. If parent is "", then all changes will be ADD changes.
	. Changes(id, parent string) ([]archive.Change, error)
	// ApplyDiff extracts the changeset from the given diff into the
	// layer with the specified id and parent, returning the size of the
	// new layer in bytes.
	. ApplyDiff(id, parent string, diff archive.ArchiveReader) (size int64, err error)
	// DiffSize calculates the changes between the specified id
	// and its parent and returns the size in bytes of the changes
	// relative to its base filesystem directory.
	. DiffSize(id, parent string) (size int64, err error)
}
*/

func (d *Driver) String() string {
	return DriverName
}

// TODO: implement this similarly to the overlay driver
// GetMetadata not implemented
func (a *Driver) GetMetadata(id string) (map[string]string, error) {
	return nil, nil
}

// Read the layers file for the current id and return all the
// layers represented by new lines in the file
//
// If there are no lines in the file then the id has no parent
// and an empty slice is returned.
func (d *Driver) getParentIds(id string) ([]string, error) {
	f, err := os.Open(d.dir(LayersPath, id))
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

// XXX: copied + modified from AUFS
// Create 4 dirs for each id: mnt, layers, work and diff
// mnt and work are not used until Get is called, but we create them here anyway to
// avoid having to create them multiple times
func (d *Driver) Create(id, parent string) error {
	if err := d.createDirsFor(id); err != nil {
		return err
	}
	// Write the layers metadata (the stack of parents)
	// XXX: Should this use the uid/gid maps? (same with the aufs driver)
	f, err := os.Create(d.dir(LayersPath, id))
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
	for _, p := range AllDirPaths {
		if err := idtools.MkdirAllAs(d.dir(p, id), 0755, rootUID, rootGID); err != nil {
			return err
		}
	}
	return nil
}

// XXX: copied from AUFS driver
// Remove will unmount and remove the given id.
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
		if err := d.unmount(m); err != nil {
			return err
		}
	}
	tmpDirs := []string{
		MntPath,
		DiffPath,
		WorkPath,
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
	if err := os.Remove(d.dir(LayersPath, id)); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// XXX: should we do like the naive diff driver and "Get" the current layer and its direct parent and then diff them???
// Changes produces a list of changes between the specified layer
// and its parent layer. If parent is "", then all changes will be ADD changes.
func (d *Driver) Changes(id, parent string) ([]archive.Change, error) {
	// AUFS doesn't have snapshots, so we need to get changes from all parent
	// layers.
	layers, err := d.getParentLayerPaths(id)
	if err != nil {
		return nil, err
	}
	return archive.Changes(layers, d.dir(DiffPath, id))
}

// XXX: copied from the overlay driver
// XXX: how does this handle a large number of ro dirs? Do we have to find a way to do a similar hack to the AUFS driver
// to make it work with a large number of dirs? Do we have to do intermediate mounts? Where will metadata about that be stored?
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
	m.path = d.dir(DiffPath, id)
	if len(ids) > 0 {
		m.path = d.dir(MntPath, id)
		if m.referenceCount == 0 {
			if err := d.mount(id, m, mountLabel); err != nil {
				return "", err
			}
		}
	}
	m.referenceCount++
	return m.path, nil
}

// XXX: TODO: handle an unlimited number of parents
func (d *Driver) mount(id string, m *ActiveMount, mountLabel string) error {
	// If the id is mounted or we get an error return
	if mounted, err := d.mounted(m); err != nil || mounted {
		return err
	}

	layers, err := d.getParentLayerPaths(id)
	if err != nil {
		return err
	}

	upperDir := d.dir(DiffPath, id)
	workDir := d.dir(WorkPath, id)
	mergedDir := d.dir(MntPath, id)

	// the lowerdirs are in order from highest to lowest
	lowerDirs := strings.Join(layers, ":")

	opts := fmt.Sprintf("lowerdir=%s,upperdir=%s,workdir=%s", lowerDirs, upperDir, workDir)
	// XXX: If the options are longer than the page size (usually 4 KB - 1 for the null terminator), we need to break up the lower layers into multiple mounts and keep intermediate mount info somewhere (so that we can unmount correctly)
	if err := syscall.Mount("overlay", mergedDir, "overlay", 0, label.FormatMountLabel(opts, mountLabel)); err != nil {
		return fmt.Errorf("error creating overlay mount to %s: %v", mergedDir, err)
	}
	// chown "workdir/work" to the remapped root UID/GID. Overlay fs inside a
	// user namespace requires this to move a directory from lower to upper.
	rootUID, rootGID, err := idtools.GetRootUIDGID(d.uidMaps, d.gidMaps)
	if err := os.Chown(workDir, rootUID, rootGID); err != nil {
		return err
	}
	// XXX: make sure that m.path == mergedDir, maybe change the signature of this function?

	return nil
}

// XXX: copied from AUFS
// Put unmounts and updates list of active mounts.
func (d *Driver) Put(id string) error {
	// Protect the d.active from concurrent access
	d.Lock()
	defer d.Unlock()

	m := d.active[id]
	if m == nil {
		// TODO: we need to make sure that if we Put an id that has not been Get we try to make sure it's unmounted
		// https://github.com/docker/docker/commit/3916561619d45a3d8ca17dfa467149824111023a
		return nil
	}
	if count := m.referenceCount; count > 1 {
		m.referenceCount = count - 1
	} else {
		ids, _ := d.getParentIds(id)
		// We only mounted if there are any parents
		if ids != nil && len(ids) > 0 {
			d.unmount(m)
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
		layers[i] = d.dir(DiffPath, p)
	}
	return layers, nil
}

func (d *Driver) unmount(m *ActiveMount) error {
	if mounted, err := d.mounted(m); err != nil || !mounted {
		return err
	}
	if err := syscall.Unmount(m.path, 0); err != nil {
		return err
	}
	return nil
}

func (d *Driver) mounted(m *ActiveMount) (bool, error) {
	return mountpk.Mounted(m.path)
}

// XXX: copied + modified from AUFS
// Status returns current information about the filesystem such as root directory, number of directories mounted, etc.
func (d *Driver) Status() [][2]string {
	ids, _ := loadIds(path.Join(d.root, LayersPath))
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
	return archive.TarWithOptions(d.dir(DiffPath, id), &archive.TarOptions{
		Compression:   archive.Uncompressed,
		UIDMaps:       d.uidMaps,
		GIDMaps:       d.gidMaps,
		OverlayFormat: true,
	})
}

func (d *Driver) Cleanup() error {
	return nil
}

// DiffSize calculates the changes between the specified id
// and its parent and returns the size in bytes of the changes
// relative to its base filesystem directory.
func (d *Driver) DiffSize(id, parent string) (size int64, err error) {
	// AUFS doesn't need the parent layer to calculate the diff size.
	// XXX: is this the size on disk or the size that will be in the tar?
	return directory.Size(d.dir(DiffPath, id))
}

// ApplyDiff extracts the changeset from the given diff into the
// layer with the specified id and parent, returning the size of the
// new layer in bytes.
func (d *Driver) ApplyDiff(id, parent string, diff archive.Reader) (size int64, err error) {
	// AUFS doesn't need the parent id to apply the diff.
	if err := chrootarchive.UntarUncompressed(diff, d.dir(DiffPath, id), &archive.TarOptions{
		UIDMaps:       d.uidMaps,
		GIDMaps:       d.gidMaps,
		OverlayFormat: true,
	}); err != nil {
		return 0, err
	}

	return d.DiffSize(id, parent)
}

// XXX: copied from aufs
// Exists returns true if the given id is registered with
// this driver
func (d *Driver) Exists(id string) bool {
	if _, err := os.Lstat(d.dir(LayersPath, id)); err != nil {
		return false
	}
	return true
}

// dir returns the directory for the given kind of path for the given container id
// kind can be one of LayersPath, DiffPath, MntPath, WorkPath
func (d *Driver) dir(kind, id string) string {
	return path.Join(d.root, kind, id)
}

// XXX: copied from aufs
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

// Differences between Overlay and AUFS drivers

// the overlay driver uses "mounted" to indicated whether or not the filesystem was ever mounted. If it was never mounted, it doesn't try to unmount it
// aufs unmounts only if the dir is mounted

// overlay uses Stat in Exists but aufs uses lstat

// overlay does nothing in Cleanup, but aufs actually unmounts everything - why? what should we do?
