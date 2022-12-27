//go:build !linux && !windows
// +build !linux,!windows

package sysinfo

import "errors"

// readMemInfo is not supported on platforms other than linux and windows.
func readMemInfo() (*Memory, error) {
	return nil, errors.New("platform and architecture is not supported")
}
