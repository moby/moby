package mount

import (
	"time"
)

// Looks at /proc/self/mountinfo to determine of the specified
// mountpoint has been mounted
func Mounted(mountpoint string) (bool, error) {
	entries, err := parseMountTable()
	if err != nil {
		return false, err
	}

	// Search the table for the mountpoint
	for _, e := range entries {
		if e.mountpoint == mountpoint {
			return true, nil
		}
	}
	return false, nil
}

// Mount the specified options at the target path
// Options must be specified as fstab style
func Mount(device, target, mType, options string) error {
	if mounted, err := Mounted(target); err != nil || mounted {
		return err
	}

	flag, data := parseOptions(options)
	if err := mount(device, target, mType, uintptr(flag), data); err != nil {
		return err
	}
	return nil

}

// Unmount the target only if it is mounted
func Unmount(target string) (err error) {
	if mounted, err := Mounted(target); err != nil || !mounted {
		return err
	}

	// Simple retry logic for unmount
	for i := 0; i < 10; i++ {
		if err = unmount(target, 0); err == nil {
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return
}
