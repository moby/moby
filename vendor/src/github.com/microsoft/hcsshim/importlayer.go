package hcsshim

import (
	"fmt"
	"syscall"
	"unsafe"

	"github.com/Sirupsen/logrus"
)

// ImportLayer will take the contents of the folder at importFolderPath and import
// that into a layer with the id layerId.  Note that in order to correctly populate
// the layer and interperet the transport format, all parent layers must already
// be present on the system at the paths provided in parentLayerPaths.
func ImportLayer(info DriverInfo, layerId string, importFolderPath string, parentLayerPaths []string) error {
	title := "hcsshim::ImportLayer "
	logrus.Debugf(title+"flavour %d layerId %s folder %s", info.Flavour, layerId, importFolderPath)

	// Load the DLL and get a handle to the procedure we need
	dll, proc, err := loadAndFind(procImportLayer)
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

	// Convert importFolderPath to uint16 pointer for calling the procedure
	importFolderPathp, err := syscall.UTF16PtrFromString(importFolderPath)
	if err != nil {
		err = fmt.Errorf(title+"- Failed conversion of importFolderPath %s to pointer %s", importFolderPath, err)
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
		uintptr(unsafe.Pointer(importFolderPathp)),
		uintptr(unsafe.Pointer(layerDescriptorsp)),
		uintptr(len(layers)))
	use(unsafe.Pointer(&infop))
	use(unsafe.Pointer(layerIdp))
	use(unsafe.Pointer(importFolderPathp))
	use(unsafe.Pointer(layerDescriptorsp))

	if r1 != 0 {
		err = fmt.Errorf(title+"- Win32 API call returned error r1=%d err=%s layerId=%s flavour=%d folder=%s",
			r1, syscall.Errno(r1), layerId, info.Flavour, importFolderPath)
		logrus.Error(err)
		return err
	}

	logrus.Debugf(title+"- succeeded layerId=%s flavour=%d folder=%s", layerId, info.Flavour, importFolderPath)
	return nil
}
