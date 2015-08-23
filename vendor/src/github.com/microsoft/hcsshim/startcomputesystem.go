package hcsshim

import (
	"fmt"
	"syscall"
	"unsafe"

	"github.com/Sirupsen/logrus"
)

// StartComputeSystem starts a container
func StartComputeSystem(id string) error {

	title := "HCSShim::StartComputeSystem"
	logrus.Debugf(title+" id=%s", id)

	// Load the DLL and get a handle to the procedure we need
	dll, proc, err := loadAndFind(procStartComputeSystem)
	if dll != nil {
		defer dll.Release()
	}
	if err != nil {
		return err
	}

	// Convert ID to uint16 pointers for calling the procedure
	idp, err := syscall.UTF16PtrFromString(id)
	if err != nil {
		err = fmt.Errorf(title+" - Failed conversion of id %s to pointer %s", id, err)
		logrus.Error(err)
		return err
	}

	// Call the procedure itself.
	r1, _, _ := proc.Call(uintptr(unsafe.Pointer(idp)))

	use(unsafe.Pointer(idp))

	if r1 != 0 {
		err = fmt.Errorf(title+" - Win32 API call returned error r1=%d err=%s id=%s", r1, syscall.Errno(r1), id)
		logrus.Error(err)
		return err
	}

	logrus.Debugf("HCSShim::StartComputeSystem - succeeded id=%s", id)
	return nil
}
