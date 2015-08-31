package hcsshim

import (
	"fmt"
	"syscall"
	"unsafe"

	"github.com/Sirupsen/logrus"
)

// GetLayerMountPath will look for a mounted layer with the given id and return
// the path at which that layer can be accessed.  This path may be a volume path
// if the layer is a mounted read-write layer, otherwise it is expected to be the
// folder path at which the layer is stored.
func GetLayerMountPath(info DriverInfo, id string) (string, error) {
	title := "hcsshim::GetLayerMountPath "
	logrus.Debugf(title+"Flavour %s ID %s", info.Flavour, id)

	// Load the DLL and get a handle to the procedure we need
	dll, proc, err := loadAndFind(procGetLayerMountPath)
	if dll != nil {
		defer dll.Release()
	}
	if err != nil {
		return "", err
	}

	// Convert id to uint16 pointer for calling the procedure
	idp, err := syscall.UTF16PtrFromString(id)
	if err != nil {
		err = fmt.Errorf(title+" - Failed conversion of id %s to pointer %s", id, err)
		logrus.Error(err)
		return "", err
	}

	// Convert info to API calling convention
	infop, err := convertDriverInfo(info)
	if err != nil {
		err = fmt.Errorf(title+" - Failed conversion info struct %s", err)
		logrus.Error(err)
		return "", err
	}

	var mountPathLength uint64
	mountPathLength = 0

	// Call the procedure itself.
	logrus.Debugf("Calling proc (1)")
	r1, _, _ := proc.Call(
		uintptr(unsafe.Pointer(&infop)),
		uintptr(unsafe.Pointer(idp)),
		uintptr(unsafe.Pointer(&mountPathLength)),
		uintptr(unsafe.Pointer(nil)))

	if r1 != 0 {
		err = fmt.Errorf(title+" - First Win32 API call returned error r1=%d err=%s id=%s flavour=%d",
			r1, syscall.Errno(r1), id, info.Flavour)
		logrus.Error(err)
		return "", err
	}

	// Allocate a mount path of the returned length.
	if mountPathLength == 0 {
		return "", nil
	}
	mountPathp := make([]uint16, mountPathLength)
	mountPathp[0] = 0

	// Call the procedure again
	logrus.Debugf("Calling proc (2)")
	r1, _, _ = proc.Call(
		uintptr(unsafe.Pointer(&infop)),
		uintptr(unsafe.Pointer(idp)),
		uintptr(unsafe.Pointer(&mountPathLength)),
		uintptr(unsafe.Pointer(&mountPathp[0])))

	use(unsafe.Pointer(&mountPathLength))
	use(unsafe.Pointer(&infop))
	use(unsafe.Pointer(idp))

	if r1 != 0 {
		err = fmt.Errorf(title+" - Second Win32 API call returned error r1=%d err=%s id=%s flavour=%d",
			r1, syscall.Errno(r1), id, info.Flavour)
		logrus.Error(err)
		return "", err
	}

	path := syscall.UTF16ToString(mountPathp[0:])
	logrus.Debugf(title+" - succeeded id=%s flavour=%d path=%s", id, info.Flavour, path)
	return path, nil
}
