package hcsshim

import (
	"fmt"
	"syscall"
	"unsafe"

	"github.com/Sirupsen/logrus"
)

// NameToGuid converts the given string into a GUID using the algorithm in the
// Host Compute Service, ensuring GUIDs generated with the same string are common
// across all clients.
func NameToGuid(name string) (id GUID, err error) {
	title := "hcsshim::NameToGuid "
	logrus.Debugf(title+"Name %s", name)

	// Load the DLL and get a handle to the procedure we need
	dll, proc, err := loadAndFind(procNameToGuid)
	if dll != nil {
		defer dll.Release()
	}
	if err != nil {
		return
	}

	// Convert name to uint16 pointer for calling the procedure
	namep, err := syscall.UTF16PtrFromString(name)
	if err != nil {
		err = fmt.Errorf(title+" - Failed conversion of name %s to pointer %s", name, err)
		logrus.Error(err)
		return
	}

	// Call the procedure itself.
	logrus.Debugf("Calling proc")
	r1, _, _ := proc.Call(
		uintptr(unsafe.Pointer(namep)),
		uintptr(unsafe.Pointer(&id)))

	if r1 != 0 {
		err = fmt.Errorf(title+" - Win32 API call returned error r1=%d err=%s name=%s",
			r1, syscall.Errno(r1), name)
		logrus.Error(err)
		return
	}

	return
}
