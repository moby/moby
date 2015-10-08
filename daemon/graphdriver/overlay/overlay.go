// +build linux

package overlay

import (
	"bufio"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"sync"
	"syscall"

	"github.com/Sirupsen/logrus"

	"github.com/docker/docker/daemon/graphdriver"
	"github.com/docker/docker/pkg/archive"
	"github.com/docker/docker/pkg/chrootarchive"
	"github.com/docker/docker/pkg/idtools"

	"github.com/opencontainers/runc/libcontainer/label"
)

// This is a small wrapper over the NaiveDiffWriter that lets us have a custom
// implementation of ApplyDiff()

var (
	// ErrApplyDiffFallback is returned to indicate that a normal ApplyDiff is applied as a fallback from Naive diff writer.
	ErrApplyDiffFallback = fmt.Errorf("Fall back to normal ApplyDiff")
)

// ApplyDiffProtoDriver wraps the ProtoDriver by extending the inteface with ApplyDiff method.
type ApplyDiffProtoDriver interface {
	graphdriver.ProtoDriver
	// ApplyDiff writes the diff to the archive for the given id and parent id.
	// It returns the size in bytes written if successful, an error ErrApplyDiffFallback is returned otherwise.
	ApplyDiff(id, parent string, diff archive.Reader) (size int64, err error)
}

type naiveDiffDriverWithApply struct {
	graphdriver.Driver
	applyDiff ApplyDiffProtoDriver
}

// NaiveDiffDriverWithApply returns a NaiveDiff driver with custom ApplyDiff.
func NaiveDiffDriverWithApply(driver ApplyDiffProtoDriver, uidMaps, gidMaps []idtools.IDMap) graphdriver.Driver {
	return &naiveDiffDriverWithApply{
		Driver:    graphdriver.NewNaiveDiffDriver(driver, uidMaps, gidMaps),
		applyDiff: driver,
	}
}

// ApplyDiff creates a diff layer with either the NaiveDiffDriver or with a fallback.
func (d *naiveDiffDriverWithApply) ApplyDiff(id, parent string, diff archive.Reader) (int64, error) {
	b, err := d.applyDiff.ApplyDiff(id, parent, diff)
	if err == ErrApplyDiffFallback {
		return d.Driver.ApplyDiff(id, parent, diff)
	}
	return b, err
}

// This backend uses the overlay union filesystem for containers
// plus hard link file sharing for images.

// Each container/image can have a "root" subdirectory which is a plain
// filesystem hierarchy, or they can use overlay.

// If they use overlay there is a "upper" directory and a "lower-id"
// file, as well as "merged" and "work" directories. The "upper"
// directory has the upper layer of the overlay, and "lower-id" contains
// the id of the parent whose "root" directory shall be used as the lower
// layer in the overlay. The overlay itself is mounted in the "merged"
// directory, and the "work" dir is needed for overlay to work.

// When a overlay layer is created there are two cases, either the
// parent has a "root" dir, then we start out with a empty "upper"
// directory overlaid on the parents root. This is typically the
// case with the init layer of a container which is based on an image.
// If there is no "root" in the parent, we inherit the lower-id from
// the parent and start by making a copy in the parent's "upper" dir.
// This is typically the case for a container layer which copies
// its parent -init upper layer.

// Additionally we also have a custom implementation of ApplyLayer
// which makes a recursive copy of the parent "root" layer using
// hardlinks to share file data, and then applies the layer on top
// of that. This means all child images share file (but not directory)
// data with the parent.

// ActiveMount contains information about the count, path and whether is mounted or not.
// This information is part of the Driver, that contains list of active mounts that are part of this overlay.
type ActiveMount struct {
	count   int
	path    string
	mounted bool
}

// Driver contains information about the home directory and the list of active mounts that are created using this driver.
type Driver struct {
	home       string
	sync.Mutex // Protects concurrent modification to active
	active     map[string]*ActiveMount
	uidMaps    []idtools.IDMap
	gidMaps    []idtools.IDMap
}

var backingFs = "<unknown>"

func init() {
	graphdriver.Register("overlay", Init)
}

