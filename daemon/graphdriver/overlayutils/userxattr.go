//go:build linux
// +build linux

// Forked from https://github.com/containerd/containerd/blob/9ade247b38b5a685244e1391c86ff41ab109556e/snapshots/overlay/check.go
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
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/containerd/containerd/mount"
	"github.com/containerd/containerd/sys"
	"github.com/docker/docker/pkg/parsers/kernel"
	"github.com/sirupsen/logrus"
)

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
	if !sys.RunningInUserNS() {
		// we are the real root (i.e., the root in the initial user NS),
		// so we do never need "userxattr" opt.
		return false, nil
	}

	// Fast path for kernel >= 5.11 .
	//
	// Keep in mind that distro vendors might be going to backport the patch to older kernels.
	// So we can't completely remove the "slow path".
	if kernel.CheckKernelVersion(5, 11, 0) {
		return true, nil
	}

	tdRoot := filepath.Join(d, "userxattr-check")
	if err := os.RemoveAll(tdRoot); err != nil {
		logrus.WithError(err).Warnf("Failed to remove check directory %v", tdRoot)
	}

	if err := os.MkdirAll(tdRoot, 0700); err != nil {
		return false, err
	}

	defer func() {
		if err := os.RemoveAll(tdRoot); err != nil {
			logrus.WithError(err).Warnf("Failed to remove check directory %v", tdRoot)
		}
	}()

	td, err := ioutil.TempDir(tdRoot, "")
	if err != nil {
		return false, err
	}

	for _, dir := range []string{"lower1", "lower2", "upper", "work", "merged"} {
		if err := os.Mkdir(filepath.Join(td, dir), 0755); err != nil {
			return false, err
		}
	}

	opts := []string{
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
		logrus.WithError(err).Debugf("cannot mount overlay with \"userxattr\", probably the kernel does not support userxattr")
		return false, nil
	}
	if err := mount.UnmountAll(dest, 0); err != nil {
		logrus.WithError(err).Warnf("Failed to unmount check directory %v", dest)
	}
	return true, nil
}
