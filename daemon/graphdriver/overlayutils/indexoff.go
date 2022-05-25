//go:build linux
// +build linux

package overlayutils

import "os"

// SupportsIndexOff tests if the kernel's overlay module recognizes the index=off mount option.
func SupportsIndexOff() (bool, error) {
	_, err := os.Stat("/sys/module/overlay/parameters/index")
	if err == nil {
		return true, nil
	} else if os.IsNotExist(err) {
		// old kernel, no index -- do nothing
		return false, nil
	}

	return false, err
}
