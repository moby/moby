// +build linux

/*

aufs driver directory structure

  .
  ├── layers // Metadata of layers
  │   ├── 1
  │   ├── 2
  │   └── 3
  ├── diff  // Content of the layer
  │   ├── 1  // Contains layers that need to be mounted for the id
  │   ├── 2
  │   └── 3
  └── mnt    // Mount points for the rw layers to be mounted
      ├── 1
      ├── 2
      └── 3

*/

package aufs

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
	"github.com/docker/docker/pkg/stringid"

	"github.com/opencontainers/runc/libcontainer/label"
)

var (
	// ErrAufsNotSupported is returned if aufs is not supported by the host.
	ErrAufsNotSupported = fmt.Errorf("AUFS was not found in /proc/filesystems")
	incompatibleFsMagic = []graphdriver.FsMagic{
		graphdriver.FsMagicBtrfs,
		graphdriver.FsMagicAufs,
	}
	backingFs = "<unknown>"

	enableDirpermLock sync.Once
	enableDirperm     bool
)

func init() {
	graphdriver.Register("aufs", Init)
}

type data struct {
	referenceCount int
	path           string
}

// Driver contains information about the filesystem mounted.
// root of the filesystem
// sync.Mutex to protect against concurrent modifications
// active maps mount id to the count
type Driver struct {
	root       string
	uidMaps    []idtools.IDMap
	gidMaps    []idtools.IDMap
	sync.Mutex // Protects concurrent modification to active
	active     map[string]*data
}

// Init returns a new AUFS driver.
// An error is returned if AUFS is not supported.
func Init(root string, options []string, uidMaps, gidMaps []idtools.IDMap) (graphdriver.Driver, error) {

	// Try to load the aufs kernel module
	if err := supportsAufs(); err != nil {
		return nil, graphdriver.ErrNotSupported
	}

	fsMagic, err := graphdriver.GetFSMagic(root)
	if err != nil {
		return nil, err
	}
	if fsName, ok := graphdriver.FsNames[fsMagic]; ok {
		backingFs = fsName
	}

	for _, magic := range incompatibleFsMagic {
		if fsMagic == magic {
			return nil, graphdriver.ErrIncompatibleFS
		}
	}

	paths := []string{
		"mnt",
		"diff",
		"layers",
	}

	a := &Driver{
		root:    root,
		active:  make(map[string]*data),
		uidMaps: uidMaps,
		gidMaps: gidMaps,
	}

	rootUID, rootGID, err := idtools.GetRootUIDGID(uidMaps, gidMaps)
	if err != nil {
		return nil, err
	}
	// Create the root aufs driver dir and return
	// if it already exists
	// If not populate the dir structure
	if err := idtools.MkdirAllAs(root, 0755, rootUID, rootGID); err != nil {
		if os.IsExist(err) {
			return a, nil
		}
		return nil, err
	}

	if err := mountpk.MakePrivate(root); err != nil {
		return nil, err
	}

	// Populate the dir structure
	for _, p := range paths {
		if err := idtools.MkdirAllAs(path.Join(root, p), 0755, rootUID, rootGID); err != nil {
			return nil, err
		}
	}
	return a, nil
}

// Return a nil error if the kernel supports aufs
// We cannot modprobe because inside dind modprobe fails
// to run
func supportsAufs() error {
	// We can try to modprobe aufs first before looking at
	// proc/filesystems for when aufs is supported
	exec.Command("modprobe", "aufs").Run()

	f, err := os.Open("/proc/filesystems")
	if err != nil {
		return err
	}
	defer f.Close()

	s := bufio.NewScanner(f)
	for s.Scan() {
		if strings.Contains(s.Text(), "aufs") {
			return nil
		}
	}
	return ErrAufsNotSupported
}

func (a *Driver) rootPath() string {
	return a.root
}

func (*Driver) String() string {
	return "aufs"
}

// Status returns current information about the filesystem such as root directory, number of directories mounted, etc.
func (a *Driver) Status() [][2]string {
	ids, _ := loadIds(path.Join(a.rootPath(), "layers"))
	return [][2]string{
		{"Root Dir", a.rootPath()},
		{"Backing Filesystem", backingFs},
		{"Dirs", fmt.Sprintf("%d", len(ids))},
		{"Dirperm1 Supported", fmt.Sprintf("%v", useDirperm())},
	}
}

// GetMetadata not implemented
func (a *Driver) GetMetadata(id string) (map[string]string, error) {
	return nil, nil
}

// Exists returns true if the given id is registered with
// this driver
func (a *Driver) Exists(id string) bool {
	if _, err := os.Lstat(path.Join(a.rootPath(), "layers", id)); err != nil {
		return false
	}
	return true
}

