package hcsshim

import (
	"fmt"
	"syscall"
	"unsafe"

	"github.com/Sirupsen/logrus"
)

// TerminateProcessInComputeSystem kills a process in a running container.
func TerminateProcessInComputeSystem(id string, processid uint32) (err error) {

	title := "HCSShim::TerminateProcessInComputeSystem"
	logrus.Debugf(title+" id=%s processid=%d", id, processid)

	// Load the DLL and get a handle to the procedure we need
	dll, proc, err := loadAndFind(procTerminateProcessInComputeSystem)
	if dll != nil {
		defer dll.Release()
	}
	if err != nil {
		return err
	}

	// Convert ID to uint16 pointer for calling the procedure
	idp, err := syscall.UTF16PtrFromString(id)
	if err != nil {
		err = fmt.Errorf(title+" - Failed conversion of id %s to pointer %s", id, err)
		logrus.Error(err)
		return err
	}

	// Call the procedure itself.
	r1, _, err := proc.Call(
		uintptr(unsafe.Pointer(idp)),
		uintptr(processid))

	use(unsafe.Pointer(idp))

	if r1 != 0 {
		err = fmt.Errorf(title+" - Win32 API call returned error r1=%d err=%s id=%s", r1, syscall.Errno(r1), id)
		logrus.Error(err)
		return err
	}

	logrus.Debugf(title+" - succeeded id=%s", id)
	return nil
}
