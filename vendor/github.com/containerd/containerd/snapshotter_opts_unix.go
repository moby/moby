//go:build !windows
// +build !windows

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

package containerd

import (
	"fmt"

	"github.com/containerd/containerd/snapshots"
)

// WithRemapperLabels creates the labels used by any supporting snapshotter
// to shift the filesystem ownership (user namespace mapping) automatically; currently
// supported by the fuse-overlayfs snapshotter
func WithRemapperLabels(ctrUID, hostUID, ctrGID, hostGID, length uint32) snapshots.Opt {
	return snapshots.WithLabels(map[string]string{
		"containerd.io/snapshot/uidmapping": fmt.Sprintf("%d:%d:%d", ctrUID, hostUID, length),
		"containerd.io/snapshot/gidmapping": fmt.Sprintf("%d:%d:%d", ctrGID, hostGID, length),
	})
}
