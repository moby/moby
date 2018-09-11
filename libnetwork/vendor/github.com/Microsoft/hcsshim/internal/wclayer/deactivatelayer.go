package wclayer

import (
	"github.com/Microsoft/hcsshim/internal/hcserror"
	"github.com/sirupsen/logrus"
)

// DeactivateLayer will dismount a layer that was mounted via ActivateLayer.
func DeactivateLayer(path string) error {
	title := "hcsshim::DeactivateLayer "
	logrus.Debugf(title+"path %s", path)

	err := deactivateLayer(&stdDriverInfo, path)
	if err != nil {
		err = hcserror.Errorf(err, title, "path=%s", path)
		logrus.Error(err)
		return err
	}

	logrus.Debugf(title+"succeeded path=%s", path)
	return nil
}
