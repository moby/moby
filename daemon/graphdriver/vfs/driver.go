// +build linux

package vfs

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"strings"

	"github.com/docker/docker/daemon/graphdriver"
	"github.com/docker/docker/pkg/chrootarchive"
	"github.com/docker/docker/pkg/system"
	"github.com/docker/libcontainer/label"
)

func init() {
	graphdriver.Register("vfs", Init)
}

func Init(home string, options []string) (graphdriver.Driver, error) {
	d := &Driver{
		home: home,
	}
	return graphdriver.NaiveDiffDriver(d), nil
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

func (d *Driver) Create(id, parent string) error {
	dir := d.dir(id)
	if err := system.MkdirAll(path.Dir(dir), 0700); err != nil {
		return err
	}
	if err := os.Mkdir(dir, 0755); err != nil {
		return err
	}
	opts := []string{"level:s0"}
	if _, mountLabel, err := label.InitLabels(opts); err == nil {
		label.SetFileLabel(dir, mountLabel)
	}
	if parent == "" {
		return nil
	}
	parentDir, err := d.Get(parent, "")
	if err != nil {
		return fmt.Errorf("%s: %s", parent, err)
	}
	if strings.Contains(id, "-init") {
		idInitFile := d.initFile(id)
		if err := ioutil.WriteFile(idInitFile, []byte(parentDir), 0600); err != nil {
			return err
		}
		return nil
	}
	if strings.Contains(parent, "-init") {
		idInitFile := d.initFile(parent)
		if _, err := os.Stat(idInitFile); err != nil {
			return nil
		}
		data, err := ioutil.ReadFile(idInitFile)
		if err != nil {
			return err
		}
		parentDir = string(data)
	}
	if err := chrootarchive.CopyWithTar(parentDir, dir); err != nil {
		return err
	}
	return nil
}

func (d *Driver) initFile(id string) string {
	return path.Join(d.home, "dir", path.Base(id), "parentdir")
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
	if strings.Contains(id, "-init") {
		idInitFile := d.initFile(id)
		if _, err := os.Stat(idInitFile); err == nil {
			data, err := ioutil.ReadFile(idInitFile)
			if err != nil {
				return "", err
			}
			dir = string(data)
		}
	}
	if st, err := os.Stat(dir); err != nil {
		return "", err
	} else if !st.IsDir() {
		return "", fmt.Errorf("%s: not a directory", dir)
	}
	return dir, nil
}

func (d *Driver) Put(id string) error {
	// The vfs driver has no runtime resources (e.g. mounts)
	// to clean up, so we don't need anything here
	return nil
}

func (d *Driver) Exists(id string) bool {
	_, err := os.Stat(d.dir(id))
	return err == nil
}
