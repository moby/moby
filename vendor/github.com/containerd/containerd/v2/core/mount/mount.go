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

package mount

import (
	"fmt"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/containerd/containerd/api/types"
	"github.com/containerd/continuity/fs"
)

// HasBindMounts This is a flag to conditionally disable code that relies on working bind-mount support, so such code is easier to find across codebase.
const HasBindMounts = runtime.GOOS != "darwin" && runtime.GOOS != "openbsd"

// Mount is the lingua franca of containerd. A mount represents a
// serialized mount syscall. Components either emit or consume mounts.
type Mount struct {
	// Type specifies the host-specific of the mount.
	Type string
	// Source specifies where to mount from. Depending on the host system, this
	// can be a source path or device.
	Source string
	// Target specifies an optional subdirectory as a mountpoint. It assumes that
	// the subdirectory exists in a parent mount.
	Target string
	// Options contains zero or more fstab-style mount options. Typically,
	// these are platform specific.
	Options []string
}

// All mounts all the provided mounts to the provided target. If submounts are
// present, it assumes that parent mounts come before child mounts.
func All(mounts []Mount, target string) error {
	for _, m := range mounts {
		if err := m.Mount(target); err != nil {
			return err
		}
	}
	return nil
}

// UnmountMounts unmounts all the mounts under a target in the reverse order of
// the mounts array provided.
func UnmountMounts(mounts []Mount, target string, flags int) error {
	for i := len(mounts) - 1; i >= 0; i-- {
		mountpoint, err := fs.RootPath(target, mounts[i].Target)
		if err != nil {
			return err
		}

		if err := UnmountAll(mountpoint, flags); err != nil {
			if i == len(mounts)-1 { // last mount
				return err
			}
		}
	}
	return nil
}

// CanonicalizePath makes path absolute and resolves symlinks in it.
// Path must exist.
func CanonicalizePath(path string) (string, error) {
	// Abs also does Clean, so we do not need to call it separately
	path, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}

	return filepath.EvalSymlinks(path)
}

// ReadOnly returns a boolean value indicating whether this mount has the "ro"
// option set.
func (m *Mount) ReadOnly() bool {
	for _, option := range m.Options {
		if option == "ro" {
			return true
		}
	}
	return false
}

// Mount to the provided target path.
func (m *Mount) Mount(target string) error {
	target, err := fs.RootPath(target, m.Target)
	if err != nil {
		return fmt.Errorf("failed to join path %q with root %q: %w", m.Target, target, err)
	}
	return m.mount(target)
}

// readonlyMounts modifies the received mount options
// to make them readonly
func readonlyMounts(mounts []Mount) []Mount {
	for i, m := range mounts {
		if m.Type == "overlay" {
			mounts[i].Options = readonlyOverlay(m.Options)
			continue
		}
		opts := make([]string, 0, len(m.Options))
		for _, opt := range m.Options {
			if opt != "rw" && opt != "ro" { // skip `ro` too so we don't append it twice
				opts = append(opts, opt)
			}
		}
		opts = append(opts, "ro")
		mounts[i].Options = opts
	}
	return mounts
}

// readonlyOverlay takes mount options for overlay mounts and makes them readonly by
// removing workdir and upperdir (and appending the upperdir layer to lowerdir) - see:
// https://www.kernel.org/doc/html/latest/filesystems/overlayfs.html#multiple-lower-layers
func readonlyOverlay(opt []string) []string {
	out := make([]string, 0, len(opt))
	upper := ""
	for _, o := range opt {
		if strings.HasPrefix(o, "upperdir=") {
			upper = strings.TrimPrefix(o, "upperdir=")
		} else if !strings.HasPrefix(o, "workdir=") {
			out = append(out, o)
		}
	}
	if upper != "" {
		for i, o := range out {
			if strings.HasPrefix(o, "lowerdir=") {
				out[i] = "lowerdir=" + upper + ":" + strings.TrimPrefix(o, "lowerdir=")
			}
		}
	}
	return out
}

// ToProto converts from [Mount] to the containerd
// APIs protobuf definition of a Mount.
func ToProto(mounts []Mount) []*types.Mount {
	apiMounts := make([]*types.Mount, len(mounts))
	for i, m := range mounts {
		apiMounts[i] = &types.Mount{
			Type:    m.Type,
			Source:  m.Source,
			Target:  m.Target,
			Options: m.Options,
		}
	}
	return apiMounts
}

// FromProto converts from the protobuf definition [types.Mount] to
// [Mount].
func FromProto(mm []*types.Mount) []Mount {
	mounts := make([]Mount, len(mm))
	for i, m := range mm {
		mounts[i] = Mount{
			Type:    m.Type,
			Source:  m.Source,
			Target:  m.Target,
			Options: m.Options,
		}
	}
	return mounts
}
