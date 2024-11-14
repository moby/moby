//go:build linux

/*
   Copyright The containerd Authors.

   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
*/

package overlayutils

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"

	"github.com/moby/sys/userns"
	"golang.org/x/sys/unix"

	"github.com/containerd/containerd/v2/core/mount"
	kernel "github.com/containerd/containerd/v2/pkg/kernelversion"
	"github.com/containerd/continuity/fs"
	"github.com/containerd/log"
)

const (
	// see https://man7.org/linux/man-pages/man2/statfs.2.html
	tmpfsMagic = 0x01021994
)

// SupportsMultipleLowerDir checks if the system supports multiple lowerdirs,
// which is required for the overlay snapshotter. On 4.x kernels, multiple lowerdirs
// are always available (so this check isn't needed), and backported to RHEL and
// CentOS 3.x kernels (3.10.0-693.el7.x86_64 and up). This function is to detect
// support on those kernels, without doing a kernel version compare.
//
// Ported from moby overlay2.
func SupportsMultipleLowerDir(d string) error {
	td, err := os.MkdirTemp(d, "multiple-lowerdir-check")
	if err != nil {
		return err
	}
	defer func() {
		if err := os.RemoveAll(td); err != nil {
			log.L.WithError(err).Warnf("Failed to remove check directory %v", td)
		}
	}()

	for _, dir := range []string{"lower1", "lower2", "upper", "work", "merged"} {
		if err := os.Mkdir(filepath.Join(td, dir), 0755); err != nil {
			return err
		}
	}

	opts := fmt.Sprintf("lowerdir=%s:%s,upperdir=%s,workdir=%s", filepath.Join(td, "lower2"), filepath.Join(td, "lower1"), filepath.Join(td, "upper"), filepath.Join(td, "work"))
	m := mount.Mount{
		Type:    "overlay",
		Source:  "overlay",
		Options: []string{opts},
	}
	dest := filepath.Join(td, "merged")
	if err := m.Mount(dest); err != nil {
		return fmt.Errorf("failed to mount overlay: %w", err)
	}
	if err := mount.UnmountAll(dest, 0); err != nil {
		log.L.WithError(err).Warnf("Failed to unmount check directory %v", dest)
	}
	return nil
}

// Supported returns nil when the overlayfs is functional on the system with the root directory.
// Supported is not called during plugin initialization, but exposed for downstream projects which uses
// this snapshotter as a library.
func Supported(root string) error {
	if err := os.MkdirAll(root, 0700); err != nil {
		return err
	}
	supportsDType, err := fs.SupportsDType(root)
	if err != nil {
		return err
	}
	if !supportsDType {
		return fmt.Errorf("%s does not support d_type. If the backing filesystem is xfs, please reformat with ftype=1 to enable d_type support", root)
	}
	return SupportsMultipleLowerDir(root)
}

// IsPathOnTmpfs returns whether the path is on a tmpfs or not.
//
// It uses statfs to check if the fs type is TMPFS_MAGIC (0x01021994)
// see https://man7.org/linux/man-pages/man2/statfs.2.html
func IsPathOnTmpfs(d string) bool {
	stat := syscall.Statfs_t{}
	err := syscall.Statfs(d, &stat)
	if err != nil {
		log.L.WithError(err).Warnf("Could not retrieve statfs for %v", d)
		return false
	}

	return stat.Type == tmpfsMagic
}

// NeedsUserXAttr returns whether overlayfs should be mounted with the "userxattr" mount option.
//
// The "userxattr" option is needed for mounting overlayfs inside a user namespace with kernel >= 5.11.
//
// The "userxattr" option is NOT needed for the initial user namespace (aka "the host").
//
// Also, Ubuntu (since circa 2015) and Debian (since 10) with kernel < 5.11 can mount
// the overlayfs in a user namespace without the "userxattr" option.
//
// The corresponding kernel commit: https://github.com/torvalds/linux/commit/2d2f2d7322ff43e0fe92bf8cccdc0b09449bf2e1
// > ovl: user xattr
// >
// > Optionally allow using "user.overlay." namespace instead of "trusted.overlay."
// > ...
// > Disable redirect_dir and metacopy options, because these would allow privilege escalation through direct manipulation of the
// > "user.overlay.redirect" or "user.overlay.metacopy" xattrs.
// > ...
//
// The "userxattr" support is not exposed in "/sys/module/overlay/parameters".
func NeedsUserXAttr(d string) (bool, error) {
	if !userns.RunningInUserNS() {
		// we are the real root (i.e., the root in the initial user NS),
		// so we do never need "userxattr" opt.
		return false, nil
	}

	// userxattr not permitted on tmpfs https://man7.org/linux/man-pages/man5/tmpfs.5.html
	if IsPathOnTmpfs(d) {
		return false, nil
	}

	// Fast path on kernels >= 5.11
	//
	// Keep in mind that distro vendors might be going to backport the patch to older kernels
	// so we can't completely remove the "slow path".
	fiveDotEleven := kernel.KernelVersion{Kernel: 5, Major: 11}
	if ok, err := kernel.GreaterEqualThan(fiveDotEleven); err == nil && ok {
		return true, nil
	}

	tdRoot := filepath.Join(d, "userxattr-check")
	if err := os.RemoveAll(tdRoot); err != nil {
		log.L.WithError(err).Warnf("Failed to remove check directory %v", tdRoot)
	}

	if err := os.MkdirAll(tdRoot, 0700); err != nil {
		return false, err
	}

	defer func() {
		if err := os.RemoveAll(tdRoot); err != nil {
			log.L.WithError(err).Warnf("Failed to remove check directory %v", tdRoot)
		}
	}()

	td, err := os.MkdirTemp(tdRoot, "")
	if err != nil {
		return false, err
	}

	for _, dir := range []string{"lower1", "lower2", "upper", "work", "merged"} {
		if err := os.Mkdir(filepath.Join(td, dir), 0755); err != nil {
			return false, err
		}
	}

	opts := []string{
		"ro",
		fmt.Sprintf("lowerdir=%s:%s,upperdir=%s,workdir=%s", filepath.Join(td, "lower2"), filepath.Join(td, "lower1"), filepath.Join(td, "upper"), filepath.Join(td, "work")),
		"userxattr",
	}

	m := mount.Mount{
		Type:    "overlay",
		Source:  "overlay",
		Options: opts,
	}

	dest := filepath.Join(td, "merged")
	if err := m.Mount(dest); err != nil {
		// Probably the host is running Ubuntu/Debian kernel (< 5.11) with the userns patch but without the userxattr patch.
		// Return false without error.
		log.L.WithError(err).Debugf("cannot mount overlay with \"userxattr\", probably the kernel does not support userxattr")
		return false, nil
	}
	if err := mount.UnmountAll(dest, 0); err != nil {
		log.L.WithError(err).Warnf("Failed to unmount check directory %v", dest)
	}
	return true, nil
}

