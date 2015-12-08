// +build windows

package windows

import (
	"fmt"
	"syscall"

	"github.com/Sirupsen/logrus"
	"github.com/docker/docker/daemon/execdriver"
	"github.com/microsoft/hcsshim"
)

// Terminate implements the exec driver Driver interface.
func (d *Driver) Terminate(p *execdriver.Command) error {
	return kill(p.ID, p.ContainerPid, syscall.SIGTERM)
}

// Kill implements the exec driver Driver interface.
func (d *Driver) Kill(p *execdriver.Command, sig int) error {
	return kill(p.ID, p.ContainerPid, syscall.Signal(sig))
}

func kill(id string, pid int, sig syscall.Signal) error {
	logrus.Debugf("WindowsExec: kill() id=%s pid=%d sig=%d", id, pid, sig)
	var err error
	context := fmt.Sprintf("kill: sig=%d pid=%d", sig, pid)

	if sig == syscall.SIGKILL || forceKill {
		// Terminate the compute system
		if errno, err := hcsshim.TerminateComputeSystem(id, hcsshim.TimeoutInfinite, context); err != nil {
			logrus.Errorf("Failed to terminate %s - 0x%X %q", id, errno, err)
		}

	} else {
		// Terminate Process
		if err = hcsshim.TerminateProcessInComputeSystem(id, uint32(pid)); err != nil {
			logrus.Warnf("Failed to terminate pid %d in %s: %q", pid, id, err)
			// Ignore errors
			err = nil
		}

		// Shutdown the compute system
		if errno, err := hcsshim.ShutdownComputeSystem(id, hcsshim.TimeoutInfinite, context); err != nil {
			logrus.Errorf("Failed to shutdown %s - 0x%X %q", id, errno, err)
		}
	}
	return err
}
