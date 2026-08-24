package launcher

import (
	"os/exec"
	"runtime"
	"syscall"
)

func startProcess(cmd *exec.Cmd) (processLifetime, <-chan error, error) {
	configureProcessGroup(cmd)
	// Pdeathsig only applies to the direct child. Process-group cleanup covers
	// host-driven shutdown, not abrupt parent death.
	cmd.SysProcAttr.Pdeathsig = syscall.SIGKILL

	started := make(chan error, 1)
	wait := make(chan error, 1)
	go func() {
		// Pdeathsig is tied to the Linux thread that creates the child, not only
		// to the parent process. Keep that thread alive until Wait has reaped it.
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()
		if err := cmd.Start(); err != nil {
			started <- err
			return
		}
		started <- nil
		wait <- reapProcess(cmd)
	}()
	if err := <-started; err != nil {
		return nil, nil, err
	}
	return processGroupLifetime{}, wait, nil
}
