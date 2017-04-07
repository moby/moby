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
