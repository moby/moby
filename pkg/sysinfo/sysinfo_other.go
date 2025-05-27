//go:build !linux

package sysinfo // import "github.com/docker/docker/pkg/sysinfo"

// New returns an empty SysInfo for non linux for now.
func New(options ...Opt) *SysInfo {
	return &SysInfo{}
}

func isCpusetListAvailable(string, map[int]struct{}) (bool, error) {
	return false, nil
}
