// +build !linux,!windows,!freebsd

package osl

// GC triggers garbage collection of namespace path right away
// and waits for it.
func GC() {
}
