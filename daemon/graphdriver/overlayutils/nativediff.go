//go:build linux
// +build linux

package overlayutils

import (
	"os"
	"path/filepath"
	"syscall"

	"github.com/containerd/containerd/pkg/userns"
	"github.com/docker/docker/pkg/system"
	"github.com/pkg/errors"
)

// SupportsNativeDiff checks whether the filesystem has a bug
// which copies up the opaque flag when copying up an opaque
// directory or the kernel enable CONFIG_OVERLAY_FS_REDIRECT_DIR.
// When these exist naive diff should be used.
//
// When running in a user namespace, returns errRunningInUserNS
// immediately.
func SupportsNativeDiff(ctx *Context, d string) error {
	if userns.RunningInUserNS() {
		return errors.New("running in a user namespace")
	}

	td, err := os.MkdirTemp(d, "opaque-bug-check-")
	if err != nil {
		return err
	}
	defer func() {
		if err := os.RemoveAll(td); err != nil {
			ctx.logger.WithError(err).Warnf("failed to remove check directory %v", td)
		}
	}()

	tm, err := makeTestMount(td, 2)
	if err != nil {
		return err
	}

	// Make directories lower1/d, lower1/d1, lower2/d
	for _, dir := range []string{filepath.Join(tm.lowerDirs[0], "d"), filepath.Join(tm.lowerDirs[0], "d1"), filepath.Join(tm.lowerDirs[1], "d")} {
		if err := os.Mkdir(filepath.Join(d, dir), 0755); err != nil {
			return err
		}
	}

	// Mark lower2/d as opaque
	if err := system.Lsetxattr(filepath.Join(tm.lowerDirs[1], "d"), "trusted.overlay.opaque", []byte("y"), 0); err != nil {
		return errors.Wrap(err, "failed to set opaque flag on middle layer")
	}

	if err := tm.mount(nil); err != nil {
		return errors.Wrap(err, "failed to mount overlay")
	}
	defer func() {
		if err := tm.unmount(); err != nil {
			ctx.logger.WithError(err).Warnf("failed to unmount check directory %v: %v", tm.mergedDir)
		}
	}()

	// Touch file in d to force copy up of opaque directory "d" from "lower2" to "upper"
	if err := os.WriteFile(filepath.Join(tm.mergedDir, "d", "f"), []byte{}, 0644); err != nil {
		return errors.Wrap(err, "failed to write to merged directory")
	}

	// Check upper/d does not have opaque flag
	xattrOpaque, err := system.Lgetxattr(filepath.Join(tm.upperDir, "d"), "trusted.overlay.opaque")
	if err != nil {
		return errors.Wrap(err, "failed to read opaque flag on upper layer")
	}
	if string(xattrOpaque) == "y" {
		return errors.New("opaque flag erroneously copied up, consider update to kernel 4.8 or later to fix")
	}

	// rename "d1" to "d2"
	if err := os.Rename(filepath.Join(tm.mergedDir, "d1"), filepath.Join(tm.mergedDir, "d2")); err != nil {
		// if rename failed with syscall.EXDEV, the kernel doesn't have CONFIG_OVERLAY_FS_REDIRECT_DIR enabled
		if err.(*os.LinkError).Err == syscall.EXDEV {
			return nil
		}
		return errors.Wrap(err, "failed to rename dir in merged directory")
	}
	// get the xattr of "d2"
	xattrRedirect, err := system.Lgetxattr(filepath.Join(tm.upperDir, "d2"), "trusted.overlay.redirect")
	if err != nil {
		return errors.Wrap(err, "failed to read redirect flag on upper layer")
	}

	if string(xattrRedirect) == "d1" {
		return errors.New("kernel has CONFIG_OVERLAY_FS_REDIRECT_DIR enabled")
	}

	return nil
}
