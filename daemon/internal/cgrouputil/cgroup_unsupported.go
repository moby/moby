//go:build !linux

package cgrouputil

import "fmt"

// GetClientCgroup is not supported on non-Linux platforms.
func GetClientCgroup(pid int32) (string, error) {
	return "", fmt.Errorf("cgroup parent from client is only supported on Linux")
}

// IsScopePath is not supported on non-Linux platforms.
func IsScopePath(p string) bool {
	return false
}

// ValidateScopeForDriver is not supported on non-Linux platforms.
// It always returns nil because IsScopePath is always false there.
func ValidateScopeForDriver(cgroupPath string, usingSystemd bool) error {
	return nil
}

// VerifyPIDOwner is not supported on non-Linux platforms.
func VerifyPIDOwner(pid int32, uid uint32) error {
	return fmt.Errorf("cgroup parent from client is only supported on Linux")
}
