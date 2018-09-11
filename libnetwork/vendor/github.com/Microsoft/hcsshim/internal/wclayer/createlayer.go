package wclayer

import (
	"github.com/Microsoft/hcsshim/internal/hcserror"
	"github.com/sirupsen/logrus"
)

// CreateLayer creates a new, empty, read-only layer on the filesystem based on
// the parent layer provided.
func CreateLayer(path, parent string) error {
	title := "hcsshim::CreateLayer "
	logrus.Debugf(title+"Flavour %d ID %s parent %s", path, parent)

	err := createLayer(&stdDriverInfo, path, parent)
	if err != nil {
		err = hcserror.Errorf(err, title, "path=%s parent=%s flavour=%d", path, parent)
		logrus.Error(err)
		return err
	}

	logrus.Debugf(title+" - succeeded path=%s parent=%s flavour=%d", path, parent)
	return nil
}
