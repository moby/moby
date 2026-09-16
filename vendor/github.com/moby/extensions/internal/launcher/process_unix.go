//go:build unix

package launcher

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"syscall"
)

type processGroupLifetime struct{}

func (processGroupLifetime) Close() error { return nil }

func configureProcessGroup(cmd *exec.Cmd) {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.Setpgid = true
	if cmd.Cancel != nil {
		// exec.CommandContext normally kills only the direct child. Keep
		// cancellation consistent with explicit shutdown by targeting the
		// extension process group.
		cmd.Cancel = func() error { return killProcess(cmd) }
	}
}

func signalProcess(cmd *exec.Cmd, sig os.Signal) error {
	signal, ok := sig.(syscall.Signal)
	if !ok {
		return fmt.Errorf("unsupported extension process signal %T", sig)
	}
	return signalProcessGroup(cmd.Process.Pid, signal)
}

func killProcess(cmd *exec.Cmd) error {
	return signalProcessGroup(cmd.Process.Pid, syscall.SIGKILL)
}

func signalProcessGroup(pgid int, sig syscall.Signal) error {
	err := syscall.Kill(-pgid, sig)
	if errors.Is(err, syscall.ESRCH) {
		return os.ErrProcessDone
	}
	return err
}

func reapProcess(cmd *exec.Cmd) error {
	err := cmd.Wait()
	// The leader may have exited after SIGTERM while descendants ignored it.
	// Kill whatever remains in the group before reporting that Wait completed.
	_ = signalProcessGroup(cmd.Process.Pid, syscall.SIGKILL)
	return err
}
