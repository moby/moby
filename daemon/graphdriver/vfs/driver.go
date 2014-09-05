package vfs

import (
	"bytes"
	"fmt"
	"github.com/docker/docker/daemon/graphdriver"
	"os"
	"os/exec"
	"path"
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

func isGNUcoreutils() bool {
	if stdout, err := exec.Command("cp", "--version").Output(); err == nil {
		return bytes.Contains(stdout, []byte("GNU coreutils"))
	}

	return false
}

func copyDir(src, dst string) error {
	argv := make([]string, 0, 4)

	if isGNUcoreutils() {
		argv = append(argv, "-aT", "--reflink=auto", src, dst)
	} else {
		argv = append(argv, "-a", src+"/.", dst+"/.")
	}

	if output, err := exec.Command("cp", argv...).CombinedOutput(); err != nil {
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
