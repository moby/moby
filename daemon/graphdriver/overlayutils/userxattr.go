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
	"github.com/containerd/containerd/pkg/userns"
	"github.com/docker/docker/pkg/parsers/kernel"
	"os"
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
func NeedsUserXAttr(ctx *Context, d string) (bool, error) {
	if !userns.RunningInUserNS() {
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

	td, err := os.MkdirTemp(d, "userxattr-")
	if err != nil {
		return false, err
	}
	defer func() {
		if err := os.RemoveAll(td); err != nil {
			ctx.logger.WithError(err).Warnf("Failed to remove check directory %v", td)
		}
	}()

	tm, err := makeTestMount(td, 2)
	if err != nil {
		return false, err
	}

	if err := tm.mount([]string{"userxattr"}); err != nil {
		// Probably the host is running Ubuntu/Debian kernel (< 5.11) with the userns patch but without the userxattr patch.
		// Return false without error.
		ctx.logger.WithError(err).Debugf("cannot mount overlay with \"userxattr\", probably the kernel does not support userxattr")
		return false, nil
	}
	if err := tm.unmount(); err != nil {
		ctx.logger.WithError(err).Warnf("failed to unmount check directory %v", tm.mergedDir)
	}
	return true, nil
}
