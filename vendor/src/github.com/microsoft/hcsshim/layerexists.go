package hcsshim

import (
	"fmt"
	"syscall"
	"unsafe"

	"github.com/Sirupsen/logrus"
)

// LayerExists will return true if a layer with the given id exists and is known
// to the system.
func LayerExists(info DriverInfo, id string) (bool, error) {
	title := "hcsshim::LayerExists "
	logrus.Debugf(title+"Flavour %d ID %s", info.Flavour, id)

	// Load the DLL and get a handle to the procedure we need
	dll, proc, err := loadAndFind(procLayerExists)
	if dll != nil {
		defer dll.Release()
	}
	if err != nil {
		return false, err
	}

	// Convert id to uint16 pointer for calling the procedure
	idp, err := syscall.UTF16PtrFromString(id)
	if err != nil {
		err = fmt.Errorf(title+" - Failed conversion of id %s to pointer %s", id, err)
		logrus.Error(err)
		return false, err
	}

	// Convert info to API calling convention
	infop, err := convertDriverInfo(info)
	if err != nil {
		err = fmt.Errorf(title+" - Failed conversion info struct %s", err)
		logrus.Error(err)
		return false, err
	}

	// Call the procedure itself.
	var exists bool // Outparam from Win32

	r1, _, _ := proc.Call(
		uintptr(unsafe.Pointer(&infop)),
		uintptr(unsafe.Pointer(idp)),
		uintptr(unsafe.Pointer(&exists)))

	use(unsafe.Pointer(&infop))
	use(unsafe.Pointer(idp))

	if r1 != 0 {
		err = fmt.Errorf(title+" - Win32 API call returned error r1=%d err=%s id=%s flavour=%d",
			r1, syscall.Errno(r1), id, info.Flavour)
		logrus.Error(err)
		return false, err
	}

	logrus.Debugf(title+"succeeded flavour=%d id=%s exists=%d", info.Flavour, id, exists)
	return exists, nil
}
