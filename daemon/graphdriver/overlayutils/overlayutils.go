//go:build linux
// +build linux

package overlayutils // import "github.com/docker/docker/daemon/graphdriver/overlayutils"

import (
	"fmt"
	"os"

	"github.com/docker/docker/daemon/graphdriver"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

type Context struct {
	driver string
	logger *logrus.Entry
}

func NewContext(driver string, logger *logrus.Entry) *Context {
	return &Context{
		driver: driver,
		logger: logger,
	}
}

// SupportsOverlay checks if the system supports overlay filesystem
// by performing an actual overlay mount.
//
// checkMultipleLowers parameter enables check for multiple lowerdirs,
// which is required for the overlay2 driver.
func SupportsOverlay(ctx *Context, d string, checkMultipleLowers bool) error {
	// We can't rely on go-selinux.GetEnabled() to detect whether SELinux is enabled,
	// because RootlessKit doesn't mount /sys/fs/selinux in the child: https://github.com/rootless-containers/rootlesskit/issues/94
	// So we check $_DOCKERD_ROOTLESS_SELINUX, which is set by dockerd-rootless.sh .
	if os.Getenv("_DOCKERD_ROOTLESS_SELINUX") == "1" {
		// Kernel 5.11 introduced support for rootless overlayfs, but incompatible with SELinux,
		// so fallback to fuse-overlayfs.
		// https://github.com/moby/moby/issues/42333
		return fmt.Errorf("%s: driver is not supported for Rootless with SELinux", ctx.driver)
	}

	td, err := os.MkdirTemp(d, "overlay-check-")
	if err != nil {
		return err
	}
	defer func() {
		if err := os.RemoveAll(td); err != nil {
			ctx.logger.WithError(err).Warnf("failed to remove check directory %v", td)
		}
	}()

	lowerCount := 1
	if checkMultipleLowers {
		lowerCount = 2
	}

	tm, err := makeTestMount(td, lowerCount)
	if err != nil {
		return err
	}

	if err := tm.mount(nil); err != nil {
		return errors.Wrap(err, "failed to mount overlay")
	}

	if err := tm.unmount(); err != nil {
		ctx.logger.WithError(err).Warnf("failed to unmount check directory %v: %v", tm.mergedDir)
	}
	return nil
}

// ErrDTypeNotSupported denotes that the backing filesystem doesn't support d_type.
func ErrDTypeNotSupported(ctx *Context, backingFs string) error {
	msg := fmt.Sprintf("%s: the backing %s filesystem is formatted without d_type support, which leads to incorrect behavior.", ctx.driver, backingFs)
	if backingFs == "xfs" {
		msg += " Reformat the filesystem with ftype=1 to enable d_type support."
	}

	if backingFs == "extfs" {
		msg += " Reformat the filesystem (or use tune2fs) with -O filetype flag to enable d_type support."
	}

	msg += " Backing filesystems without d_type support are not supported."

	return graphdriver.NotSupportedError(msg)
}
