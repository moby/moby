/*

aufs driver directory structure

.
├── layers // Metadata of layers
│   ├── 1
│   ├── 2
│   └── 3
├── diff  // Content of the layer
│   ├── 1  // Contains layers that need to be mounted for the id
│   ├── 2
│   └── 3
└── mnt    // Mount points for the rw layers to be mounted
    ├── 1
    ├── 2
    └── 3

*/

package aufs

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path"
	"strings"
	"sync"
	"syscall"

	"github.com/dotcloud/docker/archive"
	"github.com/dotcloud/docker/daemon/graphdriver"
	"github.com/dotcloud/docker/pkg/label"
	mountpk "github.com/dotcloud/docker/pkg/mount"
	"github.com/dotcloud/docker/utils"
)

var (
	ErrAufsNotSupported = fmt.Errorf("AUFS was not found in /proc/filesystems")
	incompatibleFsMagic = []graphdriver.FsMagic{
		graphdriver.FsMagicBtrfs,
		graphdriver.FsMagicAufs,
	}
)

func init() {
	graphdriver.Register("aufs", Init)
}

type Driver struct {
	root       string
	sync.Mutex // Protects concurrent modification to active
	active     map[string]int
}

// New returns a new AUFS driver.
// An error is returned if AUFS is not supported.
func Init(root string, options []string) (graphdriver.Driver, error) {
	// Try to load the aufs kernel module
	if err := supportsAufs(); err != nil {
		return nil, graphdriver.ErrNotSupported
	}

	rootdir := path.Dir(root)

	var buf syscall.Statfs_t
	if err := syscall.Statfs(rootdir, &buf); err != nil {
		return nil, fmt.Errorf("Couldn't stat the root directory: %s", err)
	}

	for _, magic := range incompatibleFsMagic {
		if graphdriver.FsMagic(buf.Type) == magic {
			return nil, graphdriver.ErrIncompatibleFS
		}
	}

	paths := []string{
		"mnt",
		"diff",
		"layers",
	}

	a := &Driver{
		root:   root,
		active: make(map[string]int),
	}

	// Create the root aufs driver dir and return
	// if it already exists
	// If not populate the dir structure
	if err := os.MkdirAll(root, 0755); err != nil {
		if os.IsExist(err) {
			return a, nil
		}
		return nil, err
	}

	if err := graphdriver.MakePrivate(root); err != nil {
		return nil, err
	}

	for _, p := range paths {
		if err := os.MkdirAll(path.Join(root, p), 0755); err != nil {
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

func (a Driver) rootPath() string {
	return a.root
}

func (Driver) String() string {
	return "aufs"
}

func (a Driver) Status() [][2]string {
	ids, _ := loadIds(path.Join(a.rootPath(), "layers"))
	return [][2]string{
		{"Root Dir", a.rootPath()},
		{"Dirs", fmt.Sprintf("%d", len(ids))},
	}
}

// Exists returns true if the given id is registered with
// this driver
func (a Driver) Exists(id string) bool {
	if _, err := os.Lstat(path.Join(a.rootPath(), "layers", id)); err != nil {
		return false
	}
	return true
}

// Three folders are created for each id
// mnt, layers, and diff
func (a *Driver) Create(id, parent string) error {
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
	return nil
}

func (a *Driver) createDirsFor(id string) error {
	paths := []string{
		"mnt",
		"diff",
	}

	for _, p := range paths {
		if err := os.MkdirAll(path.Join(a.rootPath(), p, id), 0755); err != nil {
			return err
		}
	}
	return nil
}

// Unmount and remove the dir information
func (a *Driver) Remove(id string) error {
	// Protect the a.active from concurrent access
	a.Lock()
	defer a.Unlock()

	if a.active[id] != 0 {
		utils.Errorf("Warning: removing active id %s\n", id)
	}

	// Make sure the dir is umounted first
	if err := a.unmount(id); err != nil {
		return err
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

// Return the rootfs path for the id
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

	count := a.active[id]

	// If a dir does not have a parent ( no layers )do not try to mount
	// just return the diff path to the data
	out := path.Join(a.rootPath(), "diff", id)
	if len(ids) > 0 {
		out = path.Join(a.rootPath(), "mnt", id)

		if count == 0 {
			if err := a.mount(id, mountLabel); err != nil {
				return "", err
			}
		}
	}

	a.active[id] = count + 1

	return out, nil
}

func (a *Driver) Put(id string) {
	// Protect the a.active from concurrent access
	a.Lock()
	defer a.Unlock()

	if count := a.active[id]; count > 1 {
		a.active[id] = count - 1
	} else {
		ids, _ := getParentIds(a.rootPath(), id)
		// We only mounted if there are any parents
		if ids != nil && len(ids) > 0 {
			a.unmount(id)
		}
		delete(a.active, id)
	}
}

// Returns an archive of the contents for the id
func (a *Driver) Diff(id string) (archive.Archive, error) {
	return archive.TarFilter(path.Join(a.rootPath(), "diff", id), &archive.TarOptions{
		Compression: archive.Uncompressed,
	})
}

func (a *Driver) ApplyDiff(id string, diff archive.ArchiveReader) error {
	return archive.Untar(diff, path.Join(a.rootPath(), "diff", id), nil)
}

// Returns the size of the contents for the id
func (a *Driver) DiffSize(id string) (int64, error) {
	return utils.TreeSize(path.Join(a.rootPath(), "diff", id))
}

func (a *Driver) Changes(id string) ([]archive.Change, error) {
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
	if len(parentIds) == 0 {
		return nil, fmt.Errorf("Dir %s does not have any parent layers", id)
	}
	layers := make([]string, len(parentIds))

	// Get the diff paths for all the parent ids
	for i, p := range parentIds {
		layers[i] = path.Join(a.rootPath(), "diff", p)
	}
	return layers, nil
}

func (a *Driver) mount(id, mountLabel string) error {
	// If the id is mounted or we get an error return
	if mounted, err := a.mounted(id); err != nil || mounted {
		return err
	}

	var (
		target = path.Join(a.rootPath(), "mnt", id)
		rw     = path.Join(a.rootPath(), "diff", id)
	)

	layers, err := a.getParentLayerPaths(id)
	if err != nil {
		return err
	}

	if err := a.aufsMount(layers, rw, target, mountLabel); err != nil {
		return err
	}
	return nil
}

func (a *Driver) unmount(id string) error {
	if mounted, err := a.mounted(id); err != nil || !mounted {
		return err
	}
	target := path.Join(a.rootPath(), "mnt", id)
	return Unmount(target)
}

func (a *Driver) mounted(id string) (bool, error) {
	target := path.Join(a.rootPath(), "mnt", id)
	return mountpk.Mounted(target)
}

// During cleanup aufs needs to unmount all mountpoints
func (a *Driver) Cleanup() error {
	ids, err := loadIds(path.Join(a.rootPath(), "layers"))
	if err != nil {
		return err
	}

	for _, id := range ids {
		if err := a.unmount(id); err != nil {
			utils.Errorf("Unmounting %s: %s", utils.TruncateID(id), err)
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

	if err = a.tryMount(ro, rw, target, mountLabel); err != nil {
		if err = a.mountRw(rw, target, mountLabel); err != nil {
			return
		}

		for _, layer := range ro {
			data := label.FormatMountLabel(fmt.Sprintf("append:%s=ro+wh", layer), mountLabel)
			if err = mount("none", target, "aufs", MsRemount, data); err != nil {
				return
			}
		}
	}
	return
}

// Try to mount using the aufs fast path, if this fails then
// append ro layers.
func (a *Driver) tryMount(ro []string, rw, target, mountLabel string) (err error) {
	var (
		rwBranch   = fmt.Sprintf("%s=rw", rw)
		roBranches = fmt.Sprintf("%s=ro+wh:", strings.Join(ro, "=ro+wh:"))
		data       = label.FormatMountLabel(fmt.Sprintf("br:%v:%v,xino=/dev/shm/aufs.xino", rwBranch, roBranches), mountLabel)
	)
	return mount("none", target, "aufs", 0, data)
}

func (a *Driver) mountRw(rw, target, mountLabel string) error {
	data := label.FormatMountLabel(fmt.Sprintf("br:%s,xino=/dev/shm/aufs.xino", rw), mountLabel)
	return mount("none", target, "aufs", 0, data)
}

func rollbackMount(target string, err error) {
	if err != nil {
		Unmount(target)
	}
}
