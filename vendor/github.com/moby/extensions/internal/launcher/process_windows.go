package launcher

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"unsafe"

	"golang.org/x/sys/windows"
)

// windowsProcessLifetime keeps the extension in a kill-on-close job.
// If dockerd terminates without running its shutdown hooks, Windows closes the
// job handle and prevents the extension process from being orphaned.
type windowsProcessLifetime struct {
	job windows.Handle
}

func startProcess(cmd *exec.Cmd) (processLifetime, <-chan error, error) {
	lifetime, err := newWindowsProcessLifetime()
	if err != nil {
		return nil, nil, err
	}
	if err := cmd.Start(); err != nil {
		_ = lifetime.Close()
		return nil, nil, err
	}
	if err := lifetime.assign(cmd.Process.Pid); err != nil {
		if killErr := cmd.Process.Kill(); killErr != nil && !errors.Is(killErr, os.ErrProcessDone) {
			err = errors.Join(err, fmt.Errorf("kill unassigned extension process: %w", killErr))
		}
		_ = cmd.Wait()
		if closeErr := lifetime.Close(); closeErr != nil {
			err = errors.Join(err, closeErr)
		}
		return nil, nil, fmt.Errorf("assign extension process to job: %w", err)
	}
	wait := make(chan error, 1)
	go func() { wait <- cmd.Wait() }()
	return lifetime, wait, nil
}

func signalProcess(cmd *exec.Cmd, sig os.Signal) error { return cmd.Process.Signal(sig) }

func killProcess(cmd *exec.Cmd) error { return cmd.Process.Kill() }

func newWindowsProcessLifetime() (*windowsProcessLifetime, error) {
	job, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		return nil, fmt.Errorf("create extension process job: %w", err)
	}
	info := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{}
	info.BasicLimitInformation.LimitFlags = windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE
	if _, err := windows.SetInformationJobObject(
		job,
		windows.JobObjectExtendedLimitInformation,
		uintptr(unsafe.Pointer(&info)),
		uint32(unsafe.Sizeof(info)),
	); err != nil {
		_ = windows.CloseHandle(job)
		return nil, fmt.Errorf("configure extension process job: %w", err)
	}
	return &windowsProcessLifetime{job: job}, nil
}

func (l *windowsProcessLifetime) assign(pid int) error {
	process, err := windows.OpenProcess(
		windows.PROCESS_SET_QUOTA|windows.PROCESS_TERMINATE,
		false,
		uint32(pid),
	)
	if err != nil {
		return err
	}
	defer func() { _ = windows.CloseHandle(process) }()
	return windows.AssignProcessToJobObject(l.job, process)
}

func (l *windowsProcessLifetime) Close() error {
	if l.job == 0 {
		return nil
	}
	err := windows.CloseHandle(l.job)
	l.job = 0
	if err != nil {
		return fmt.Errorf("close extension process job: %w", err)
	}
	return nil
}
