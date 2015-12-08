package hcsshim

import (
	"fmt"
	"syscall"
	"unsafe"

	"github.com/Sirupsen/logrus"
)

// WaitForProcessInComputeSystem waits for a process ID to terminate and returns
// the exit code. Returns exitcode, errno, error
func WaitForProcessInComputeSystem(id string, processid uint32, timeout uint32) (int32, uint32, error) {

	title := "HCSShim::WaitForProcessInComputeSystem"
	logrus.Debugf(title+" id=%s processid=%d", id, processid)

	// Load the DLL and get a handle to the procedure we need
	dll, proc, err := loadAndFind(procWaitForProcessInComputeSystem)
	if dll != nil {
		defer dll.Release()
	}
	if err != nil {
		return 0, 0, err
	}

	// Convert id to uint16 pointer for calling the procedure
	idp, err := syscall.UTF16PtrFromString(id)
	if err != nil {
		err = fmt.Errorf(title+" - Failed conversion of id %s to pointer %s", id, err)
		logrus.Error(err)
		return 0, 0, err
	}

	// To get a POINTER to the ExitCode
	ec := new(int32)

	// Call the procedure itself.
	r1, _, err := proc.Call(
		uintptr(unsafe.Pointer(idp)),
		uintptr(processid),
		uintptr(timeout),
		uintptr(unsafe.Pointer(ec)))

	use(unsafe.Pointer(idp))

	if r1 != 0 {
		err = fmt.Errorf(title+" - Win32 API call returned error r1=0x%X err=%s id=%s", r1, syscall.Errno(r1), id)
		return 0, uint32(r1), err
	}

	logrus.Debugf(title+" succeeded id=%s processid=%d exitcode=%d", id, processid, *ec)
	return *ec, 0, nil
}
