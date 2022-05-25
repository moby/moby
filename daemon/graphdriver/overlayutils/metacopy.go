//go:build linux
// +build linux

// Forked from https://github.com/containers/storage/blob/05c69f1b2a5871d170c07dc8d2eec69c681e143b/drivers/overlay/check.go

package overlayutils

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/containerd/containerd/pkg/userns"
	"github.com/docker/docker/pkg/system"
	"github.com/pkg/errors"
)

// IsUsingMetacopy checks if overlayfs's metacopy feature is active. When
// active, overlayfs will only copy up metadata (as opposed to the whole file)
// when a metadata-only operation is performed. Affected inodes will be marked
// with the "(trusted|user).overlay.metacopy" xattr.
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
func IsUsingMetacopy(ctx *Context, d string, userxattr bool) (bool, error) {
	td, err := os.MkdirTemp(d, "metacopy-check-")
	if err != nil {
		return false, err
	}
	defer func() {
		if err := os.RemoveAll(td); err != nil {
			ctx.logger.WithError(err).Warnf("failed to remove check directory %v", td)
		}
	}()

	tm, err := makeTestMount(td, 1)
	if err != nil {
		return false, err
	}

	// Create empty file in lower1 with 0700 permissions for metacopy test
	if err := os.WriteFile(filepath.Join(tm.lowerDirs[0], "f"), []byte{}, 0700); err != nil {
		return false, err
	}

	var opts []string
	if userxattr {
		opts = append(opts, "userxattr")
	}

	if err := tm.mount(opts); err != nil {
		return false, errors.Wrap(err, "failed to mount overlay for metacopy check")
	}
	defer func() {
		if err := tm.unmount(); err != nil {
			ctx.logger.WithError(err).Warnf("failed to unmount check directory %v", tm.mergedDir)
		}
	}()

	// Make a change that only impacts the inode, in the upperdir
	if err := os.Chmod(filepath.Join(tm.mergedDir, "f"), 0600); err != nil {
		return false, errors.Wrap(err, "error changing permissions on file for metacopy check")
	}

	// ...and check if the pulled-up copy is marked as metadata-only
	xattr, err := system.Lgetxattr(filepath.Join(tm.upperDir, "f"), getOverlayXattr("metacopy"))
	if err != nil {
		return false, errors.Wrap(err, "metacopy flag was not set on file in the upperdir")
	}
	usingMetacopy := xattr != nil

	ctx.logger.WithField("usingMetacopy", usingMetacopy).Debug("successfully detected metacopy status")
	return usingMetacopy, nil
}

// getOverlayXattr combines the overlay module's xattr class with the named
// xattr -- `user` when mounted inside a user namespace, and `trusted` when
// mounted in the 'root' namespace.
func getOverlayXattr(name string) string {
	class := "trusted"
	if userns.RunningInUserNS() {
		class = "user"
	}
	return fmt.Sprintf("%s.overlay.%s", class, name)
}
