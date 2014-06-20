// +build linux

package mount

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"syscall"

	"github.com/dotcloud/docker/pkg/system"
)

func PivotRoot(rootfs string) error {
	pivotDir, err := ioutil.TempDir(rootfs, ".pivot_root")
	if err != nil {
		return fmt.Errorf("can't create pivot_root dir %s, error %v", pivotDir, err)
	}
	if err := system.Pivotroot(rootfs, pivotDir); err != nil {
		return fmt.Errorf("pivot_root %s", err)
	}
	if err := system.Chdir("/"); err != nil {
		return fmt.Errorf("chdir / %s", err)
	}
	// path to pivot dir now changed, update
	pivotDir = filepath.Join("/", filepath.Base(pivotDir))
	if err := system.Unmount(pivotDir, syscall.MNT_DETACH); err != nil {
		return fmt.Errorf("unmount pivot_root dir %s", err)
	}
	return os.Remove(pivotDir)
}
