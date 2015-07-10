package hcsshim

import (
	"fmt"
	"syscall"
	"unsafe"

	"github.com/Sirupsen/logrus"
)

// TerminateComputeSystem force terminates a container
func TerminateComputeSystem(id string) error {

	var title = "HCSShim::TerminateComputeSystem"
	logrus.Debugf(title+" id=%s", id)

	// Load the DLL and get a handle to the procedure we need
	dll, proc, err := loadAndFind(procTerminateComputeSystem)
	if dll != nil {
		defer dll.Release()
	}
	if err != nil {
		return err
	}

	// Convert id to uint16 pointers for calling the procedure
	idp, err := syscall.UTF16PtrFromString(id)
	if err != nil {
		err = fmt.Errorf(title+" - Failed conversion of id %s to pointer %s", id, err)
		logrus.Error(err)
		return err
	}

	timeout := uint32(0xffffffff)

	// Call the procedure itself.
	r1, _, err := proc.Call(
		uintptr(unsafe.Pointer(idp)), uintptr(timeout))

	use(unsafe.Pointer(idp))

	if r1 != 0 {
		err = fmt.Errorf(title+" - Win32 API call returned error r1=%d err=%s id=%s", r1, syscall.Errno(r1), id)
		return syscall.Errno(r1)
	}

	logrus.Debugf(title+" - succeeded id=%s", id)
	return nil
}
