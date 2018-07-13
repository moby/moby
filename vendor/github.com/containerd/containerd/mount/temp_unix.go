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
	"strings"
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
func CleanupTempMounts(flags int) error {
	mounts, err := Self()
	if err != nil {
		return err
	}
	var toUnmount []string
	for _, m := range mounts {
		if strings.HasPrefix(m.Mountpoint, tempMountLocation) {
			toUnmount = append(toUnmount, m.Mountpoint)
		}
	}
	sort.Sort(sort.Reverse(sort.StringSlice(toUnmount)))
	for _, path := range toUnmount {
		if err := UnmountAll(path, flags); err != nil {
			return err
		}
		if err := os.Remove(path); err != nil {
			return err
		}
	}
	return nil
}
