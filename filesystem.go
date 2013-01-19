package docker

import (
	"fmt"
	"os"
	"os/exec"
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
	if err := fs.createMountPoints(); err != nil {
		return err
	}
	rwBranch := fmt.Sprintf("%v=rw", fs.RWPath)
	roBranches := ""
	for _, layer := range fs.Layers {
		roBranches += fmt.Sprintf("%v=ro:", layer)
	}
	branches := fmt.Sprintf("br:%v:%v", rwBranch, roBranches)
	cmd := exec.Command("mount", "-t", "aufs", "-o", branches, "none", fs.RootFS)
	if err := cmd.Run(); err != nil {
		return err
	}
	return nil
}

func (fs *Filesystem) Umount() error {
	return exec.Command("umount", fs.RootFS).Run()
}

func newFilesystem(rootfs string, rwpath string, layers []string) *Filesystem {
	return &Filesystem{
		RootFS: rootfs,
		RWPath: rwpath,
		Layers: layers,
	}
}
