package aufs

import (
	"fmt"
	"github.com/dotcloud/docker/graphdriver"
	"log"
	"os"
	"os/exec"
	"path"
)

func init() {
	graphdriver.Register("aufs", Init)
}

type AufsDriver struct {
}

// New returns a new AUFS driver.
// An error is returned if AUFS is not supported.
func Init(root string) (graphdriver.Driver, error) {
	// Try to load the aufs kernel module
	if err := exec.Command("modprobe", "aufs").Run(); err != nil {
		return nil, err
	}
	return &AufsDriver{}, nil
}

func (a *AufsDriver) Mount(img graphdriver.Image, root string) error {
	layers, err := img.Layers()
	if err != nil {
		return err
	}

	target := path.Join(root, "rootfs")
	rw := path.Join(root, "rw")

	// Create the target directories if they don't exist
	if err := os.Mkdir(target, 0755); err != nil && !os.IsExist(err) {
		return err
	}
	if err := os.Mkdir(rw, 0755); err != nil && !os.IsExist(err) {
		return err
	}
	if err := a.aufsMount(layers, rw, target); err != nil {
		return err
	}
	return nil
}

func (a *AufsDriver) Unmount(root string) error {
	target := path.Join(root, "rootfs")
	if _, err := os.Stat(target); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	return Unmount(target)
}

func (a *AufsDriver) Mounted(root string) (bool, error) {
	return Mounted(path.Join(root, "rootfs"))
}

func (a *AufsDriver) aufsMount(ro []string, rw, target string) error {
	rwBranch := fmt.Sprintf("%v=rw", rw)
	roBranches := ""
	for _, layer := range ro {
		roBranches += fmt.Sprintf("%v=ro+wh:", layer)
	}
	branches := fmt.Sprintf("br:%v:%v,xino=/dev/shm/aufs.xino", rwBranch, roBranches)

	//if error, try to load aufs kernel module
	if err := mount("none", target, "aufs", 0, branches); err != nil {
		log.Printf("Kernel does not support AUFS, trying to load the AUFS module with modprobe...")
		if err := exec.Command("modprobe", "aufs").Run(); err != nil {
			return fmt.Errorf("Unable to load the AUFS module")
		}
		log.Printf("...module loaded.")
		if err := mount("none", target, "aufs", 0, branches); err != nil {
			return fmt.Errorf("Unable to mount using aufs")
		}
	}
	return nil
}
