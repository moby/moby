// +build apparmor,linux,amd64

package apparmor

// #cgo LDFLAGS: -lapparmor
// #include <sys/apparmor.h>
// #include <stdlib.h>
import "C"
import (
	"io/ioutil"
	"unsafe"
)

func IsEnabled() bool {
	buf, err := ioutil.ReadFile("/sys/module/apparmor/parameters/enabled")
	return err == nil && len(buf) > 1 && buf[0] == 'Y'
}

func ApplyProfile(pid int, name string) error {
	if name == "" {
		return nil
	}

	cName := C.CString(name)
	defer C.free(unsafe.Pointer(cName))

	if _, err := C.aa_change_onexec(cName); err != nil {
		return err
	}
	return nil
}
