// +build linux

package mount

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"syscall"
)

func PivotRoot(rootfs, pivotBaseDir string) error {
	if pivotBaseDir == "" {
		pivotBaseDir = "/"
	}
	tmpDir := filepath.Join(rootfs, pivotBaseDir)
	if err := os.MkdirAll(tmpDir, 0755); err != nil {
		return fmt.Errorf("can't create tmp dir %s, error %v", tmpDir, err)
	}
	pivotDir, err := ioutil.TempDir(tmpDir, ".pivot_root")
	if err != nil {
		return fmt.Errorf("can't create pivot_root dir %s, error %v", pivotDir, err)
	}

	if err := syscall.PivotRoot(rootfs, pivotDir); err != nil {
		return fmt.Errorf("pivot_root %s", err)
	}

	if err := syscall.Chdir("/"); err != nil {
		return fmt.Errorf("chdir / %s", err)
	}

	// path to pivot dir now changed, update
	pivotDir = filepath.Join(pivotBaseDir, filepath.Base(pivotDir))
	if err := syscall.Unmount(pivotDir, syscall.MNT_DETACH); err != nil {
		return fmt.Errorf("unmount pivot_root dir %s", err)
	}

	return os.Remove(pivotDir)
}
