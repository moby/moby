// +build !linux,!freebsd,!openbsd,!windows freebsd,!cgo openbsd,!cgo

package mount

func mount(device, target, mType string, flag uintptr, data string) error {
	panic("cgo required on freebsd and openbsd")
}
