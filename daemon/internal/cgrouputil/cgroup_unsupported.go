//go:build !linux

package cgrouputil

import "fmt"

// GetClientCgroup is not supported on non-Linux platforms.
func GetClientCgroup(pid int32) (string, error) {
	return "", fmt.Errorf("cgroup parent from client is only supported on Linux")
}
