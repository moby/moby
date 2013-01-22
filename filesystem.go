package docker

import (
	"errors"
	"fmt"
	"os"
	"syscall"
)

type Filesystem struct {
	RootFS string
	RWPath string
	Layers []string
}

func (fs *Filesystem) createMountPoints() error {
	if err := os.Mkdir(fs.RootFS, 0700); err != nil && !os.IsExist(err) {
		return err
	}
	if err := os.Mkdir(fs.RWPath, 0700); err != nil && !os.IsExist(err) {
		return err
	}
	return nil
}

func (fs *Filesystem) Mount() error {
	if fs.IsMounted() {
		return errors.New("Mount: Filesystem already mounted")
	}
	if err := fs.createMountPoints(); err != nil {
		return err
	}
	rwBranch := fmt.Sprintf("%v=rw", fs.RWPath)
	roBranches := ""
	for _, layer := range fs.Layers {
		roBranches += fmt.Sprintf("%v=ro:", layer)
	}
	branches := fmt.Sprintf("br:%v:%v", rwBranch, roBranches)
	return syscall.Mount("none", fs.RootFS, "aufs", 0, branches)
}

func (fs *Filesystem) Umount() error {
	if !fs.IsMounted() {
		return errors.New("Umount: Filesystem not mounted")
	}
	return syscall.Unmount(fs.RootFS, 0)
}

func (fs *Filesystem) IsMounted() bool {
	f, err := os.Open(fs.RootFS)
	if err != nil {
		if os.IsNotExist(err) {
			return false
		}
		panic(err)
	}
	list, err := f.Readdirnames(1)
	f.Close()
	if err != nil {
		return false
	}
	if len(list) > 0 {
		return true
	}
	return false
}

func newFilesystem(rootfs string, rwpath string, layers []string) *Filesystem {
	return &Filesystem{
		RootFS: rootfs,
		RWPath: rwpath,
		Layers: layers,
	}
}