// Create three folders for each id
// mnt, layers, and diff
func (a *Driver) Create(id, parent, mountLabel string) error {
	if err := a.createDirsFor(id); err != nil {
		return err
	}
	// Write the layers metadata
	f, err := os.Create(path.Join(a.rootPath(), "layers", id))
	if err != nil {
		return err
	}
	defer f.Close()

	if parent != "" {
		ids, err := getParentIds(a.rootPath(), parent)
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
	a.active[id] = &data{}
	return nil
}

func (a *Driver) createDirsFor(id string) error {
	paths := []string{
		"mnt",
		"diff",
	}

	rootUID, rootGID, err := idtools.GetRootUIDGID(a.uidMaps, a.gidMaps)
	if err != nil {
		return err
	}
	for _, p := range paths {
		if err := idtools.MkdirAllAs(path.Join(a.rootPath(), p, id), 0755, rootUID, rootGID); err != nil {
			return err
		}
	}
	return nil
}

// Remove will unmount and remove the given id.
func (a *Driver) Remove(id string) error {
	// Protect the a.active from concurrent access
	a.Lock()
	defer a.Unlock()

	m := a.active[id]
	if m != nil {
		if m.referenceCount > 0 {
			return nil
		}
		// Make sure the dir is umounted first
		if err := a.unmount(m); err != nil {
			return err
		}
	}
	tmpDirs := []string{
		"mnt",
		"diff",
	}

	// Atomically remove each directory in turn by first moving it out of the
	// way (so that docker doesn't find it anymore) before doing removal of
	// the whole tree.
	for _, p := range tmpDirs {
		realPath := path.Join(a.rootPath(), p, id)
		tmpPath := path.Join(a.rootPath(), p, fmt.Sprintf("%s-removing", id))
		if err := os.Rename(realPath, tmpPath); err != nil && !os.IsNotExist(err) {
			return err
		}
		defer os.RemoveAll(tmpPath)
	}
	// Remove the layers file for the id
	if err := os.Remove(path.Join(a.rootPath(), "layers", id)); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// Get returns the rootfs path for the id.
// This will mount the dir at it's given path
func (a *Driver) Get(id, mountLabel string) (string, error) {
	ids, err := getParentIds(a.rootPath(), id)
	if err != nil {
		if !os.IsNotExist(err) {
			return "", err
		}
		ids = []string{}
	}

	// Protect the a.active from concurrent access
	a.Lock()
	defer a.Unlock()

	m := a.active[id]
	if m == nil {
		m = &data{}
		a.active[id] = m
	}

	// If a dir does not have a parent ( no layers )do not try to mount
	// just return the diff path to the data
	m.path = path.Join(a.rootPath(), "diff", id)
	if len(ids) > 0 {
		m.path = path.Join(a.rootPath(), "mnt", id)
		if m.referenceCount == 0 {
			if err := a.mount(id, m, mountLabel); err != nil {
				return "", err
			}
		}
	}
	m.referenceCount++
	return m.path, nil
}

// Put unmounts and updates list of active mounts.
func (a *Driver) Put(id string) error {
	// Protect the a.active from concurrent access
	a.Lock()
	defer a.Unlock()

	m := a.active[id]
	if m == nil {
		return nil
	}
	if count := m.referenceCount; count > 1 {
		m.referenceCount = count - 1
	} else {
		ids, _ := getParentIds(a.rootPath(), id)
		// We only mounted if there are any parents
		if ids != nil && len(ids) > 0 {
			a.unmount(m)
		}
		delete(a.active, id)
	}
	return nil
}

// Diff produces an archive of the changes between the specified
// layer and its parent layer which may be "".
func (a *Driver) Diff(id, parent string) (archive.Archive, error) {
	// AUFS doesn't need the parent layer to produce a diff.
	return archive.TarWithOptions(path.Join(a.rootPath(), "diff", id), &archive.TarOptions{
		Compression:     archive.Uncompressed,
		ExcludePatterns: []string{archive.WhiteoutMetaPrefix + "*", "!" + archive.WhiteoutOpaqueDir},
		UIDMaps:         a.uidMaps,
		GIDMaps:         a.gidMaps,
	})
}

func (a *Driver) applyDiff(id string, diff archive.Reader) error {
	dir := path.Join(a.rootPath(), "diff", id)
	if err := chrootarchive.UntarUncompressed(diff, dir, &archive.TarOptions{
		UIDMaps: a.uidMaps,
		GIDMaps: a.gidMaps,
	}); err != nil {
		return err
	}

	// show invalid whiteouts warning.
	files, err := ioutil.ReadDir(path.Join(dir, archive.WhiteoutLinkDir))
	if err == nil && len(files) > 0 {
		logrus.Warnf("Archive contains aufs hardlink references that are not supported.")
	}
	return nil
}

// DiffSize calculates the changes between the specified id
// and its parent and returns the size in bytes of the changes
// relative to its base filesystem directory.
func (a *Driver) DiffSize(id, parent string) (size int64, err error) {
	// AUFS doesn't need the parent layer to calculate the diff size.
	return directory.Size(path.Join(a.rootPath(), "diff", id))
}

// ApplyDiff extracts the changeset from the given diff into the
// layer with the specified id and parent, returning the size of the
// new layer in bytes.
func (a *Driver) ApplyDiff(id, parent string, diff archive.Reader) (size int64, err error) {
	// AUFS doesn't need the parent id to apply the diff.
	if err = a.applyDiff(id, diff); err != nil {
		return
	}

	return a.DiffSize(id, parent)
}

// Changes produces a list of changes between the specified layer
// and its parent layer. If parent is "", then all changes will be ADD changes.
func (a *Driver) Changes(id, parent string) ([]archive.Change, error) {
	// AUFS doesn't have snapshots, so we need to get changes from all parent
	// layers.
	layers, err := a.getParentLayerPaths(id)
	if err != nil {
		return nil, err
	}
	return archive.Changes(layers, path.Join(a.rootPath(), "diff", id))
}

func (a *Driver) getParentLayerPaths(id string) ([]string, error) {
	parentIds, err := getParentIds(a.rootPath(), id)
	if err != nil {
		return nil, err
	}
	layers := make([]string, len(parentIds))

	// Get the diff paths for all the parent ids
	for i, p := range parentIds {
		layers[i] = path.Join(a.rootPath(), "diff", p)
	}
	return layers, nil
}

func (a *Driver) mount(id string, m *data, mountLabel string) error {
	// If the id is mounted or we get an error return
	if mounted, err := a.mounted(m); err != nil || mounted {
		return err
	}

	var (
		target = m.path
		rw     = path.Join(a.rootPath(), "diff", id)
	)

	layers, err := a.getParentLayerPaths(id)
	if err != nil {
		return err
	}

	if err := a.aufsMount(layers, rw, target, mountLabel); err != nil {
		return fmt.Errorf("error creating aufs mount to %s: %v", target, err)
	}
	return nil
}

func (a *Driver) unmount(m *data) error {
	if mounted, err := a.mounted(m); err != nil || !mounted {
		return err
	}
	return Unmount(m.path)
}

func (a *Driver) mounted(m *data) (bool, error) {
	return mountpk.Mounted(m.path)
}

// Cleanup aufs and unmount all mountpoints
func (a *Driver) Cleanup() error {
	for id, m := range a.active {
		if err := a.unmount(m); err != nil {
			logrus.Errorf("Unmounting %s: %s", stringid.TruncateID(id), err)
		}
	}
	return mountpk.Unmount(a.root)
}

func (a *Driver) aufsMount(ro []string, rw, target, mountLabel string) (err error) {
	defer func() {
		if err != nil {
			Unmount(target)
		}
	}()

	// Mount options are clipped to page size(4096 bytes). If there are more
	// layers then these are remounted individually using append.

	offset := 54
	if useDirperm() {
		offset += len("dirperm1")
	}
	b := make([]byte, syscall.Getpagesize()-len(mountLabel)-offset) // room for xino & mountLabel
	bp := copy(b, fmt.Sprintf("br:%s=rw", rw))

	firstMount := true
	i := 0

	for {
		for ; i < len(ro); i++ {
			layer := fmt.Sprintf(":%s=ro+wh", ro[i])

			if firstMount {
				if bp+len(layer) > len(b) {
					break
				}
				bp += copy(b[bp:], layer)
			} else {
				data := label.FormatMountLabel(fmt.Sprintf("append%s", layer), mountLabel)
				if err = mount("none", target, "aufs", syscall.MS_REMOUNT, data); err != nil {
					return
				}
			}
		}

		if firstMount {
			opts := "dio,noplink,xino=/dev/shm/aufs.xino"
			if useDirperm() {
				opts += ",dirperm1"
			}
			data := label.FormatMountLabel(fmt.Sprintf("%s,%s", string(b[:bp]), opts), mountLabel)
			if err = mount("none", target, "aufs", 0, data); err != nil {
				return
			}
			firstMount = false
		}

		if i == len(ro) {
			break
		}
	}

	return
}

// useDirperm checks dirperm1 mount option can be used with the current
// version of aufs.
func useDirperm() bool {
	enableDirpermLock.Do(func() {
		base, err := ioutil.TempDir("", "docker-aufs-base")
		if err != nil {
			logrus.Errorf("error checking dirperm1: %v", err)
			return
		}
		defer os.RemoveAll(base)

		union, err := ioutil.TempDir("", "docker-aufs-union")
		if err != nil {
			logrus.Errorf("error checking dirperm1: %v", err)
			return
		}
		defer os.RemoveAll(union)

		opts := fmt.Sprintf("br:%s,dirperm1,xino=/dev/shm/aufs.xino", base)
		if err := mount("none", union, "aufs", 0, opts); err != nil {
			return
		}
		enableDirperm = true
		if err := Unmount(union); err != nil {
			logrus.Errorf("error checking dirperm1: failed to unmount %v", err)
		}
	})
	return enableDirperm
}
