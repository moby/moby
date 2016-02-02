package hcsshim

import "github.com/Sirupsen/logrus"

// WaitForProcessInComputeSystem waits for a process ID to terminate and returns
// the exit code. Returns exitcode, errno, error
func WaitForProcessInComputeSystem(id string, processid uint32, timeout uint32) (int32, uint32, error) {

	title := "HCSShim::WaitForProcessInComputeSystem"
	logrus.Debugf(title+" id=%s processid=%d", id, processid)

	var exitCode uint32
	err := waitForProcessInComputeSystem(id, processid, timeout, &exitCode)
	if err != nil {
		err := makeErrorf(err, title, "id=%s", id)
		return 0, err.HResult(), err
	}

	logrus.Debugf(title+" succeeded id=%s processid=%d exitcode=%d", id, processid, exitCode)
	return int32(exitCode), 0, nil
}
