package hcsshim

import "github.com/Sirupsen/logrus"

// ResizeConsoleInComputeSystem updates the height and width of the console
// session for the process with the given id in the container with the given id.
func ResizeConsoleInComputeSystem(id string, processid uint32, h, w int) error {

	title := "HCSShim::ResizeConsoleInComputeSystem"
	logrus.Debugf(title+" id=%s processid=%d (%d,%d)", id, processid, h, w)

	err := resizeConsoleInComputeSystem(id, processid, uint16(h), uint16(w), 0)
	if err != nil {
		err = makeErrorf(err, title, "id=%s pid=%d", id, processid)
		logrus.Error(err)
		return err
	}

	logrus.Debugf(title+" succeeded id=%s processid=%d (%d,%d)", id, processid, h, w)
	return nil

}
