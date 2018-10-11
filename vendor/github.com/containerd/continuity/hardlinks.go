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

package continuity

import (
	"fmt"
	"os"
)

var (
	errNotAHardLink = fmt.Errorf("invalid hardlink")
)

type hardlinkManager struct {
	hardlinks map[hardlinkKey][]Resource
}

func newHardlinkManager() *hardlinkManager {
	return &hardlinkManager{
		hardlinks: map[hardlinkKey][]Resource{},
	}
}

// Add attempts to add the resource to the hardlink manager. If the resource
// cannot be considered as a hardlink candidate, errNotAHardLink is returned.
func (hlm *hardlinkManager) Add(fi os.FileInfo, resource Resource) error {
	if _, ok := resource.(Hardlinkable); !ok {
		return errNotAHardLink
	}

	key, err := newHardlinkKey(fi)
	if err != nil {
		return err
	}

	hlm.hardlinks[key] = append(hlm.hardlinks[key], resource)

	return nil
}

// Merge processes the current state of the hardlink manager and merges any
// shared nodes into hardlinked resources.
func (hlm *hardlinkManager) Merge() ([]Resource, error) {
	var resources []Resource
	for key, linked := range hlm.hardlinks {
		if len(linked) < 1 {
			return nil, fmt.Errorf("no hardlink entrys for dev, inode pair: %#v", key)
		}

		merged, err := Merge(linked...)
		if err != nil {
			return nil, fmt.Errorf("error merging hardlink: %v", err)
		}

		resources = append(resources, merged)
	}

	return resources, nil
}
