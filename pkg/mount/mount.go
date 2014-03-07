package mount

import (
	"fmt"
	"path/filepath"
	"time"
)

func GetMounts() ([]*MountInfo, error) {
	return parseMountTable()
}

// Looks at /proc/self/mountinfo to determine of the specified
// mountpoint has been mounted
func Mounted(mountpoint string) (bool, error) {
	entries, err := parseMountTable()
	if err != nil {
		return false, err
	}

	// Search the table for the mountpoint
	for _, e := range entries {
		if e.Mountpoint == mountpoint {
			return true, nil
		}
	}
	return false, nil
}

// Mount the specified options at the target path only if
// the target is not mounted
// Options must be specified as fstab style
func Mount(device, target, mType, options string) error {
	if mounted, err := Mounted(target); err != nil || mounted {
		return err
	}
	return ForceMount(device, target, mType, options)
}

// Mount the specified options at the target path
// reguardless if the target is mounted or not
// Options must be specified as fstab style
func ForceMount(device, target, mType, options string) error {
	flag, data := parseOptions(options)
	if err := mount(device, target, mType, uintptr(flag), data); err != nil {
		return err
	}
	return nil
}

// Unmount the target only if it is mounted
func Unmount(target string) error {
	if mounted, err := Mounted(target); err != nil || !mounted {
		return err
	}
	return ForceUnmount(target)
}

// Unmount the target reguardless if it is mounted or not
func ForceUnmount(target string) (err error) {
	// Simple retry logic for unmount
	for i := 0; i < 10; i++ {
		if err = unmount(target, 0); err == nil {
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return
}

// FindMountType takes a given path and finds it's filesystem type
func FindMountType(path string) (string, error) {
	mounts, err := GetMounts()
	if err != nil {
		return "", err
	}
	return searchForFsType(path, mounts)
}

// walk up the path until we find path's or path's parent's mountpoint and get the
// fstype
func searchForFsType(path string, mounts []*MountInfo) (string, error) {
	var (
		origpath = path
		cache    = make(map[string]string, len(mounts))
	)
	for _, m := range mounts {
		cache[m.Mountpoint] = m.Fstype
	}

	for path != "" {
		if t, exists := cache[path]; exists {
			return t, nil
		}
		path = filepath.Dir(path)
	}
	return "", fmt.Errorf("no filesystem type found for %s", origpath)

}
