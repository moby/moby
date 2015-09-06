package hcsshim

import (
	"fmt"
	"syscall"
	"unsafe"

	"github.com/Sirupsen/logrus"
)

// CreateComputeSystem creates a container, initializing its configuration in
// the Host Compute Service such that it can be started by a call to the
// StartComputeSystem method.
func CreateComputeSystem(id string, configuration string) error {

	title := "HCSShim::CreateComputeSystem"
	logrus.Debugln(title+" id=%s, configuration=%s", id, configuration)

	// Load the DLL and get a handle to the procedure we need
	dll, proc, err := loadAndFind(procCreateComputeSystem)
	if dll != nil {
		defer dll.Release()
	}
	if err != nil {
		return err
	}

	// Convert id to uint16 pointers for calling the procedure
	idp, err := syscall.UTF16PtrFromString(id)
	if err != nil {
		err = fmt.Errorf(title+"- Failed conversion of id %s to pointer %s", id, err)
		logrus.Error(err)
		return err
	}

	// Convert configuration to uint16 pointers for calling the procedure
	configurationp, err := syscall.UTF16PtrFromString(configuration)
	if err != nil {
		err = fmt.Errorf(title+" - Failed conversion of configuration %s to pointer %s", configuration, err)
		logrus.Error(err)
		return err
	}

	// Call the procedure itself.
	r1, _, _ := proc.Call(
		uintptr(unsafe.Pointer(idp)), uintptr(unsafe.Pointer(configurationp)))

	use(unsafe.Pointer(idp))
	use(unsafe.Pointer(configurationp))

	if r1 != 0 {
		err = fmt.Errorf(title+" - Win32 API call returned error r1=%d err=%s id=%s configuration=%s", r1, syscall.Errno(r1), id, configuration)
		logrus.Error(err)
		return err
	}

	logrus.Debugf(title+"- succeeded %s", id)
	return nil
}
