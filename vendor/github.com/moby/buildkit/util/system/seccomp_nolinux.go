// +build !linux,seccomp

package system

func SeccompSupported() bool {
	return false
}
