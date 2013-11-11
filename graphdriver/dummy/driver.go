package dummy

import (
	"fmt"
	"github.com/dotcloud/docker/archive"
	"github.com/dotcloud/docker/graphdriver"
	"os"
	"path"
)

func init() {
	graphdriver.Register("dummy", Init)
}

func Init(home string) (graphdriver.Driver, error) {
	d := &Driver{
		home: home,
	}
	return d, nil
}

type Driver struct {
	home string
}

func (d *Driver) String() string {
	return "dummy"
}

func (d *Driver) Cleanup() error {
	return nil
}

func (d *Driver) Create(id string, parent string) error {
	dir := d.dir(id)
	if err := os.MkdirAll(path.Dir(dir), 0700); err != nil {
		return err
	}
	if err := os.Mkdir(dir, 0700); err != nil {
		return err
	}
	if parent == "" {
		return nil
	}
	parentDir, err := d.Get(parent)
	if err != nil {
		return fmt.Errorf("%s: %s", parent, err)
	}
	if err := archive.CopyWithTar(parentDir, dir); err != nil {
		return err
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

func (d *Driver) Get(id string) (string, error) {
	dir := d.dir(id)
	if st, err := os.Stat(dir); err != nil {
		return "", err
	} else if !st.IsDir() {
		return "", fmt.Errorf("%s: not a directory", dir)
	}
	return dir, nil
}

func (d *Driver) DiffSize(id string) (int64, error) {
	return -1, fmt.Errorf("Not implemented")
}

func (d *Driver) Diff(id string) (archive.Archive, error) {
	return nil, fmt.Errorf("Not implemented)")
}

func (d *Driver) Changes(id string) ([]archive.Change, error) {
	return nil, fmt.Errorf("asdlfj)")
}
