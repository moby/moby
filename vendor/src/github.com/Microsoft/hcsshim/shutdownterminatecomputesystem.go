package hcsshim

import "github.com/Sirupsen/logrus"

// TerminateComputeSystem force terminates a container.
func TerminateComputeSystem(id string, timeout uint32, context string) error {
	return shutdownTerminate(false, id, timeout, context)
}

// ShutdownComputeSystem shuts down a container by requesting a shutdown within
// the container operating system.
func ShutdownComputeSystem(id string, timeout uint32, context string) error {
	return shutdownTerminate(true, id, timeout, context)
}

// shutdownTerminate is a wrapper for ShutdownComputeSystem and TerminateComputeSystem
// which have very similar calling semantics
func shutdownTerminate(shutdown bool, id string, timeout uint32, context string) error {

	var (
		title = "HCSShim::"
	)
	if shutdown {
		title = title + "ShutdownComputeSystem"
	} else {
		title = title + "TerminateComputeSystem"
	}
	logrus.Debugf(title+" id=%s context=%s", id, context)

	var err error
	if shutdown {
		err = shutdownComputeSystem(id, timeout)
	} else {
		err = terminateComputeSystem(id)
	}

	if err != nil {
		return makeErrorf(err, title, "id=%s context=%s", id, context)
	}

	logrus.Debugf(title+" succeeded id=%s context=%s", id, context)
	return nil
}
