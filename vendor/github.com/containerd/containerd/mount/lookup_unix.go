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

package mount

import (
	"path/filepath"

	"github.com/moby/sys/mountinfo"
	"github.com/pkg/errors"
)

// Lookup returns the mount info corresponds to the path.
func Lookup(dir string) (Info, error) {
	dir = filepath.Clean(dir)

	m, err := mountinfo.GetMounts(mountinfo.ParentsFilter(dir))
	if err != nil {
		return Info{}, errors.Wrapf(err, "failed to find the mount info for %q", dir)
	}
	if len(m) == 0 {
		return Info{}, errors.Errorf("failed to find the mount info for %q", dir)
	}

	// find the longest matching mount point
	var idx, maxlen int
	for i := range m {
		if len(m[i].Mountpoint) > maxlen {
			maxlen = len(m[i].Mountpoint)
			idx = i
		}
	}
	return *m[idx], nil
}
