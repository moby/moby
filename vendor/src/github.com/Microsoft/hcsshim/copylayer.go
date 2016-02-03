package hcsshim

import "github.com/Sirupsen/logrus"

// CopyLayer performs a commit of the srcId (which is expected to be a read-write
// layer) into a new read-only layer at dstId.  This requires the full list of
// on-disk paths to parent layers, provided in parentLayerPaths, in order to
// complete the commit.
func CopyLayer(info DriverInfo, srcId, dstId string, parentLayerPaths []string) error {
	title := "hcsshim::CopyLayer "
	logrus.Debugf(title+"srcId %s dstId", srcId, dstId)

	// Generate layer descriptors
	layers, err := layerPathsToDescriptors(parentLayerPaths)
	if err != nil {
		return err
	}

	// Convert info to API calling convention
	infop, err := convertDriverInfo(info)
	if err != nil {
		return err
	}

	err = copyLayer(&infop, srcId, dstId, layers)
	if err != nil {
		err = makeErrorf(err, title, "srcId=%s dstId=%d", srcId, dstId)
		logrus.Error(err)
		return err
	}

	logrus.Debugf(title+" - succeeded srcId=%s dstId=%s", srcId, dstId)
	return nil
}
