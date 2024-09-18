//go:build !linux && !windows

package sysinfo

func numCPU() int {
	// not implemented
	return 0
}
