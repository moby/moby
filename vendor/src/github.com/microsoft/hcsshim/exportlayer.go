package hcsshim

import (
	"fmt"
	"syscall"
	"unsafe"

	"github.com/Sirupsen/logrus"
)

// ExportLayer will create a folder at exportFolderPath and fill that folder with
// the transport format version of the layer identified by layerId. This transport
// format includes any metadata required for later importing the layer (using
// ImportLayer), and requires the full list of parent layer paths in order to
// perform the export.
func ExportLayer(info DriverInfo, layerId string, exportFolderPath string, parentLayerPaths []string) error {
	title := "hcsshim::ExportLayer "
	logrus.Debugf(title+"flavour %d layerId %s folder %s", info.Flavour, layerId, exportFolderPath)

	// Load the DLL and get a handle to the procedure we need
	dll, proc, err := loadAndFind(procExportLayer)
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

	// Convert exportFolderPath to uint16 pointer for calling the procedure
	exportFolderPathp, err := syscall.UTF16PtrFromString(exportFolderPath)
	if err != nil {
		err = fmt.Errorf(title+"- Failed conversion of exportFolderPath %s to pointer %s", exportFolderPath, err)
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
		uintptr(unsafe.Pointer(exportFolderPathp)),
		uintptr(unsafe.Pointer(layerDescriptorsp)),
		uintptr(len(layers)))
	use(unsafe.Pointer(&infop))
	use(unsafe.Pointer(layerIdp))
	use(unsafe.Pointer(exportFolderPathp))
	use(unsafe.Pointer(layerDescriptorsp))

	if r1 != 0 {
		err = fmt.Errorf(title+"- Win32 API call returned error r1=%d err=%s layerId=%s flavour=%d folder=%s",
			r1, syscall.Errno(r1), layerId, info.Flavour, exportFolderPath)
		logrus.Error(err)
		return err
	}

	logrus.Debugf(title+"succeeded flavour=%d layerId=%s folder=%s", info.Flavour, layerId, exportFolderPath)
	return nil
}
