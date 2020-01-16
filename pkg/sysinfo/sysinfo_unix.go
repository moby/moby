// +build !linux,!windows

package sysinfo // import "github.com/moby/moby/pkg/sysinfo"

// New returns an empty SysInfo for non linux for now.
func New(quiet bool) *SysInfo {
	sysInfo := &SysInfo{}
	return sysInfo
}
