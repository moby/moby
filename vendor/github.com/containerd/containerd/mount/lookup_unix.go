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
	"sort"
	"strings"

	"github.com/pkg/errors"
)

// Lookup returns the mount info corresponds to the path.
func Lookup(dir string) (Info, error) {
	dir = filepath.Clean(dir)

	mounts, err := Self()
	if err != nil {
		return Info{}, err
	}

	// Sort descending order by Info.Mountpoint
	sort.SliceStable(mounts, func(i, j int) bool {
		return mounts[j].Mountpoint < mounts[i].Mountpoint
	})
	for _, m := range mounts {
		// Note that m.{Major, Minor} are generally unreliable for our purpose here
		// https://www.spinics.net/lists/linux-btrfs/msg58908.html
		// Note that device number is not checked here, because for overlayfs files
		// may have different device number with the mountpoint.
		if strings.HasPrefix(dir, m.Mountpoint) {
			return m, nil
		}
	}

	return Info{}, errors.Errorf("failed to find the mount info for %q", dir)
}
