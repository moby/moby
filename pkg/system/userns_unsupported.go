// +build !linux

package system

// RunningInUserNS - User namespaces are only supported in linux
func RunningInUserNS() bool {
	return false
}
