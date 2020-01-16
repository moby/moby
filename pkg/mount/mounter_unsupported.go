// +build !linux,!freebsd freebsd,!cgo

package mount // import "github.com/moby/moby/pkg/mount"

func mount(device, target, mType string, flag uintptr, data string) error {
	panic("Not implemented")
}
