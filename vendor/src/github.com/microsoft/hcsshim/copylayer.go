package hcsshim

import (
	"fmt"
	"syscall"
	"unsafe"

	"github.com/Sirupsen/logrus"
)

func CopyLayer(info DriverInfo, srcId, dstId string, parentLayerPaths []string) error {
	title := "hcsshim::CopyLayer "
	logrus.Debugf(title+"srcId %s dstId", srcId, dstId)

	// Load the DLL and get a handle to the procedure we need
	dll, proc, err := loadAndFind(procCopyLayer)
	if dll != nil {
		defer dll.Release()
	}
	if err != nil {
		return err
	}

	// Convert srcId to uint16 pointer for calling the procedure
	srcIdp, err := syscall.UTF16PtrFromString(srcId)
	if err != nil {
		err = fmt.Errorf(title+" - Failed conversion of srcId %s to pointer %s", srcId, err)
		logrus.Error(err)
		return err
	}

	// Convert dstId to uint16 pointer for calling the procedure
	dstIdp, err := syscall.UTF16PtrFromString(dstId)
	if err != nil {
		err = fmt.Errorf(title+" - Failed conversion of dstId %s to pointer %s", dstId, err)
		logrus.Error(err)
		return err
	}

	// Generate layer descriptors
	layers, err := layerPathsToDescriptors(parentLayerPaths)
	if err != nil {
		err = fmt.Errorf(title+" - Failed to generate layer descriptors %s", err)
		logrus.Error(err)
		return err
	}

	// Convert info to API calling convention
	infop, err := convertDriverInfo(info)
	if err != nil {
		err = fmt.Errorf(title+" - Failed conversion info struct %s", err)
		logrus.Error(err)
		return err
	}

	var layerDescriptorsp *WC_LAYER_DESCRIPTOR
	if len(layers) > 0 {
		layerDescriptorsp = &(layers[0])
	} else {
		layerDescriptorsp = nil
	}

	// Call the procedure itself.
	r1, _, _ := proc.Call(
		uintptr(unsafe.Pointer(&infop)),
		uintptr(unsafe.Pointer(srcIdp)),
		uintptr(unsafe.Pointer(dstIdp)),
		uintptr(unsafe.Pointer(layerDescriptorsp)),
		uintptr(len(layers)))

	use(unsafe.Pointer(&infop))
	use(unsafe.Pointer(srcIdp))
	use(unsafe.Pointer(dstIdp))
	use(unsafe.Pointer(layerDescriptorsp))

	if r1 != 0 {
		err = fmt.Errorf(title+" - Win32 API call returned error r1=%d err=%s srcId=%s dstId=%d",
			r1, syscall.Errno(r1), srcId, dstId)
		logrus.Error(err)
		return err
	}

	logrus.Debugf(title+" - succeeded srcId=%s dstId=%s", srcId, dstId)
	return nil
}
