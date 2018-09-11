package wclayer

import (
	"github.com/Microsoft/hcsshim/internal/hcserror"
	"github.com/sirupsen/logrus"
)

// ActivateLayer will find the layer with the given id and mount it's filesystem.
// For a read/write layer, the mounted filesystem will appear as a volume on the
// host, while a read-only layer is generally expected to be a no-op.
// An activated layer must later be deactivated via DeactivateLayer.
func ActivateLayer(path string) error {
	title := "hcsshim::ActivateLayer "
	logrus.Debugf(title+"path %s", path)

	err := activateLayer(&stdDriverInfo, path)
	if err != nil {
		err = hcserror.Errorf(err, title, "path=%s", path)
		logrus.Error(err)
		return err
	}

	logrus.Debugf(title+" - succeeded path=%s", path)
	return nil
}
