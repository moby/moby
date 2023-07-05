//go:build linux
// +build linux

package overlayutils // import "github.com/docker/docker/daemon/graphdriver/overlayutils"

import (
	"context"
	"fmt"
	"os"
	"path"
	"path/filepath"

	"github.com/containerd/containerd/log"
	"github.com/containerd/containerd/pkg/userns"
	"github.com/docker/docker/daemon/graphdriver"
	"github.com/opencontainers/selinux/go-selinux"
	"github.com/pkg/errors"
	"golang.org/x/sys/unix"
)

// ErrDTypeNotSupported denotes that the backing filesystem doesn't support d_type.
func ErrDTypeNotSupported(driver, backingFs string) error {
	msg := fmt.Sprintf("%s: the backing %s filesystem is formatted without d_type support, which leads to incorrect behavior.", driver, backingFs)
	if backingFs == "xfs" {
		msg += " Reformat the filesystem with ftype=1 to enable d_type support."
	}

	if backingFs == "extfs" {
		msg += " Reformat the filesystem (or use tune2fs) with -O filetype flag to enable d_type support."
	}

	msg += " Backing filesystems without d_type support are not supported."

	return graphdriver.NotSupportedError(msg)
}

const (
	testSELinuxLabel = "system_u:object_r:container_file_t:s0"
)

// SupportsOverlay checks if the system supports overlay filesystem
// by performing an actual overlay mount.
//
// checkMultipleLowers parameter enables check for multiple lowerdirs,
// which is required for the overlay2 driver.
func SupportsOverlay(d string, checkMultipleLowers bool) error {
	// We can't solely rely on selinux.GetEnabled() to detect whether SELinux is enabled, as [RootlessKit] does not
	// mount /sys/fs/selinux for us. _DOCKERD_ROOTLESS_SELINUX is set in the dockerd-rootless.sh wrapper script instead.
	//
	// [RootlessKit]: https://github.com/rootless-containers/rootlesskit/issues/94
	checkSELinux := false
	if selinux.GetEnabled() || os.Getenv("_DOCKERD_ROOTLESS_SELINUX") == "1" {
		// We only test for SELinux if the test label is valid; this prevents incorrectly testing for the container
		// SELinux profile, instead of kernel support.
		checkSELinux = selinux.SecurityCheckContext(testSELinuxLabel) == nil
	}

	td, err := os.MkdirTemp(d, "check-overlayfs-support")
	if err != nil {
		return err
	}
	defer func() {
		if err := os.RemoveAll(td); err != nil {
			log.G(context.TODO()).Warnf("Failed to remove check directory %v: %v", td, err)
		}
	}()

	for _, dir := range []string{"lower1", "lower2", "upper", "work", "merged"} {
		if err := os.Mkdir(filepath.Join(td, dir), 0o755); err != nil {
			return err
		}
	}

	mnt := filepath.Join(td, "merged")
	lowerDir := path.Join(td, "lower2")
	if checkMultipleLowers {
		lowerDir += ":" + path.Join(td, "lower1")
	}
	opts := fmt.Sprintf("lowerdir=%s,upperdir=%s,workdir=%s", lowerDir, path.Join(td, "upper"), path.Join(td, "work"))
	if checkSELinux {
		opts = fmt.Sprintf("%s,context=%s", opts, testSELinuxLabel)
	}
	if err := unix.Mount("overlay", mnt, "overlay", 0, opts); err != nil {
		return errors.Wrap(err, "failed to mount overlay")
	}
	if err := unix.Unmount(mnt, 0); err != nil {
		log.G(context.TODO()).Warnf("Failed to unmount check directory %v: %v", mnt, err)
	}
	return nil
}

// GetOverlayXattr combines the overlay module's xattr class with the named
// xattr -- `user` when mounted inside a user namespace, and `trusted` when
// mounted in the 'root' namespace.
func GetOverlayXattr(name string) string {
	class := "trusted"
	if userns.RunningInUserNS() {
		class = "user"
	}
	return fmt.Sprintf("%s.overlay.%s", class, name)
}
