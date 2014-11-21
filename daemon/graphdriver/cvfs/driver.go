// +build linux

package cvfs

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"syscall"

	"github.com/docker/docker/daemon/graphdriver"
	"github.com/docker/libcontainer/label"
)
func String() string {
	return "cvfs"
}

func init() {
	graphdriver.Register("cvfs", Init)
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
	return "cvfs"
}

func (d *Driver) Status() [][2]string {
	return nil
}

func (d *Driver) Cleanup() error {
	return nil
}

func isGNUcoreutils() bool {
	if stdout, err := exec.Command("cp", "--version").Output(); err == nil {
		return bytes.Contains(stdout, []byte("GNU coreutils"))
	}

	return false
}

func (d *Driver) Create(id, parent string) error {
	dir := d.dir(id)
	if err := os.MkdirAll(path.Dir(dir), 0700); err != nil {
		return err
	}
	if err := os.Mkdir(dir, 0755); err != nil {
		return err
	}
	opts := []string{"level:s0"}
	if _, mountLabel, err := label.InitLabels(opts); err == nil {
		label.Relabel(dir, mountLabel, "")
	}
	if parent == "" {
		return nil
	}
	parentDir, err := d.Get(parent, "")
	if err != nil {
		return fmt.Errorf("%s: %s", parent, err)
	}
	if err = d.recursivelyHardlink(parentDir, dir); err != nil {
		return err
	}
	return nil
}

func (d *Driver) mkHardLink(source string, dest string) filepath.WalkFunc {
	return func(path string, info os.FileInfo, err error) error {
		subpath, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		newpath := filepath.Join(dest, subpath)
		if (info.IsDir()) {
			if err = os.MkdirAll(newpath, info.Mode()); err != nil {
				return err
			}
			stat := info.Sys().(*syscall.Stat_t)
			if err = os.Chown(newpath, int(stat.Uid), int(stat.Gid)); err != nil {
				return err
			}
			return nil
		} else {
			return os.Link(path, newpath)
		}
	}
}

func (d *Driver) recursivelyHardlink(source string, dest string) error {
	hardlink := d.mkHardLink(source, dest)
	return filepath.Walk(source, hardlink)
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
	// The cvfs driver has no runtime resources (e.g. mounts)
	// to clean up, so we don't need anything here
}

func (d *Driver) Exists(id string) bool {
	_, err := os.Stat(d.dir(id))
	return err == nil
}
