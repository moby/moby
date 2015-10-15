package hcsshim

import (
	"fmt"
	"syscall"
	"unsafe"

	"github.com/Sirupsen/logrus"
)

// TerminateComputeSystem force terminates a container.
func TerminateComputeSystem(id string, timeout uint32, context string) (uint32, error) {
	return shutdownTerminate(false, id, timeout, context)
}

// ShutdownComputeSystem shuts down a container by requesting a shutdown within
// the container operating system.
func ShutdownComputeSystem(id string, timeout uint32, context string) (uint32, error) {
	return shutdownTerminate(true, id, timeout, context)
}

// shutdownTerminate is a wrapper for ShutdownComputeSystem and TerminateComputeSystem
// which have very similar calling semantics
func shutdownTerminate(shutdown bool, id string, timeout uint32, context string) (uint32, error) {

	var (
		title    = "HCSShim::"
		procName string
	)
	if shutdown {
		title = title + "ShutdownComputeSystem"
		procName = procShutdownComputeSystem
	} else {
		title = title + "TerminateComputeSystem"
		procName = procTerminateComputeSystem
	}
	logrus.Debugf(title+" id=%s context=%s", id, context)

	// Load the DLL and get a handle to the procedure we need
	dll, proc, err := loadAndFind(procName)
	if dll != nil {
		defer dll.Release()
	}
	if err != nil {
		return 0xffffffff, err
	}

	// Convert id to uint16 pointers for calling the procedure
	idp, err := syscall.UTF16PtrFromString(id)
	if err != nil {
		err = fmt.Errorf(title+" - Failed conversion of id %s to pointer %s", id, err)
		logrus.Error(err)
		return 0xffffffff, err
	}

	// Call the procedure itself.
	r1, _, err := proc.Call(
		uintptr(unsafe.Pointer(idp)), uintptr(timeout))

	use(unsafe.Pointer(idp))

	if r1 != 0 {
		err = fmt.Errorf(title+" - Win32 API call returned error r1=0x%X err=%s id=%s context=%s", r1, syscall.Errno(r1), id, context)
		return uint32(r1), err
	}

	logrus.Debugf(title+" succeeded id=%s context=%s", id, context)
	return 0, nil
}
