package wclayer

import (
	"github.com/Microsoft/hcsshim/internal/hcserror"
	"github.com/sirupsen/logrus"
)

// UnprepareLayer disables the filesystem filter for the read-write layer with
// the given id.
func UnprepareLayer(path string) error {
	title := "hcsshim::UnprepareLayer "
	logrus.Debugf(title+"path %s", path)

	err := unprepareLayer(&stdDriverInfo, path)
	if err != nil {
		err = hcserror.Errorf(err, title, "path=%s", path)
		logrus.Error(err)
		return err
	}

	logrus.Debugf(title+"succeeded path=%s", path)
	return nil
}
