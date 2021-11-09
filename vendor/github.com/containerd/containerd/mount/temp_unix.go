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
	"os"
	"path/filepath"
	"sort"

	"github.com/moby/sys/mountinfo"
)

// SetTempMountLocation sets the temporary mount location
func SetTempMountLocation(root string) error {
	root, err := filepath.Abs(root)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(root, 0700); err != nil {
		return err
	}
	tempMountLocation = root
	return nil
}

// CleanupTempMounts all temp mounts and remove the directories
func CleanupTempMounts(flags int) (warnings []error, err error) {
	mounts, err := mountinfo.GetMounts(mountinfo.PrefixFilter(tempMountLocation))
	if err != nil {
		return nil, err
	}
	// Make the deepest mount be first
	sort.Slice(mounts, func(i, j int) bool {
		return len(mounts[i].Mountpoint) > len(mounts[j].Mountpoint)
	})
	for _, mount := range mounts {
		if err := UnmountAll(mount.Mountpoint, flags); err != nil {
			warnings = append(warnings, err)
			continue
		}
		if err := os.Remove(mount.Mountpoint); err != nil {
			warnings = append(warnings, err)
		}
	}
	return warnings, nil
}
