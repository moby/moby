package libcontainerd

import (
	"io"
	"strings"
	"syscall"
	"time"

	"github.com/Microsoft/hcsshim"
	"github.com/Sirupsen/logrus"
)

type container struct {
	containerCommon

	// Platform specific fields are below here. There are none presently on Windows.
	options []CreateOption

	// The ociSpec is required, as client.Create() needs a spec,
	// but can be called from the RestartManager context which does not
	// otherwise have access to the Spec
	ociSpec Spec

	manualStopRequested bool
	hcsContainer        hcsshim.Container
}

func (ctr *container) newProcess(friendlyName string) *process {
	return &process{
		processCommon: processCommon{
			containerID:  ctr.containerID,
			friendlyName: friendlyName,
			client:       ctr.client,
		},
	}
}

func (ctr *container) start() error {
	var err error
	isServicing := false

	for _, option := range ctr.options {
		if s, ok := option.(*ServicingOption); ok && s.IsServicing {
			isServicing = true
		}
	}

	// Start the container.  If this is a servicing container, this call will block
	// until the container is done with the servicing execution.
	logrus.Debugln("libcontainerd: starting container ", ctr.containerID)
	if err = ctr.hcsContainer.Start(); err != nil {
		logrus.Errorf("libcontainerd: failed to start container: %s", err)
		if err := ctr.terminate(); err != nil {
			logrus.Errorf("libcontainerd: failed to cleanup after a failed Start. %s", err)
		} else {
			logrus.Debugln("libcontainerd: cleaned up after failed Start by calling Terminate")
		}
		return err
	}

	// Note we always tell HCS to
	// create stdout as it's required regardless of '-i' or '-t' options, so that
	// docker can always grab the output through logs. We also tell HCS to always
	// create stdin, even if it's not used - it will be closed shortly. Stderr
	// is only created if it we're not -t.
	createProcessParms := &hcsshim.ProcessConfig{
		EmulateConsole:   ctr.ociSpec.Process.Terminal,
		WorkingDirectory: ctr.ociSpec.Process.Cwd,
		ConsoleSize:      ctr.ociSpec.Process.InitialConsoleSize,
		CreateStdInPipe:  !isServicing,
		CreateStdOutPipe: !isServicing,
		CreateStdErrPipe: !ctr.ociSpec.Process.Terminal && !isServicing,
	}

	// Configure the environment for the process
	createProcessParms.Environment = setupEnvironmentVariables(ctr.ociSpec.Process.Env)
	createProcessParms.CommandLine = strings.Join(ctr.ociSpec.Process.Args, " ")

	// Start the command running in the container.
	hcsProcess, err := ctr.hcsContainer.CreateProcess(createProcessParms)
	if err != nil {
		logrus.Errorf("libcontainerd: CreateProcess() failed %s", err)
		if err := ctr.terminate(); err != nil {
			logrus.Errorf("libcontainerd: failed to cleanup after a failed CreateProcess. %s", err)
		} else {
			logrus.Debugln("libcontainerd: cleaned up after failed CreateProcess by calling Terminate")
		}
		return err
	}
	ctr.startedAt = time.Now()

	// Save the hcs Process and PID
	ctr.process.friendlyName = InitFriendlyName
	pid := hcsProcess.Pid()
	ctr.process.hcsProcess = hcsProcess

	// If this is a servicing container, wait on the process synchronously here and
	// immediately call shutdown/terminate when it returns.
	if isServicing {
		exitCode := ctr.waitProcessExitCode(&ctr.process)

		if exitCode != 0 {
			logrus.Warnf("libcontainerd: servicing container %s returned non-zero exit code %d", ctr.containerID, exitCode)
			return ctr.terminate()
		}

		return ctr.shutdown()
	}

	var stdout, stderr io.ReadCloser
	var stdin io.WriteCloser
	stdin, stdout, stderr, err = hcsProcess.Stdio()
	if err != nil {
		logrus.Errorf("libcontainerd: failed to get stdio pipes: %s", err)
		if err := ctr.terminate(); err != nil {
			logrus.Errorf("libcontainerd: failed to cleanup after a failed Stdio. %s", err)
		}
		return err
	}

	iopipe := &IOPipe{Terminal: ctr.ociSpec.Process.Terminal}

	iopipe.Stdin = createStdInCloser(stdin, hcsProcess)

	// TEMP: Work around Windows BS/DEL behavior.
	iopipe.Stdin = fixStdinBackspaceBehavior(iopipe.Stdin, ctr.ociSpec.Platform.OSVersion, ctr.ociSpec.Process.Terminal)

	// Convert io.ReadClosers to io.Readers
	if stdout != nil {
		iopipe.Stdout = openReaderFromPipe(stdout)
	}
	if stderr != nil {
		iopipe.Stderr = openReaderFromPipe(stderr)
	}

	// Save the PID
	logrus.Debugf("libcontainerd: process started - PID %d", pid)
	ctr.systemPid = uint32(pid)

	// Spin up a go routine waiting for exit to handle cleanup
	go ctr.waitExit(&ctr.process, true)

	ctr.client.appendContainer(ctr)

	if err := ctr.client.backend.AttachStreams(ctr.containerID, *iopipe); err != nil {
		// OK to return the error here, as waitExit will handle tear-down in HCS
		return err
	}

	// Tell the docker engine that the container has started.
	si := StateInfo{
		CommonStateInfo: CommonStateInfo{
			State: StateStart,
			Pid:   ctr.systemPid, // Not sure this is needed? Double-check monitor.go in daemon BUGBUG @jhowardmsft
		}}
	return ctr.client.backend.StateChanged(ctr.containerID, si)

}

