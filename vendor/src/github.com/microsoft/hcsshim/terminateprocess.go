package hcsshim

import "github.com/Sirupsen/logrus"

// TerminateProcessInComputeSystem kills a process in a running container.
func TerminateProcessInComputeSystem(id string, processid uint32) (err error) {

	title := "HCSShim::TerminateProcessInComputeSystem"
	logrus.Debugf(title+" id=%s processid=%d", id, processid)

	err = terminateProcessInComputeSystem(id, processid)
	if err != nil {
		err = makeErrorf(err, title, "err=%s id=%s", id)
		logrus.Error(err)
		return err
	}

	logrus.Debugf(title+" succeeded id=%s", id)
	return nil
}
