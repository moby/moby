package vfs

import (
	"fmt"
	"os"
	"os/exec"
	"path"

	"github.com/dotcloud/docker/archive"
	"github.com/dotcloud/docker/daemon/graphdriver"
)

func init() {
	graphdriver.Register("vfs", Init)
}

func Init(home string, options []string) (graphdriver.Driver, error) {
	d := &Driver{
		home: home,
	}
	return d, nil
}

type Driver struct {
	home string
}

func (d *Driver) String() string {
	return "vfs"
}

func (d *Driver) Status() [][2]string {
	return nil
}

func (d *Driver) Cleanup() error {
	return nil
}

func copyDir(src, dst string) error {
	if output, err := exec.Command("cp", "-aT", "--reflink=auto", src, dst).CombinedOutput(); err != nil {
		return fmt.Errorf("Error VFS copying directory: %s (%s)", err, output)
	}
	return nil
}

func (d *Driver) Create(id, parent string) error {
	dir := d.dir(id)
	if err := os.MkdirAll(path.Dir(dir), 0700); err != nil {
		return err
	}
	if err := os.Mkdir(dir, 0755); err != nil {
		return err
	}
	if parent == "" {
		return nil
	}
	parentDir, err := d.Get(parent, "")
	if err != nil {
		return fmt.Errorf("%s: %s", parent, err)
	}
	if err := copyDir(parentDir, dir); err != nil {
		return err
	}
	return nil
}

func (d *Driver) CreateWithParent(newID, parentID, startID, endID string) error {
	var (
		layerData archive.Archive
		err       error
	)
	cDir, err := d.Get(endID, "")
	if err != nil {
		return fmt.Errorf("Error getting container rootfs %s from driver %s: %s", endID, d, err)
	}
	defer d.Put(endID)

	initDir, err := d.Get(startID, "")
	if err != nil {
		return fmt.Errorf("Error getting container init rootfs %s from driver %s: %s", startID, d, err)
	}
	defer d.Put(startID)

	changes, err := archive.ChangesDirs(cDir, initDir)
	if err != nil {
		return fmt.Errorf("Error getting changes between %s and %s from driver %s: %s", initDir, cDir, d, err)
	}

	layerData, err = archive.ExportChanges(cDir, changes)
	if err != nil {
		return fmt.Errorf("Error getting the archive with changes from %s from driver %s: %s", cDir, d, err)
	}

	defer layerData.Close()
	if err := d.Create(newID, parentID); err != nil {
		return fmt.Errorf("Driver %s failed to create image rootfs %s: %s", d, newID, err)
	}
	newImagePath, err := d.Get(newID, "")
	if err != nil {
		return fmt.Errorf("Error getting image rootfs %s from driver %s: %s", newID, d, err)
	}
	defer d.Put(newID)

	if err = archive.ApplyLayer(newImagePath, layerData); err != nil {
		return fmt.Errorf("Error applying changes from %s to %s from driver %s: %s", cDir, newID, d, err)
	}

	return nil
}

func (d *Driver) dir(id string) string {
	return path.Join(d.home, "dir", path.Base(id))
}

func (d *Driver) Remove(id string) error {
	if _, err := os.Stat(d.dir(id)); err != nil {
		return err
	}
	return os.RemoveAll(d.dir(id))
}

func (d *Driver) Get(id, mountLabel string) (string, error) {
	dir := d.dir(id)
	if st, err := os.Stat(dir); err != nil {
		return "", err
	} else if !st.IsDir() {
		return "", fmt.Errorf("%s: not a directory", dir)
	}
	return dir, nil
}

func (d *Driver) Put(id string) {
	// The vfs driver has no runtime resources (e.g. mounts)
	// to clean up, so we don't need anything here
}

func (d *Driver) Exists(id string) bool {
	_, err := os.Stat(d.dir(id))
	return err == nil
}
