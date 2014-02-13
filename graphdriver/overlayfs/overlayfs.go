// +build linux

package overlayfs

import (
	"fmt"
	"github.com/dotcloud/docker/archive"
	"github.com/dotcloud/docker/graphdriver"
	"github.com/dotcloud/docker/utils"
	"io/ioutil"
	"os"
	"path"
	"sync"
	"syscall"
)

type ActiveMount struct {
	count   int
	path    string
	mounted bool
}

type Driver struct {
	home       string
	sync.Mutex // Protects concurrent modification to active
	active     map[string]*ActiveMount
}

func init() {
	graphdriver.Register("overlayfs", Init)
}
func Init(home string) (graphdriver.Driver, error) {
	// Create the driver home dir
	if err := os.MkdirAll(home, 0755); err != nil && !os.IsExist(err) {
		return nil, err
	}

	d := &Driver{
		home:   home,
		active: make(map[string]*ActiveMount),
	}

	return d, nil
}

func (d *Driver) String() string {
	return "overlayfs"
}

func (d *Driver) Status() [][2]string {
	return nil
}

func (d *Driver) Cleanup() error {
	return nil
}

func (d *Driver) Create(id string, parent string) (retErr error) {
	dir := d.dir(id)
	if err := os.MkdirAll(path.Dir(dir), 0700); err != nil {
		return err
	}
	if err := os.Mkdir(dir, 0700); err != nil {
		return err
	}

	if err := ioutil.WriteFile(path.Join(dir, "parent"), []byte(parent), 0666); err != nil {
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
		if err := os.Mkdir(path.Join(dir, "root"), 0700); err != nil {
			return err
		}
		return nil
	}

	parentDir := d.dir(parent)

	// Ensure parent exists
	if _, err := os.Lstat(parentDir); err != nil {
		return err
	}

	// If parent has a root, just do a overlayfs to it
	parentRoot := path.Join(parentDir, "root")

	if s, err := os.Lstat(parentRoot); err == nil {
		if err := os.Mkdir(path.Join(dir, "rw"), s.Mode()); err != nil {
			return err
		}
		if err := os.Mkdir(path.Join(dir, "overlay"), 0700); err != nil {
			return err
		}
		if err := ioutil.WriteFile(path.Join(dir, "rw-parent"), []byte(parent), 0666); err != nil {
			return err
		}
		return nil
	}

	// Otherwise, copy the rw and the parent-id from the parent

	rwParent, err := ioutil.ReadFile(path.Join(parentDir, "rw-parent"))
	if err != nil {
		return err
	}

	if err := ioutil.WriteFile(path.Join(dir, "rw-parent"), rwParent, 0666); err != nil {
		return err
	}

	rwDir := path.Join(dir, "rw")
	if err := os.Mkdir(rwDir, 0700); err != nil {
		return err
	}
	if err := os.Mkdir(path.Join(dir, "overlay"), 0700); err != nil {
		return err
	}

	parentRwDir := path.Join(parentDir, "rw")
	return copyDir(parentRwDir, rwDir, 0)
}

func (d *Driver) dir(id string) string {
	return path.Join(d.home, id)
}

func (d *Driver) Remove(id string) error {
	dir := d.dir(id)
	if _, err := os.Stat(dir); err != nil {
		return err
	}
	return os.RemoveAll(dir)
}

func (d *Driver) Get(id string) (string, error) {
	// Protect the d.active from concurrent access
	d.Lock()
	defer d.Unlock()

	mount := d.active[id]
	if mount != nil {
		mount.count++
		return mount.path, nil
	} else {
		mount = &ActiveMount{count: 1}
	}

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

	rwParent, err := ioutil.ReadFile(path.Join(dir, "rw-parent"))
	if err != nil {
		return "", err
	}
	rwParentRoot := path.Join(d.dir(string(rwParent)), "root")
	rwDir := path.Join(dir, "rw")
	overlayDir := path.Join(dir, "overlay")

	if err := syscall.Mount("overlayfs", overlayDir, "overlayfs", 0, fmt.Sprintf("lowerdir=%s,upperdir=%s", rwParentRoot, rwDir)); err != nil {
		return "", err
	}
	mount.path = overlayDir
	mount.mounted = true
	d.active[id] = mount

	return mount.path, nil
}

