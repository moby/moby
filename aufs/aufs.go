package aufs

import (
	"fmt"
	"github.com/dotcloud/docker/archive"
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
	root string
}

// New returns a new AUFS driver.
// An error is returned if AUFS is not supported.
func Init(root string) (graphdriver.Driver, error) {
	// Try to load the aufs kernel module
	if err := exec.Command("modprobe", "aufs").Run(); err != nil {
		return nil, err
	}
	return &AufsDriver{root}, nil
}

func (a *AufsDriver) OnCreate(dir graphdriver.Dir, layer archive.Archive) error {
	tmp := path.Join(os.TempDir(), dir.ID())
	if err := os.MkdirAll(tmp, 0755); err != nil {
		return err
	}
	defer os.RemoveAll(tmp)

	layerRoot := path.Join(a.root, dir.ID())
	if err := os.MkdirAll(layerRoot, 0755); err != nil {
		return err
	}

	if layer != nil {
		if err := archive.Untar(layer, tmp); err != nil {
			return err
		}
	}

	if err := os.Rename(tmp, layerRoot); err != nil {
		return err
	}
	return nil
}

func (a *AufsDriver) OnRemove(dir graphdriver.Dir) error {
	tmp := path.Join(os.TempDir(), dir.ID())

	if err := os.MkdirAll(tmp, 0755); err != nil {
		return err
	}

	if err := os.Rename(path.Join(a.root, dir.ID()), tmp); err != nil {
		return err
	}
	return os.RemoveAll(tmp)
}

func (a *AufsDriver) OnMount(dir graphdriver.Dir, dest string) error {
	layers, err := a.getLayers(dir)
	if err != nil {
		return err
	}

	target := path.Join(dest, "rootfs")
	rw := path.Join(dest, "rw")

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

func (a *AufsDriver) OnUnmount(dest string) error {
	target := path.Join(dest, "rootfs")
	if _, err := os.Stat(target); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	return Unmount(target)
}

func (a *AufsDriver) Mounted(dest string) (bool, error) {
	return Mounted(path.Join(dest, "rootfs"))
}

func (a *AufsDriver) Layer(dir graphdriver.Dir, dest string) (archive.Archive, error) {
	return nil, fmt.Errorf("not implemented")
}

func (a *AufsDriver) Cleanup() error {
	return nil
}

func (a *AufsDriver) getLayers(dir graphdriver.Dir) ([]string, error) {
	var (
		err     error
		layers  = []string{}
		current = dir
	)

	for current != nil {
		layers = append(layers, current.Path())
		if current, err = current.Parent(); err != nil {
			return nil, err
		}
	}
	return layers, nil
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