// Init returns the NaiveDiffDriver, a native diff driver for overlay filesystem.
// If overlay filesystem is not supported on the host, graphdriver.ErrNotSupported is returned as error.
// If a overlay filesystem is not supported over a existing filesystem then error graphdriver.ErrIncompatibleFS is returned.
func Init(home string, options []string, uidMaps, gidMaps []idtools.IDMap) (graphdriver.Driver, error) {

	if err := supportsOverlay(); err != nil {
		return nil, graphdriver.ErrNotSupported
	}

	fsMagic, err := graphdriver.GetFSMagic(home)
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
	// Create the driver home dir
	if err := idtools.MkdirAllAs(home, 0755, rootUID, rootGID); err != nil && !os.IsExist(err) {
		return nil, err
	}

	d := &Driver{
		home:    home,
		active:  make(map[string]*ActiveMount),
		uidMaps: uidMaps,
		gidMaps: gidMaps,
	}

	return NaiveDiffDriverWithApply(d, uidMaps, gidMaps), nil
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

func (d *Driver) String() string {
	return "overlay"
}

// Status returns current driver information in a two dimensional string array.
// Output contains "Backing Filesystem" used in this implementation.
func (d *Driver) Status() [][2]string {
	return [][2]string{
		{"Backing Filesystem", backingFs},
	}
}

// GetMetadata returns meta data about the overlay driver such as root, LowerDir, UpperDir, WorkDir and MergeDir used to store data.
func (d *Driver) GetMetadata(id string) (map[string]string, error) {
	dir := d.dir(id)
	if _, err := os.Stat(dir); err != nil {
		return nil, err
	}

	metadata := make(map[string]string)

	// If id has a root, it is an image
	rootDir := path.Join(dir, "root")
	if _, err := os.Stat(rootDir); err == nil {
		metadata["RootDir"] = rootDir
		return metadata, nil
	}

	lowerID, err := ioutil.ReadFile(path.Join(dir, "lower-id"))
	if err != nil {
		return nil, err
	}

	metadata["LowerDir"] = path.Join(d.dir(string(lowerID)), "root")
	metadata["UpperDir"] = path.Join(dir, "upper")
	metadata["WorkDir"] = path.Join(dir, "work")
	metadata["MergedDir"] = path.Join(dir, "merged")

	return metadata, nil
}

// Cleanup simply returns nil and do not change the existing filesystem.
// This is required to satisfy the graphdriver.Driver interface.
func (d *Driver) Cleanup() error {
	return nil
}

// Create is used to create the upper, lower, and merge directories required for overlay fs for a given id.
// The parent filesystem is used to configure these directories for the overlay.
func (d *Driver) Create(id string, parent string) (retErr error) {
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

	// Toplevel images are just a "root" dir
	if parent == "" {
		if err := idtools.MkdirAs(path.Join(dir, "root"), 0755, rootUID, rootGID); err != nil {
			return err
		}
		return nil
	}

	parentDir := d.dir(parent)

	// Ensure parent exists
	if _, err := os.Lstat(parentDir); err != nil {
		return err
	}

	// If parent has a root, just do a overlay to it
	parentRoot := path.Join(parentDir, "root")

	if s, err := os.Lstat(parentRoot); err == nil {
		if err := os.Mkdir(path.Join(dir, "upper"), s.Mode()); err != nil {
			return err
		}
		if err := os.Mkdir(path.Join(dir, "work"), 0700); err != nil {
			return err
		}
		if err := idtools.MkdirAs(path.Join(dir, "merged"), 0700, rootUID, rootGID); err != nil {
			return err
		}
		if err := ioutil.WriteFile(path.Join(dir, "lower-id"), []byte(parent), 0666); err != nil {
			return err
		}
		return nil
	}

	// Otherwise, copy the upper and the lower-id from the parent

	lowerID, err := ioutil.ReadFile(path.Join(parentDir, "lower-id"))
	if err != nil {
		return err
	}

	if err := ioutil.WriteFile(path.Join(dir, "lower-id"), lowerID, 0666); err != nil {
		return err
	}

	parentUpperDir := path.Join(parentDir, "upper")
	s, err := os.Lstat(parentUpperDir)
	if err != nil {
		return err
	}

	upperDir := path.Join(dir, "upper")
	if err := os.Mkdir(upperDir, s.Mode()); err != nil {
		return err
	}
	if err := os.Mkdir(path.Join(dir, "work"), 0700); err != nil {
		return err
	}
	if err := idtools.MkdirAs(path.Join(dir, "merged"), 0700, rootUID, rootGID); err != nil {
		return err
	}

	return copyDir(parentUpperDir, upperDir, 0)
}

func (d *Driver) dir(id string) string {
	return path.Join(d.home, id)
}

// Remove cleans the directories that are created for this id.
func (d *Driver) Remove(id string) error {
	return os.RemoveAll(d.dir(id))
}

// Get creates and mounts the required file system for the given id and returns the mount path.
func (d *Driver) Get(id string, mountLabel string) (string, error) {
	// Protect the d.active from concurrent access
	d.Lock()
	defer d.Unlock()

	mount := d.active[id]
	if mount != nil {
		mount.count++
		return mount.path, nil
	}

	mount = &ActiveMount{count: 1}

	dir := d.dir(id)
	if _, err := os.Stat(dir); err != nil {
		return "", err
	}

	// If id has a root, just return it
	rootDir := path.Join(dir, "root")
	if _, err := os.Stat(rootDir); err == nil {
		mount.path = rootDir
		d.active[id] = mount
		return mount.path, nil
	}

	lowerID, err := ioutil.ReadFile(path.Join(dir, "lower-id"))
	if err != nil {
		return "", err
	}
	lowerDir := path.Join(d.dir(string(lowerID)), "root")
	upperDir := path.Join(dir, "upper")
	workDir := path.Join(dir, "work")
	mergedDir := path.Join(dir, "merged")

	opts := fmt.Sprintf("lowerdir=%s,upperdir=%s,workdir=%s", lowerDir, upperDir, workDir)
	if err := syscall.Mount("overlay", mergedDir, "overlay", 0, label.FormatMountLabel(opts, mountLabel)); err != nil {
		return "", fmt.Errorf("error creating overlay mount to %s: %v", mergedDir, err)
	}
	// chown "workdir/work" to the remapped root UID/GID. Overlay fs inside a
	// user namespace requires this to move a directory from lower to upper.
	rootUID, rootGID, err := idtools.GetRootUIDGID(d.uidMaps, d.gidMaps)
	if err := os.Chown(path.Join(workDir, "work"), rootUID, rootGID); err != nil {
		return "", err
	}
	mount.path = mergedDir
	mount.mounted = true
	d.active[id] = mount

	return mount.path, nil
}

// Put unmounts the mount path created for the give id.
func (d *Driver) Put(id string) error {
	// Protect the d.active from concurrent access
	d.Lock()
	defer d.Unlock()

	mount := d.active[id]
	if mount == nil {
		logrus.Debugf("Put on a non-mounted device %s", id)
		// but it might be still here
		if d.Exists(id) {
			mergedDir := path.Join(d.dir(id), "merged")
			err := syscall.Unmount(mergedDir, 0)
			if err != nil {
				logrus.Debugf("Failed to unmount %s overlay: %v", id, err)
			}
		}
		return nil
	}

	mount.count--
	if mount.count > 0 {
		return nil
	}

	defer delete(d.active, id)
	if mount.mounted {
		err := syscall.Unmount(mount.path, 0)
		if err != nil {
			logrus.Debugf("Failed to unmount %s overlay: %v", id, err)
		}
		return err
	}
	return nil
}

// ApplyDiff applies the new layer on top of the root, if parent does not exist with will return a ErrApplyDiffFallback error.
func (d *Driver) ApplyDiff(id string, parent string, diff archive.Reader) (size int64, err error) {
	dir := d.dir(id)

	if parent == "" {
		return 0, ErrApplyDiffFallback
	}

	parentRootDir := path.Join(d.dir(parent), "root")
	if _, err := os.Stat(parentRootDir); err != nil {
		return 0, ErrApplyDiffFallback
	}

	// We now know there is a parent, and it has a "root" directory containing
	// the full root filesystem. We can just hardlink it and apply the
	// layer. This relies on two things:
	// 1) ApplyDiff is only run once on a clean (no writes to upper layer) container
	// 2) ApplyDiff doesn't do any in-place writes to files (would break hardlinks)
	// These are all currently true and are not expected to break

	tmpRootDir, err := ioutil.TempDir(dir, "tmproot")
	if err != nil {
		return 0, err
	}
	defer func() {
		if err != nil {
			os.RemoveAll(tmpRootDir)
		} else {
			os.RemoveAll(path.Join(dir, "upper"))
			os.RemoveAll(path.Join(dir, "work"))
			os.RemoveAll(path.Join(dir, "merged"))
			os.RemoveAll(path.Join(dir, "lower-id"))
		}
	}()

	if err = copyDir(parentRootDir, tmpRootDir, copyHardlink); err != nil {
		return 0, err
	}

	options := &archive.TarOptions{UIDMaps: d.uidMaps, GIDMaps: d.gidMaps}
	if size, err = chrootarchive.ApplyUncompressedLayer(tmpRootDir, diff, options); err != nil {
		return 0, err
	}

	rootDir := path.Join(dir, "root")
	if err := os.Rename(tmpRootDir, rootDir); err != nil {
		return 0, err
	}

	return
}

// Exists checks to see if the id is already mounted.
func (d *Driver) Exists(id string) bool {
	_, err := os.Stat(d.dir(id))
	return err == nil
}