func (d *Driver) Put(id string) {
	// Protect the d.active from concurrent access
	d.Lock()
	defer d.Unlock()

	mount := d.active[id]
	if mount == nil {
		utils.Debugf("Put on a non-mounted device %s", id)
		return
	}

	mount.count--
	if mount.count > 0 {
		return
	}

	if mount.mounted {
		if err := syscall.Unmount(mount.path, 0); err != nil {
			utils.Debugf("Failed to unmount %s overlayfs: %v", id, err)
		}
	}

	delete(d.active, id)
}

func (d *Driver) Diff(id string) (retArchive archive.Archive, retErr error) {
	overlayDir, err := d.Get(id)
	if err != nil {
		return nil, err
	}
	defer func() {
		if retErr != nil {
			d.Put(id)
		}
	}()

	changes, err := d.Changes(id)
	if err != nil {
		return nil, err
	}

	archive, err := archive.ExportChanges(overlayDir, changes)
	if err != nil {
		return nil, err
	}

	return utils.NewReadCloserWrapper(archive, func() error {
		err := archive.Close()
		d.Put(id)
		return err
	}), nil
}

func (d *Driver) ApplyDiff(id string, diff archive.ArchiveReader) (retErr error) {
	dir := d.dir(id)

	parent, err := ioutil.ReadFile(path.Join(dir, "parent"))
	if err != nil {
		return err
	}

	hasParentRootDir := false
	parentRootDir := ""

	if string(parent) != "" {
		parentDir := d.dir(string(parent))

		// If parent has a root, we can just hardlink it and apply the
		// layer. This relies on two things:
		// 1) ApplyDiff is only run once on a clean (no rw data) container
		// 2) ApplyDiff doesn't do any in-place writes to files (would break hardlinks)
		// These are all currently true
		parentRootDir = path.Join(parentDir, "root")
		if _, err := os.Stat(parentRootDir); err == nil {
			hasParentRootDir = true
		}
	}

	if hasParentRootDir || string(parent) == "" {
		tmpRootDir, err := ioutil.TempDir(dir, "tmproot")
		if err != nil {
			return err
		}
		defer func() {
			if retErr != nil {
				os.RemoveAll(tmpRootDir)
			}
		}()

		if hasParentRootDir {
			if err := copyDir(parentRootDir, tmpRootDir, CopyHardlink); err != nil {
				return err
			}
		}

		if err := archive.ApplyLayer(tmpRootDir, diff); err != nil {
			return err
		}

		rootDir := path.Join(dir, "root")
		if err := os.Rename(tmpRootDir, rootDir); err != nil {
			return err
		}

		return nil
	}

	// Otherwise apply as normal

	overlayDir, err := d.Get(id)
	if err != nil {
		return err
	}
	defer d.Put(id)

	if err := archive.ApplyLayer(overlayDir, diff); err != nil {
		return err
	}

	return nil
}

func (d *Driver) DiffSize(id string) (int64, error) {
	overlayDir, err := d.Get(id)
	if err != nil {
		return -1, err
	}
	defer d.Put(id)

	changes, err := d.Changes(id)
	if err != nil {
		return -1, err
	}

	return archive.ChangesSize(overlayDir, changes), nil
}

func (d *Driver) Changes(id string) ([]archive.Change, error) {
	dir := d.dir(id)

	parent, err := ioutil.ReadFile(path.Join(dir, "parent"))
	if err != nil {
		return nil, err
	}

	overlayDir, err := d.Get(id)
	if err != nil {
		return nil, err
	}
	defer d.Put(id)

	parentOverlayDir := ""
	if string(parent) != "" {
		parentOverlayDir, err = d.Get(string(parent))
		if err != nil {
			return nil, err
		}
		defer d.Put(string(parent))
	}

	return archive.ChangesDirs(overlayDir, parentOverlayDir)
}

func (d *Driver) Exists(id string) bool {
	_, err := os.Stat(d.dir(id))
	return err == nil
}
