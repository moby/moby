//go:build linux
// +build linux

package overlay2 // import "github.com/docker/docker/daemon/graphdriver/overlay2"

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"syscall"

	"github.com/containerd/containerd/pkg/userns"
	"github.com/docker/docker/pkg/system"
	"github.com/pkg/errors"
	"golang.org/x/sys/unix"
)

// doesSupportNativeDiff checks whether the filesystem has a bug
// which copies up the opaque flag when copying up an opaque
// directory or the kernel enable CONFIG_OVERLAY_FS_REDIRECT_DIR.
// When these exist naive diff should be used.
//
// When running in a user namespace, returns errRunningInUserNS
// immediately.
func doesSupportNativeDiff(d string) error {
	if userns.RunningInUserNS() {
		return errors.New("running in a user namespace")
	}

	td, err := os.MkdirTemp(d, "opaque-bug-check")
	if err != nil {
		return err
	}
	defer func() {
		if err := os.RemoveAll(td); err != nil {
			logger.Warnf("Failed to remove check directory %v: %v", td, err)
		}
	}()

	// Make directories l1/d, l1/d1, l2/d, l3, work, merged
	if err := os.MkdirAll(filepath.Join(td, "l1", "d"), 0755); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Join(td, "l1", "d1"), 0755); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Join(td, "l2", "d"), 0755); err != nil {
		return err
	}
	if err := os.Mkdir(filepath.Join(td, "l3"), 0755); err != nil {
		return err
	}
	if err := os.Mkdir(filepath.Join(td, workDirName), 0755); err != nil {
		return err
	}
	if err := os.Mkdir(filepath.Join(td, mergedDirName), 0755); err != nil {
		return err
	}

	// Mark l2/d as opaque
	if err := system.Lsetxattr(filepath.Join(td, "l2", "d"), "trusted.overlay.opaque", []byte("y"), 0); err != nil {
		return errors.Wrap(err, "failed to set opaque flag on middle layer")
	}

	opts := fmt.Sprintf("lowerdir=%s:%s,upperdir=%s,workdir=%s", path.Join(td, "l2"), path.Join(td, "l1"), path.Join(td, "l3"), path.Join(td, workDirName))
	if err := unix.Mount("overlay", filepath.Join(td, mergedDirName), "overlay", 0, opts); err != nil {
		return errors.Wrap(err, "failed to mount overlay")
	}
	defer func() {
		if err := unix.Unmount(filepath.Join(td, mergedDirName), 0); err != nil {
			logger.Warnf("Failed to unmount check directory %v: %v", filepath.Join(td, mergedDirName), err)
		}
	}()

	// Touch file in d to force copy up of opaque directory "d" from "l2" to "l3"
	if err := os.WriteFile(filepath.Join(td, mergedDirName, "d", "f"), []byte{}, 0644); err != nil {
		return errors.Wrap(err, "failed to write to merged directory")
	}

	// Check l3/d does not have opaque flag
	xattrOpaque, err := system.Lgetxattr(filepath.Join(td, "l3", "d"), "trusted.overlay.opaque")
	if err != nil {
		return errors.Wrap(err, "failed to read opaque flag on upper layer")
	}
	if string(xattrOpaque) == "y" {
		return errors.New("opaque flag erroneously copied up, consider update to kernel 4.8 or later to fix")
	}

	// rename "d1" to "d2"
	if err := os.Rename(filepath.Join(td, mergedDirName, "d1"), filepath.Join(td, mergedDirName, "d2")); err != nil {
		// if rename failed with syscall.EXDEV, the kernel doesn't have CONFIG_OVERLAY_FS_REDIRECT_DIR enabled
		if err.(*os.LinkError).Err == syscall.EXDEV {
			return nil
		}
		return errors.Wrap(err, "failed to rename dir in merged directory")
	}
	// get the xattr of "d2"
	xattrRedirect, err := system.Lgetxattr(filepath.Join(td, "l3", "d2"), "trusted.overlay.redirect")
	if err != nil {
		return errors.Wrap(err, "failed to read redirect flag on upper layer")
	}

	if string(xattrRedirect) == "d1" {
		return errors.New("kernel has CONFIG_OVERLAY_FS_REDIRECT_DIR enabled")
	}

	return nil
}
