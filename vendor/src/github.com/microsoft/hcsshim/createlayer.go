package hcsshim

import (
	"fmt"
	"syscall"
	"unsafe"

	"github.com/Sirupsen/logrus"
)

// CreateLayer creates a new, empty, read-only layer on the filesystem based on
// the parent layer provided.
func CreateLayer(info DriverInfo, id, parent string) error {
	title := "hcsshim::CreateLayer "
	logrus.Debugf(title+"Flavour %s ID %s parent %s", info.Flavour, id, parent)

	// Load the DLL and get a handle to the procedure we need
	dll, proc, err := loadAndFind(procCreateLayer)
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

	// Convert parent to uint16 pointer for calling the procedure
	parentp, err := syscall.UTF16PtrFromString(parent)
	if err != nil {
		err = fmt.Errorf(title+" - Failed conversion of parent %s to pointer %s", parent, err)
		logrus.Error(err)
		return err
	}

	// Convert info to API calling convention
	infop, err := convertDriverInfo(info)
	if err != nil {
		err = fmt.Errorf(title+" - Failed conversion info struct %s", parent, err)
		logrus.Error(err)
		return err
	}

	// Call the procedure itself.
	r1, _, _ := proc.Call(
		uintptr(unsafe.Pointer(&infop)),
		uintptr(unsafe.Pointer(idp)),
		uintptr(unsafe.Pointer(parentp)))

	use(unsafe.Pointer(&infop))
	use(unsafe.Pointer(idp))
	use(unsafe.Pointer(parentp))

	if r1 != 0 {
		err = fmt.Errorf(title+" - Win32 API call returned error r1=%d err=%s id=%s parent=%s flavour=%d",
			r1, syscall.Errno(r1), id, parent, info.Flavour)
		logrus.Error(err)
		return err
	}

	logrus.Debugf(title+" - succeeded id=%s parent=%s flavour=%d", id, parent, info.Flavour)
	return nil
}
