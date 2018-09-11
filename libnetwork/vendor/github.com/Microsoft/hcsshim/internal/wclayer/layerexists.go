package wclayer

import (
	"github.com/Microsoft/hcsshim/internal/hcserror"
	"github.com/sirupsen/logrus"
)

// LayerExists will return true if a layer with the given id exists and is known
// to the system.
func LayerExists(path string) (bool, error) {
	title := "hcsshim::LayerExists "
	logrus.Debugf(title+"path %s", path)

	// Call the procedure itself.
	var exists uint32
	err := layerExists(&stdDriverInfo, path, &exists)
	if err != nil {
		err = hcserror.Errorf(err, title, "path=%s", path)
		logrus.Error(err)
		return false, err
	}

	logrus.Debugf(title+"succeeded path=%s exists=%d", path, exists)
	return exists != 0, nil
}
