// +build !windows

package system // import "github.com/moby/moby/pkg/system"

// InitLCOW does nothing since LCOW is a windows only feature
func InitLCOW(experimental bool) {
}

// ContainerdRuntimeSupported returns true if the use of ContainerD runtime is supported.
func ContainerdRuntimeSupported(_ bool, _ string) bool {
	return true
}
