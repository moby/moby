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
// Source: https://github.com/containerd/containerd/blob/ccb4c7429e99e101ec0720841605194ef66a879f/core/mount/mount.go#L109-L150
// TODO: Export in containerd or move to a more appropriate package

package containerd

import (
	"strings"

	"github.com/containerd/containerd/mount"
)

// readonlyMounts modifies the received mount options
// to make them readonly
func readonlyMounts(mounts []mount.Mount) []mount.Mount {
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
