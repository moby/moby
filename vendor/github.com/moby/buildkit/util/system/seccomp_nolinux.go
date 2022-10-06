//go:build !linux && seccomp
// +build !linux,seccomp

package system

func SeccompSupported() bool {
	return false
}
