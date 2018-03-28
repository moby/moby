// +build !windows

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