// SupportsIDMappedMounts tells if this kernel supports idmapped mounts for overlayfs
// or not.
//
// This function returns error whether the kernel supports idmapped mounts
// for overlayfs or not, i.e. if e.g. -ENOSYS may be returned as well as -EPERM.
// So, caller should check for (true, err == nil), otherwise treat it as there's
// no support from the kernel side.
func SupportsIDMappedMounts() (bool, error) {
	// Fast path
	fiveDotNineteen := kernel.KernelVersion{Kernel: 5, Major: 19}
	if ok, err := kernel.GreaterEqualThan(fiveDotNineteen); err == nil && ok {
		return true, nil
	}

	// Do slow path, because idmapped mounts may be backported to older kernels.
	uidMap := syscall.SysProcIDMap{
		ContainerID: 0,
		HostID:      666,
		Size:        1,
	}
	gidMap := syscall.SysProcIDMap{
		ContainerID: 0,
		HostID:      666,
		Size:        1,
	}
	td, err := os.MkdirTemp("", "ovl-idmapped-check")
	if err != nil {
		return false, fmt.Errorf("failed to create check directory: %w", err)
	}
	defer func() {
		if err := os.RemoveAll(td); err != nil {
			log.L.WithError(err).Warnf("failed to remove check directory %s", td)
		}
	}()

	for _, dir := range []string{"lower", "upper", "work", "merged"} {
		if err = os.Mkdir(filepath.Join(td, dir), 0755); err != nil {
			return false, fmt.Errorf("failed to create %s directory: %w", dir, err)
		}
	}
	defer func() {
		if err = os.RemoveAll(td); err != nil {
			log.L.WithError(err).Warnf("failed remove overlay check directory %s", td)
		}
	}()

	if err = os.Lchown(filepath.Join(td, "upper"), uidMap.HostID, gidMap.HostID); err != nil {
		return false, fmt.Errorf("failed to chown upper directory %s: %w", filepath.Join(td, "upper"), err)
	}

	lowerDir := filepath.Join(td, "lower")
	uidmap := fmt.Sprintf("%d:%d:%d", uidMap.ContainerID, uidMap.HostID, uidMap.Size)
	gidmap := fmt.Sprintf("%d:%d:%d", gidMap.ContainerID, gidMap.HostID, gidMap.Size)

	usernsFd, err := mount.GetUsernsFD(uidmap, gidmap)
	if err != nil {
		return false, err
	}
	defer usernsFd.Close()

	if err = mount.IDMapMount(lowerDir, lowerDir, int(usernsFd.Fd())); err != nil {
		return false, fmt.Errorf("failed to remap lowerdir %s: %w", lowerDir, err)
	}
	defer func() {
		if err = unix.Unmount(lowerDir, 0); err != nil {
			log.L.WithError(err).Warnf("failed to unmount lowerdir %s", lowerDir)
		}
	}()

	opts := fmt.Sprintf("index=off,lowerdir=%s,upperdir=%s,workdir=%s", lowerDir, filepath.Join(td, "upper"), filepath.Join(td, "work"))
	if err = unix.Mount("", filepath.Join(td, "merged"), "overlay", uintptr(unix.MS_RDONLY), opts); err != nil {
		return false, fmt.Errorf("failed to mount idmapped overlay to %s: %w", filepath.Join(td, "merged"), err)
	}
	defer func() {
		if err = unix.Unmount(filepath.Join(td, "merged"), 0); err != nil {
			log.L.WithError(err).Warnf("failed to unmount overlay check directory %s", filepath.Join(td, "merged"))
		}
	}()

	// NOTE: we can't just return true if mount didn't fail since overlay supports
	// idmappings for {lower,upper}dir. That means we need to check merged directory
	// to make sure it completely  supports idmapped mounts.
	st, err := os.Stat(filepath.Join(td, "merged"))
	if err != nil {
		return false, fmt.Errorf("failed to stat %s: %w", filepath.Join(td, "merged"), err)
	}
	if stat, ok := st.Sys().(*syscall.Stat_t); !ok {
		return false, fmt.Errorf("incompatible types after stat call: *syscall.Stat_t expected")
	} else if int(stat.Uid) != uidMap.HostID || int(stat.Gid) != gidMap.HostID {
		return false, fmt.Errorf("bad mapping: expected {uid: %d, gid: %d}; real {uid: %d, gid: %d}", uidMap.HostID, gidMap.HostID, int(stat.Uid), int(stat.Gid))
	}

	return true, nil
}
