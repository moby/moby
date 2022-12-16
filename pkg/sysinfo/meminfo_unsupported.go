//go:build !linux && !windows
// +build !linux,!windows

package sysinfo

import "errors"

// ReadMemInfo is not supported on platforms other than linux and windows.
func ReadMemInfo() (*Memory, error) {
	return nil, errors.New("platform and architecture is not supported")
}