// waitProcessExitCode will wait for the given process to exit and return its error code.
func (ctr *container) waitProcessExitCode(process *process) int {
	// Block indefinitely for the process to exit.
	err := process.hcsProcess.Wait()
	if err != nil {
		if herr, ok := err.(*hcsshim.ProcessError); ok && herr.Err != syscall.ERROR_BROKEN_PIPE {
			logrus.Warnf("libcontainerd: Wait() failed (container may have been killed): %s", err)
		}
		// Fall through here, do not return. This ensures we attempt to continue the
		// shutdown in HCS and tell the docker engine that the process/container
		// has exited to avoid a container being dropped on the floor.
	}

	exitCode, err := process.hcsProcess.ExitCode()
	if err != nil {
		if herr, ok := err.(*hcsshim.ProcessError); ok && herr.Err != syscall.ERROR_BROKEN_PIPE {
			logrus.Warnf("libcontainerd: unable to get exit code from container %s", ctr.containerID)
		}
		// Fall through here, do not return. This ensures we attempt to continue the
		// shutdown in HCS and tell the docker engine that the process/container
		// has exited to avoid a container being dropped on the floor.
	}

	if err := process.hcsProcess.Close(); err != nil {
		logrus.Errorf("libcontainerd: hcsProcess.Close(): %v", err)
	}

	return exitCode
}

// waitExit runs as a goroutine waiting for the process to exit. It's
// equivalent to (in the linux containerd world) where events come in for
// state change notifications from containerd.
func (ctr *container) waitExit(process *process, isFirstProcessToStart bool) error {
	logrus.Debugln("libcontainerd: waitExit() on pid", process.systemPid)

	exitCode := ctr.waitProcessExitCode(process)

	// Assume the container has exited
	si := StateInfo{
		CommonStateInfo: CommonStateInfo{
			State:     StateExit,
			ExitCode:  uint32(exitCode),
			Pid:       process.systemPid,
			ProcessID: process.friendlyName,
		},
		UpdatePending: false,
	}

	// But it could have been an exec'd process which exited
	if !isFirstProcessToStart {
		si.State = StateExitProcess
	} else {
		updatePending, err := ctr.hcsContainer.HasPendingUpdates()
		if err != nil {
			logrus.Warnf("libcontainerd: HasPendingUpdates() failed (container may have been killed): %s", err)
		} else {
			si.UpdatePending = updatePending
		}

		logrus.Debugf("libcontainerd: shutting down container %s", ctr.containerID)
		if err := ctr.shutdown(); err != nil {
			logrus.Debugf("libcontainerd: failed to shutdown container %s", ctr.containerID)
		} else {
			logrus.Debugf("libcontainerd: completed shutting down container %s", ctr.containerID)
		}
		if err := ctr.hcsContainer.Close(); err != nil {
			logrus.Error(err)
		}

		if !ctr.manualStopRequested && ctr.restartManager != nil {
			restart, wait, err := ctr.restartManager.ShouldRestart(uint32(exitCode), false, time.Since(ctr.startedAt))
			if err != nil {
				logrus.Error(err)
			} else if restart {
				si.State = StateRestart
				ctr.restarting = true
				go func() {
					err := <-wait
					ctr.restarting = false
					ctr.client.deleteContainer(ctr.friendlyName)
					if err != nil {
						si.State = StateExit
						if err := ctr.client.backend.StateChanged(ctr.containerID, si); err != nil {
							logrus.Error(err)
						}
						logrus.Error(err)
					} else {
						ctr.client.Create(ctr.containerID, ctr.ociSpec, ctr.options...)
					}
				}()
			}
		}

		// Remove process from list if we have exited
		// We need to do so here in case the Message Handler decides to restart it.
		if si.State == StateExit {
			ctr.client.deleteContainer(ctr.friendlyName)
		}
	}

	// Call into the backend to notify it of the state change.
	logrus.Debugf("libcontainerd: waitExit() calling backend.StateChanged %+v", si)
	if err := ctr.client.backend.StateChanged(ctr.containerID, si); err != nil {
		logrus.Error(err)
	}

	logrus.Debugf("libcontainerd: waitExit() completed OK, %+v", si)
	return nil
}

func (ctr *container) shutdown() error {
	const shutdownTimeout = time.Minute * 5
	err := ctr.hcsContainer.Shutdown()
	if err == hcsshim.ErrVmcomputeOperationPending {
		// Explicit timeout to avoid a (remote) possibility that shutdown hangs indefinitely.
		err = ctr.hcsContainer.WaitTimeout(shutdownTimeout)
	}

	if err != nil {
		logrus.Debugf("libcontainerd: error shutting down container %s %v calling terminate", ctr.containerID, err)
		if err := ctr.terminate(); err != nil {
			return err
		}
		return err
	}

	return nil
}

func (ctr *container) terminate() error {
	const terminateTimeout = time.Minute * 5
	err := ctr.hcsContainer.Terminate()

	if err == hcsshim.ErrVmcomputeOperationPending {
		err = ctr.hcsContainer.WaitTimeout(terminateTimeout)
	}

	if err != nil {
		logrus.Debugf("libcontainerd: error terminating container %s %v", ctr.containerID, err)
		return err
	}

	return nil
}
