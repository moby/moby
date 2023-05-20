//go:build linux

package overlay2 // import "github.com/docker/docker/daemon/graphdriver/overlay2"

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"syscall"

	"github.com/containerd/containerd/mount"
	"github.com/containerd/containerd/pkg/userns"
	"github.com/docker/docker/daemon/graphdriver/overlayutils"
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

// Forked from https://github.com/containers/storage/blob/05c69f1b2a5871d170c07dc8d2eec69c681e143b/drivers/overlay/check.go
//
// usingMetacopy checks if overlayfs's metacopy feature is active. When active,
// overlayfs will only copy up metadata (as opposed to the whole file) when a
// metadata-only operation is performed. Affected inodes will be marked with
// the "(trusted|user).overlay.metacopy" xattr.
//
// The CONFIG_OVERLAY_FS_METACOPY option, the overlay.metacopy parameter, or
// the metacopy mount option can all enable metacopy mode. For more details on
// this feature, see filesystems/overlayfs.txt in the kernel documentation
// tree.
//
// Note that the mount option should never be relevant should never come up the
// daemon has control over all of its own mounts and presently does not request
// metacopy. Nonetheless, a user or kernel distributor may enable metacopy, so
// we should report in the daemon whether or not we detect its use.
func usingMetacopy(d string) (bool, error) {
	userxattr := false
	if userns.RunningInUserNS() {
		needed, err := overlayutils.NeedsUserXAttr(d)
		if err != nil {
			return false, err
		}
		if needed {
			userxattr = true
		}
	}

	td, err := os.MkdirTemp(d, "metacopy-check")
	if err != nil {
		return false, err
	}
	defer func() {
		if err := os.RemoveAll(td); err != nil {
			logger.WithError(err).Warnf("failed to remove check directory %v", td)
		}
	}()

	l1, l2, work, merged := filepath.Join(td, "l1"), filepath.Join(td, "l2"), filepath.Join(td, "work"), filepath.Join(td, "merged")
	for _, dir := range []string{l1, l2, work, merged} {
		if err := os.Mkdir(dir, 0755); err != nil {
			return false, err
		}
	}

	// Create empty file in l1 with 0700 permissions for metacopy test
	if err := os.WriteFile(filepath.Join(l1, "f"), []byte{}, 0700); err != nil {
		return false, err
	}

	opts := []string{fmt.Sprintf("lowerdir=%s,upperdir=%s,workdir=%s", l1, l2, work)}
	if userxattr {
		opts = append(opts, "userxattr")
	}

	m := mount.Mount{
		Type:    "overlay",
		Source:  "overlay",
		Options: opts,
	}

	if err := m.Mount(merged); err != nil {
		return false, errors.Wrap(err, "failed to mount overlay for metacopy check")
	}
	defer func() {
		if err := mount.UnmountAll(merged, 0); err != nil {
			logger.WithError(err).Warnf("failed to unmount check directory %v", merged)
		}
	}()

	// Make a change that only impacts the inode, in the upperdir
	if err := os.Chmod(filepath.Join(merged, "f"), 0600); err != nil {
		return false, errors.Wrap(err, "error changing permissions on file for metacopy check")
	}

	// ...and check if the pulled-up copy is marked as metadata-only
	xattr, err := system.Lgetxattr(filepath.Join(l2, "f"), overlayutils.GetOverlayXattr("metacopy"))
	if err != nil {
		// ENOTSUP signifies the FS does not support either xattrs or metacopy. In either case,
		// it is not a fatal error, and we should report metacopy as unused.
		if errors.Is(err, unix.ENOTSUP) {
			return false, nil
		}
		return false, errors.Wrap(err, "metacopy flag was not set on file in the upperdir")
	}
	usingMetacopy := xattr != nil

	logger.WithField("usingMetacopy", usingMetacopy).Debug("successfully detected metacopy status")
	return usingMetacopy, nil
}
