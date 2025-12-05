//go:build !cgo && !windows

package platform

func nanotime() int64 {
	return nanotimePortable()
}
