package wclayer

import (
	"github.com/Microsoft/hcsshim/internal/hcserror"
	"github.com/Microsoft/hcsshim/internal/interop"
	"github.com/sirupsen/logrus"
)

// GetSharedBaseImages will enumerate the images stored in the common central
// image store and return descriptive info about those images for the purpose
// of registering them with the graphdriver, graph, and tagstore.
func GetSharedBaseImages() (imageData string, err error) {
	title := "hcsshim::GetSharedBaseImages "

	logrus.Debugf("Calling proc")
	var buffer *uint16
	err = getBaseImages(&buffer)
	if err != nil {
		err = hcserror.New(err, title, "")
		logrus.Error(err)
		return
	}
	imageData = interop.ConvertAndFreeCoTaskMemString(buffer)
	logrus.Debugf(title+" - succeeded output=%s", imageData)
	return
}
