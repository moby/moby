package wclayer

import (
	"syscall"

	"github.com/Microsoft/hcsshim/internal/hcserror"
	"github.com/sirupsen/logrus"
)

// GetLayerMountPath will look for a mounted layer with the given path and return
// the path at which that layer can be accessed.  This path may be a volume path
// if the layer is a mounted read-write layer, otherwise it is expected to be the
// folder path at which the layer is stored.
func GetLayerMountPath(path string) (string, error) {
	title := "hcsshim::GetLayerMountPath "
	logrus.Debugf(title+"path %s", path)

	var mountPathLength uintptr
	mountPathLength = 0

	// Call the procedure itself.
	logrus.Debugf("Calling proc (1)")
	err := getLayerMountPath(&stdDriverInfo, path, &mountPathLength, nil)
	if err != nil {
		err = hcserror.Errorf(err, title, "(first call) path=%s", path)
		logrus.Error(err)
		return "", err
	}

	// Allocate a mount path of the returned length.
	if mountPathLength == 0 {
		return "", nil
	}
	mountPathp := make([]uint16, mountPathLength)
	mountPathp[0] = 0

	// Call the procedure again
	logrus.Debugf("Calling proc (2)")
	err = getLayerMountPath(&stdDriverInfo, path, &mountPathLength, &mountPathp[0])
	if err != nil {
		err = hcserror.Errorf(err, title, "(second call) path=%s", path)
		logrus.Error(err)
		return "", err
	}

	mountPath := syscall.UTF16ToString(mountPathp[0:])
	logrus.Debugf(title+"succeeded path=%s mountPath=%s", path, mountPath)
	return mountPath, nil
}
