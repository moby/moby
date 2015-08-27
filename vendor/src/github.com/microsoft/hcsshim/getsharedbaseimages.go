package hcsshim

import (
	"fmt"
	"syscall"
	"unsafe"

	"github.com/Sirupsen/logrus"
)

// GetSharedBaseImages will enumerate the images stored in the common central
// image store and return descriptive info about those images for the purpose
// of registering them with the graphdriver, graph, and tagstore.
func GetSharedBaseImages() (imageData string, err error) {
	title := "hcsshim::GetSharedBaseImages "

	// Load the DLL and get a handle to the procedure we need
	dll, proc, err := loadAndFind(procGetSharedBaseImages)
	if dll != nil {
		defer dll.Release()
	}
	if err != nil {
		return
	}

	// Load the OLE DLL and get a handle to the CoTaskMemFree procedure
	dll2, proc2, err := loadAndFindFromDll(oleDLLName, procCoTaskMemFree)
	if dll2 != nil {
		defer dll2.Release()
	}
	if err != nil {
		return
	}

	var output uintptr

	// Call the procedure again
	logrus.Debugf("Calling proc")
	r1, _, _ := proc.Call(
		uintptr(unsafe.Pointer(&output)))

	if r1 != 0 {
		err = fmt.Errorf(title+" - Win32 API call returned error r1=%d errno=%s",
			r1, syscall.Errno(r1))
		logrus.Error(err)
		return
	}

	// Defer the cleanup of the memory using CoTaskMemFree
	defer proc2.Call(output)

	imageData = syscall.UTF16ToString((*[1 << 30]uint16)(unsafe.Pointer(output))[:])
	logrus.Debugf(title+" - succeeded output=%s", imageData)

	return
}
