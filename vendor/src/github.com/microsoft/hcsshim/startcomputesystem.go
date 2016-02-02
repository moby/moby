package hcsshim

import "github.com/Sirupsen/logrus"

// StartComputeSystem starts a container that has previously been created via
// CreateComputeSystem.
func StartComputeSystem(id string) error {

	title := "HCSShim::StartComputeSystem"
	logrus.Debugf(title+" id=%s", id)

	err := startComputeSystem(id)
	if err != nil {
		err = makeErrorf(err, title, "id=%s", id)
		logrus.Error(err)
		return err
	}

	logrus.Debugf(title+" succeeded id=%s", id)
	return nil
}
