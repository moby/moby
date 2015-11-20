package daemon

import (
	"io"
	"os/exec"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/docker/docker/daemon/execdriver"
	derr "github.com/docker/docker/errors"
	"github.com/docker/docker/pkg/stringid"
	"github.com/docker/docker/runconfig"
	"github.com/docker/docker/utils"
)

const (
	defaultTimeIncrement = 100
	loggerCloseTimeout   = 10 * time.Second
)

// containerSupervisor defines the interface that a supervisor must implement
type containerSupervisor interface {
	// LogContainerEvent generates events related to a given container
	LogContainerEvent(*Container, string)
	// Cleanup ensures that the container is properly unmounted
	Cleanup(*Container)
	// StartLogging starts the logging driver for the container
	StartLogging(*Container) error
	// Run starts a container
	Run(c *Container, pipes *execdriver.Pipes, startCallback execdriver.DriverCallback) (execdriver.ExitStatus, error)
	// IsShuttingDown tells whether the supervisor is shutting down or not
	IsShuttingDown() bool
}

// containerMonitor monitors the execution of a container's main process.
// If a restart policy is specified for the container the monitor will ensure that the
// process is restarted based on the rules of the policy.  When the container is finally stopped
// the monitor will reset and cleanup any of the container resources such as networking allocations
// and the rootfs
type containerMonitor struct {
	mux sync.Mutex

	// supervisor keeps track of the container and the events it generates
	supervisor containerSupervisor

	// container is the container being monitored
	container *Container

	// restartPolicy is the current policy being applied to the container monitor
	restartPolicy runconfig.RestartPolicy

	// failureCount is the number of times the container has failed to
	// start in a row
	failureCount int

	// shouldStop signals the monitor that the next time the container exits it is
	// either because docker or the user asked for the container to be stopped
	shouldStop bool

	// startSignal is a channel that is closes after the container initially starts
	startSignal chan struct{}

	// stopChan is used to signal to the monitor whenever there is a wait for the
	// next restart so that the timeIncrement is not honored and the user is not
	// left waiting for nothing to happen during this time
	stopChan chan struct{}

	// timeIncrement is the amount of time to wait between restarts
	// this is in milliseconds
	timeIncrement int

	// lastStartTime is the time which the monitor last exec'd the container's process
	lastStartTime time.Time
}

// newContainerMonitor returns an initialized containerMonitor for the provided container
// honoring the provided restart policy
func (daemon *Daemon) newContainerMonitor(container *Container, policy runconfig.RestartPolicy) *containerMonitor {
	return &containerMonitor{
		supervisor:    daemon,
		container:     container,
		restartPolicy: policy,
		timeIncrement: defaultTimeIncrement,
		stopChan:      make(chan struct{}),
		startSignal:   make(chan struct{}),
	}
}

// Stop signals to the container monitor that it should stop monitoring the container
// for exits the next time the process dies
func (m *containerMonitor) ExitOnNext() {
	m.mux.Lock()

	// we need to protect having a double close of the channel when stop is called
	// twice or else we will get a panic
	if !m.shouldStop {
		m.shouldStop = true
		close(m.stopChan)
	}

	m.mux.Unlock()
}

// Close closes the container's resources such as networking allocations and
// unmounts the contatiner's root filesystem
func (m *containerMonitor) Close() error {
	// Cleanup networking and mounts
	m.supervisor.Cleanup(m.container)

	// FIXME: here is race condition between two RUN instructions in Dockerfile
	// because they share same runconfig and change image. Must be fixed
	// in builder/builder.go
	if err := m.container.toDisk(); err != nil {
		logrus.Errorf("Error dumping container %s state to disk: %s", m.container.ID, err)

		return err
	}

	return nil
}

// Start starts the containers process and monitors it according to the restart policy
func (m *containerMonitor) Start() error {
	var (
		err        error
		exitStatus execdriver.ExitStatus
		// this variable indicates where we in execution flow:
		// before Run or after
		afterRun bool
	)

	// ensure that when the monitor finally exits we release the networking and unmount the rootfs
	defer func() {
		if afterRun {
			m.container.Lock()
			defer m.container.Unlock()
			m.container.setStopped(&exitStatus)
		}
		m.Close()
	}()
	// reset stopped flag
	if m.container.HasBeenManuallyStopped {
		m.container.HasBeenManuallyStopped = false
	}

	// reset the restart count
	m.container.RestartCount = -1

	for {
		m.container.RestartCount++

		if err := m.supervisor.StartLogging(m.container); err != nil {
			m.resetContainer(false)

			return err
		}

		pipes := execdriver.NewPipes(m.container.Stdin(), m.container.Stdout(), m.container.Stderr(), m.container.Config.OpenStdin)

		m.logEvent("start")

		m.lastStartTime = time.Now()

		if exitStatus, err = m.supervisor.Run(m.container, pipes, m.callback); err != nil {
			// if we receive an internal error from the initial start of a container then lets
			// return it instead of entering the restart loop
			// set to 127 for container cmd not found/does not exist)
			if strings.Contains(err.Error(), "executable file not found") ||
				strings.Contains(err.Error(), "no such file or directory") ||
				strings.Contains(err.Error(), "system cannot find the file specified") {
				if m.container.RestartCount == 0 {
					m.container.ExitCode = 127
					m.resetContainer(false)
					return derr.ErrorCodeCmdNotFound
				}
			}
			// set to 126 for container cmd can't be invoked errors
			if strings.Contains(err.Error(), syscall.EACCES.Error()) {
				if m.container.RestartCount == 0 {
					m.container.ExitCode = 126
					m.resetContainer(false)
					return derr.ErrorCodeCmdCouldNotBeInvoked
				}
			}

			if m.container.RestartCount == 0 {
				m.container.ExitCode = -1
				m.resetContainer(false)

				return derr.ErrorCodeCantStart.WithArgs(m.container.ID, utils.GetErrorMessage(err))
			}

			logrus.Errorf("Error running container: %s", err)
		}

		// here container.Lock is already lost
		afterRun = true

		m.resetMonitor(err == nil && exitStatus.ExitCode == 0)

		if m.shouldRestart(exitStatus.ExitCode) {
			m.container.setRestarting(&exitStatus)
			m.logEvent("die")
			m.resetContainer(true)

			// sleep with a small time increment between each restart to help avoid issues cased by quickly
			// restarting the container because of some types of errors ( networking cut out, etc... )
			m.waitForNextRestart()

			// we need to check this before reentering the loop because the waitForNextRestart could have
			// been terminated by a request from a user
			if m.shouldStop {
				return err
			}
			continue
		}

		m.logEvent("die")
		m.resetContainer(true)
		return err
	}
}

