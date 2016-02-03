package hcsshim

import "github.com/Sirupsen/logrus"

// ImportLayer will take the contents of the folder at importFolderPath and import
// that into a layer with the id layerId.  Note that in order to correctly populate
// the layer and interperet the transport format, all parent layers must already
// be present on the system at the paths provided in parentLayerPaths.
func ImportLayer(info DriverInfo, layerId string, importFolderPath string, parentLayerPaths []string) error {
	title := "hcsshim::ImportLayer "
	logrus.Debugf(title+"flavour %d layerId %s folder %s", info.Flavour, layerId, importFolderPath)

	// Generate layer descriptors
	layers, err := layerPathsToDescriptors(parentLayerPaths)
	if err != nil {
		return err
	}

	// Convert info to API calling convention
	infop, err := convertDriverInfo(info)
	if err != nil {
		logrus.Error(err)
		return err
	}

	err = importLayer(&infop, layerId, importFolderPath, layers)
	if err != nil {
		err = makeErrorf(err, title, "layerId=%s flavour=%d folder=%s", layerId, info.Flavour, importFolderPath)
		logrus.Error(err)
		return err
	}

	logrus.Debugf(title+"succeeded flavour=%d layerId=%s folder=%s", info.Flavour, layerId, importFolderPath)
	return nil
}
