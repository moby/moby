// +build linux

package overlay2 // import "github.com/docker/docker/daemon/graphdriver/overlay2"

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"

	"github.com/pkg/errors"
	"golang.org/x/sys/unix"
)

// supportsMultipleLowerDir checks if the system supports multiple lowerdirs,
// which is required for the overlay2 driver. On 4.x kernels, multiple lowerdirs
// are always available (so this check isn't needed), and backported to RHEL and
// CentOS 3.x kernels (3.10.0-693.el7.x86_64 and up). This function is to detect
// support on those kernels, without doing a kernel version compare.
func supportsMultipleLowerDir(d string) error {
	td, err := ioutil.TempDir(d, "multiple-lowerdir-check")
	if err != nil {
		return err
	}
	defer func() {
		if err := os.RemoveAll(td); err != nil {
			logger.Warnf("Failed to remove check directory %v: %v", td, err)
		}
	}()

	for _, dir := range []string{"lower1", "lower2", "upper", "work", "merged"} {
		if err := os.Mkdir(filepath.Join(td, dir), 0755); err != nil {
			return err
		}
	}

	opts := fmt.Sprintf("lowerdir=%s:%s,upperdir=%s,workdir=%s", path.Join(td, "lower2"), path.Join(td, "lower1"), path.Join(td, "upper"), path.Join(td, "work"))
	if err := unix.Mount("overlay", filepath.Join(td, "merged"), "overlay", 0, opts); err != nil {
		return errors.Wrap(err, "failed to mount overlay")
	}
	if err := unix.Unmount(filepath.Join(td, "merged"), 0); err != nil {
		logger.Warnf("Failed to unmount check directory %v: %v", filepath.Join(td, "merged"), err)
	}
	return nil
}
