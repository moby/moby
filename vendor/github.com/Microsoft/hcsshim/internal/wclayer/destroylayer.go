package wclayer

import (
	"github.com/Microsoft/hcsshim/internal/hcserror"
	"github.com/sirupsen/logrus"
)

// DestroyLayer will remove the on-disk files representing the layer with the given
// path, including that layer's containing folder, if any.
func DestroyLayer(path string) error {
	title := "hcsshim::DestroyLayer "
	logrus.Debugf(title+"path %s", path)

	err := destroyLayer(&stdDriverInfo, path)
	if err != nil {
		err = hcserror.Errorf(err, title, "path=%s", path)
		logrus.Error(err)
		return err
	}

	logrus.Debugf(title+"succeeded path=%s", path)
	return nil
}
