//go:build (!linux && !freebsd && !openbsd && !windows && !darwin) || (freebsd && !cgo) || (openbsd && !cgo)
// +build !linux,!freebsd,!openbsd,!windows,!darwin freebsd,!cgo openbsd,!cgo

package mount

func mount(device, target, mType string, flag uintptr, data string) error {
	panic("cgo required on freebsd and openbsd")
}
