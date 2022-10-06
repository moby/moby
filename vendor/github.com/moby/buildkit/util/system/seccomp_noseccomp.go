//go:build !seccomp
// +build !seccomp

package system

func SeccompSupported() bool {
	return false
}