// resetMonitor resets the stateful fields on the containerMonitor based on the
// previous runs success or failure.  Regardless of success, if the container had
// an execution time of more than 10s then reset the timer back to the default
func (m *containerMonitor) resetMonitor(successful bool) {
	executionTime := time.Now().Sub(m.lastStartTime).Seconds()

	if executionTime > 10 {
		m.timeIncrement = defaultTimeIncrement
	} else {
		// otherwise we need to increment the amount of time we wait before restarting
		// the process.  We will build up by multiplying the increment by 2
		m.timeIncrement *= 2
	}

	// the container exited successfully so we need to reset the failure counter
	if successful {
		m.failureCount = 0
	} else {
		m.failureCount++
	}
}

// waitForNextRestart waits with the default time increment to restart the container unless
// a user or docker asks for the container to be stopped
func (m *containerMonitor) waitForNextRestart() {
	select {
	case <-time.After(time.Duration(m.timeIncrement) * time.Millisecond):
	case <-m.stopChan:
	}
}

// shouldRestart checks the restart policy and applies the rules to determine if
// the container's process should be restarted
func (m *containerMonitor) shouldRestart(exitCode int) bool {
	m.mux.Lock()
	defer m.mux.Unlock()

	// do not restart if the user or docker has requested that this container be stopped
	if m.shouldStop {
		m.container.HasBeenManuallyStopped = !m.supervisor.IsShuttingDown()
		return false
	}

	switch {
	case m.restartPolicy.IsAlways(), m.restartPolicy.IsUnlessStopped():
		return true
	case m.restartPolicy.IsOnFailure():
		// the default value of 0 for MaximumRetryCount means that we will not enforce a maximum count
		if max := m.restartPolicy.MaximumRetryCount; max != 0 && m.failureCount > max {
			logrus.Debugf("stopping restart of container %s because maximum failure could of %d has been reached",
				stringid.TruncateID(m.container.ID), max)
			return false
		}

		return exitCode != 0
	}

	return false
}

// callback ensures that the container's state is properly updated after we
// received ack from the execution drivers
func (m *containerMonitor) callback(processConfig *execdriver.ProcessConfig, pid int, chOOM <-chan struct{}) error {
	go func() {
		_, ok := <-chOOM
		if ok {
			m.logEvent("oom")
		}
	}()

	if processConfig.Tty {
		// The callback is called after the process Start()
		// so we are in the parent process. In TTY mode, stdin/out/err is the PtySlave
		// which we close here.
		if c, ok := processConfig.Stdout.(io.Closer); ok {
			c.Close()
		}
	}

	m.container.setRunning(pid)

	// signal that the process has started
	// close channel only if not closed
	select {
	case <-m.startSignal:
	default:
		close(m.startSignal)
	}

	if err := m.container.toDiskLocking(); err != nil {
		logrus.Errorf("Error saving container to disk: %v", err)
	}
	return nil
}

// resetContainer resets the container's IO and ensures that the command is able to be executed again
// by copying the data into a new struct
// if lock is true, then container locked during reset
func (m *containerMonitor) resetContainer(lock bool) {
	container := m.container
	if lock {
		container.Lock()
		defer container.Unlock()
	}

	if err := container.CloseStreams(); err != nil {
		logrus.Errorf("%s: %s", container.ID, err)
	}

	if container.command != nil && container.command.ProcessConfig.Terminal != nil {
		if err := container.command.ProcessConfig.Terminal.Close(); err != nil {
			logrus.Errorf("%s: Error closing terminal: %s", container.ID, err)
		}
	}

	// Re-create a brand new stdin pipe once the container exited
	if container.Config.OpenStdin {
		container.NewInputPipes()
	}

	if container.logDriver != nil {
		if container.logCopier != nil {
			exit := make(chan struct{})
			go func() {
				container.logCopier.Wait()
				close(exit)
			}()
			select {
			case <-time.After(loggerCloseTimeout):
				logrus.Warnf("Logger didn't exit in time: logs may be truncated")
			case <-exit:
			}
		}
		container.logDriver.Close()
		container.logCopier = nil
		container.logDriver = nil
	}

	c := container.command.ProcessConfig.Cmd

	container.command.ProcessConfig.Cmd = exec.Cmd{
		Stdin:       c.Stdin,
		Stdout:      c.Stdout,
		Stderr:      c.Stderr,
		Path:        c.Path,
		Env:         c.Env,
		ExtraFiles:  c.ExtraFiles,
		Args:        c.Args,
		Dir:         c.Dir,
		SysProcAttr: c.SysProcAttr,
	}
}

func (m *containerMonitor) logEvent(action string) {
	m.supervisor.LogContainerEvent(m.container, action)
}
