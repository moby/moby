// +build windows

package system

import (
	"syscall"
)

func Lstat(path string) (*syscall.Win32FileAttributeData, error) {
	// should not be called on cli code path
	return nil, ErrNotSupportedPlatform
}
