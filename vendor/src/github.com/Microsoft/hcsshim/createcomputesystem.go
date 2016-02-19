package hcsshim

import "github.com/Sirupsen/logrus"

// CreateComputeSystem creates a container, initializing its configuration in
// the Host Compute Service such that it can be started by a call to the
// StartComputeSystem method.
func CreateComputeSystem(id string, configuration string) error {

	title := "HCSShim::CreateComputeSystem"
	logrus.Debugln(title+" id=%s, configuration=%s", id, configuration)

	err := createComputeSystem(id, configuration)
	if err != nil {
		err = makeErrorf(err, title, "id=%s configuration=%s", id, configuration)
		logrus.Error(err)
		return err
	}

	logrus.Debugf(title+"- succeeded %s", id)
	return nil
}
