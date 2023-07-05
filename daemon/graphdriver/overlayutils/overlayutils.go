//go:build linux
// +build linux

package overlayutils // import "github.com/docker/docker/daemon/graphdriver/overlayutils"

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/containerd/containerd/log"
	"github.com/containerd/containerd/mount"
	"github.com/containerd/containerd/pkg/userns"
	"github.com/docker/docker/daemon/graphdriver"
	"github.com/opencontainers/selinux/go-selinux"
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

// SupportsOverlay determines whether the kernel supports overlayfs (meeting our needs) by performing an actual mount.
// Before the mount, we also check for basic vfs functionality that overlay depends on, but may not check before
// creating a mount, e.g. the RENAME_EXCHANGE, RENAME_NOREPLACE, RENAME_WHITEOUT, and flags in renameat2(2).
//
// checkMultipleLowers tests for multiple lowerdir support, which is needed for overlay2.
//
// [renameat2(2)]: https://linux.die.net/man/2/renameat
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
			log.G(context.TODO()).WithError(err).Warnf("failed to remove check directory %v", td)
		}
	}()

	l1, l2, l3, work, merged := filepath.Join(td, "l1"), filepath.Join(td, "l2"), filepath.Join(td, "l3"), filepath.Join(td, "work"), filepath.Join(td, "merged")
	for _, dir := range []string{l1, l2, l3, work, merged} {
		if err := os.Mkdir(dir, 0o755); err != nil {
			return err
		}
	}

	// Create files, each holding their own path, which will be used for subsequent testing.
	f1, f2 := filepath.Join(l1, "f1"), filepath.Join(l1, "f2")
	for _, file := range []string{f1, f2} {
		if err := os.WriteFile(file, []byte(file), 0o644); err != nil {
			return err
		}
	}

	// RENAME_EXCHANGE will cause an atomic swap of f1 and f2. Create two files, each with their own paths as their
	// contents.
	err = unix.Renameat2(0, f1, 0, f2, unix.RENAME_EXCHANGE)
	if err != nil {
		return err
	}
	// The contents of f1 should now be the path to f2, and vice versa.
	content, err := os.ReadFile(f1)
	if !bytes.Equal(content, []byte(f2)) || err != nil {
		return fmt.Errorf("RENAME_EXCHANGE of %v and %v failed: %w", f1, f2, err)
	}

	// RENAME_NOREPLACE should cause an error since f2 already exists.
	err = unix.Renameat2(0, f1, 0, f2, unix.RENAME_NOREPLACE)
	if !errors.Is(err, unix.EEXIST) {
		return fmt.Errorf("RENAME_NOREPLACE of %v to %v should have failed with EEXIST, but got %w", f1, f2, err)
	}

	// RENAME_WHITEOUT will move f3 to f4, leaving a whiteout file behind. This is used internally by overlayfs, but is
	// not validated before a mount is attempted, as some use cases may not break. Ours will.
	f3, f4 := filepath.Join(l1, "f3"), filepath.Join(l1, "f4")
	if err := os.WriteFile(f3, []byte{}, 0o700); err != nil {
		return err
	}
	err = unix.Renameat2(0, f3, 0, f4, unix.RENAME_WHITEOUT)
	if err != nil {
		return fmt.Errorf("RENAME_WHITEOUT of %v to %v failed: %w", f3, f4, err)
	}

	lowers := l1
	if checkMultipleLowers {
		lowers += ":" + l2
	}
	opts := []string{fmt.Sprintf("lowerdir=%s,upperdir=%s,workdir=%s", lowers, l3, work)}
	if checkSELinux {
		opts = append(opts, fmt.Sprintf("context=%s", testSELinuxLabel))
	}

	m := mount.Mount{
		Type:    "overlay",
		Source:  "overlay",
		Options: opts,
	}

	if err := m.Mount(merged); err != nil {
		return fmt.Errorf("failed to mount overlayfs (checkMultipleLowers=%t,checkSELinux=%t): %w", checkMultipleLowers, checkSELinux, err)
	}
	if err := mount.UnmountAll(merged, 0); err != nil {
		log.G(context.TODO()).WithError(err).Warnf("failed to unmount check directory %v", merged)
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
