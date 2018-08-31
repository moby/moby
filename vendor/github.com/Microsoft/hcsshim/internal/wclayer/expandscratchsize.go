package wclayer

import (
	"github.com/Microsoft/hcsshim/internal/hcserror"
	"github.com/sirupsen/logrus"
)

// ExpandScratchSize expands the size of a layer to at least size bytes.
func ExpandScratchSize(path string, size uint64) error {
	title := "hcsshim::ExpandScratchSize "
	logrus.Debugf(title+"path=%s size=%d", path, size)

	err := expandSandboxSize(&stdDriverInfo, path, size)
	if err != nil {
		err = hcserror.Errorf(err, title, "path=%s size=%d", path, size)
		logrus.Error(err)
		return err
	}

	logrus.Debugf(title+"- succeeded path=%s size=%d", path, size)
	return nil
}
