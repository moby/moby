package hcsshim

import (
	"fmt"
	"syscall"
	"unsafe"

	"github.com/Sirupsen/logrus"
)

func PrepareLayer(info DriverInfo, layerId string, parentLayerPaths []string) error {
	title := "hcsshim::PrepareLayer "
	logrus.Debugf(title+"flavour %d layerId %s", info.Flavour, layerId)

	// Load the DLL and get a handle to the procedure we need
	dll, proc, err := loadAndFind(procPrepareLayer)
	if dll != nil {
		defer dll.Release()
	}
	if err != nil {
		return err
	}

	// Generate layer descriptors
	layers, err := layerPathsToDescriptors(parentLayerPaths)
	if err != nil {
		err = fmt.Errorf(title+"- Failed to generate layer descriptors ", err)
		return err
	}

	// Convert layerId to uint16 pointer for calling the procedure
	layerIdp, err := syscall.UTF16PtrFromString(layerId)
	if err != nil {
		err = fmt.Errorf(title+"- Failed conversion of layerId %s to pointer %s", layerId, err)
		logrus.Error(err)
		return err
	}

	// Convert info to API calling convention
	infop, err := convertDriverInfo(info)
	if err != nil {
		err = fmt.Errorf(title+"- Failed conversion info struct %s", err)
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
		uintptr(unsafe.Pointer(layerIdp)),
		uintptr(unsafe.Pointer(layerDescriptorsp)),
		uintptr(len(layers)))
	use(unsafe.Pointer(&infop))
	use(unsafe.Pointer(layerIdp))
	use(unsafe.Pointer(layerDescriptorsp))

	if r1 != 0 {
		err = fmt.Errorf(title+"- Win32 API call returned error r1=%d err=%s layerId=%s flavour=%d",
			r1, syscall.Errno(r1), layerId, info.Flavour)
		logrus.Error(err)
		return err
	}

	logrus.Debugf(title+"- succeeded layerId=%s flavour=%d", layerId, info.Flavour)
	return nil
}
