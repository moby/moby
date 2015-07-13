package hcsshim

import (
	"fmt"
	"syscall"
	"unsafe"

	"github.com/Sirupsen/logrus"
)

func ResizeConsoleInComputeSystem(id string, processid uint32, h, w int) error {

	title := "HCSShim::ResizeConsoleInComputeSystem"
	logrus.Debugf(title+" id=%s processid=%d (%d,%d)", id, processid, h, w)

	// Load the DLL and get a handle to the procedure we need
	dll, proc, err := loadAndFind(procResizeConsoleInComputeSystem)
	if dll != nil {
		defer dll.Release()
	}
	if err != nil {
		return err
	}

	// Convert id to uint16 pointer for calling the procedure
	idp, err := syscall.UTF16PtrFromString(id)
	if err != nil {
		err = fmt.Errorf(title+" - Failed conversion of id %s to pointer %s", id, err)
		logrus.Error(err)
		return err
	}

	h16 := uint16(h)
	w16 := uint16(w)

	r1, _, _ := proc.Call(uintptr(unsafe.Pointer(idp)), uintptr(processid), uintptr(h16), uintptr(w16), uintptr(0))
	if r1 != 0 {
		err = fmt.Errorf(title+" - Win32 API call returned error r1=%d err=%s, id=%s pid=%d", r1, syscall.Errno(r1), id, processid)
		logrus.Error(err)
		return err
	}

	logrus.Debugf(title+" - succeeded id=%s processid=%d (%d,%d)", id, processid, h, w)
	return nil

}
