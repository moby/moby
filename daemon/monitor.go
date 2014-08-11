package daemon

import (
	"io"
	"os/exec"
	"sync"
	"time"

	"github.com/docker/docker/daemon/execdriver"
	"github.com/docker/docker/runconfig"
	"github.com/docker/docker/utils"
)

const defaultTimeIncrement = 100

// containerMonitor monitors the execution of a container's main process.
// If a restart policy is specified for the cotnainer the monitor will ensure that the
// process is restarted based on the rules of the policy.  When the container is finally stopped
// the monitor will reset and cleanup any of the container resources such as networking allocations
// and the rootfs
type containerMonitor struct {
	mux sync.Mutex

	// container is the container being monitored
	container *Container

	// restartPolicy is the being applied to the container monitor
	restartPolicy runconfig.RestartPolicy

	// failureCount is the number of times the container has failed to
	// start in a row
	failureCount int

	// shouldStop signals the monitor that the next time the container exits it is
	// either because docker or the user asked for the container to be stopped
	shouldStop bool

	// timeIncrement is the amount of time to wait between restarts
	// this is in milliseconds
	timeIncrement int
}

func newContainerMonitor(container *Container, policy runconfig.RestartPolicy) *containerMonitor {
	return &containerMonitor{
		container:     container,
		restartPolicy: policy,
		timeIncrement: defaultTimeIncrement,
	}
}

// Stop signals to the container monitor that it should stop monitoring the container
// for exits the next time the process dies
func (m *containerMonitor) ExitOnNext() {
	m.mux.Lock()
	m.shouldStop = true
	m.mux.Unlock()
}

// Close closes the container's resources such as networking allocations and
// unmounts the contatiner's root filesystem
func (m *containerMonitor) Close() error {
	// Cleanup networking and mounts
	m.container.cleanup()

	// FIXME: here is race condition between two RUN instructions in Dockerfile
	// because they share same runconfig and change image. Must be fixed
	// in builder/builder.go
	if err := m.container.toDisk(); err != nil {
		utils.Errorf("Error dumping container %s state to disk: %s\n", m.container.ID, err)

		return err
	}

	return nil
}

// reset resets the container's IO and ensures that the command is able to be executed again
// by copying the data into a new struct
func (m *containerMonitor) reset(successful bool) {
	container := m.container

	if container.Config.OpenStdin {
		if err := container.stdin.Close(); err != nil {
			utils.Errorf("%s: Error close stdin: %s", container.ID, err)
		}
	}

	if err := container.stdout.Clean(); err != nil {
		utils.Errorf("%s: Error close stdout: %s", container.ID, err)
	}

	if err := container.stderr.Clean(); err != nil {
		utils.Errorf("%s: Error close stderr: %s", container.ID, err)
	}

	if container.command != nil && container.command.Terminal != nil {
		if err := container.command.Terminal.Close(); err != nil {
			utils.Errorf("%s: Error closing terminal: %s", container.ID, err)
		}
	}

	// Re-create a brand new stdin pipe once the container exited
	if container.Config.OpenStdin {
		container.stdin, container.stdinPipe = io.Pipe()
	}

	container.LogEvent("die")

	c := container.command.Cmd

	container.command.Cmd = exec.Cmd{
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

	// the container exited successfully so we need to reset the failure counter
	// and the timeIncrement back to the default values
	if successful {
		m.failureCount = 0
		m.timeIncrement = defaultTimeIncrement
	} else {
		// otherwise we need to increment the amount of time we wait before restarting
		// the process.  We will build up by multiplying the increment by 2

		m.failureCount++
		m.timeIncrement *= 2
	}
}

// Start starts the containers process and monitors it according to the restart policy
func (m *containerMonitor) Start() error {
	var (
		err        error
		exitStatus int
	)

	// ensure that when the monitor finally exits we release the networking and unmount the rootfs
	defer m.Close()

	// reset the restart count
	m.container.RestartCount = -1

	for !m.shouldStop {
		m.container.RestartCount++

		if err := m.container.startLoggingToDisk(); err != nil {
			m.reset(false)

			return err
		}

		pipes := execdriver.NewPipes(m.container.stdin, m.container.stdout, m.container.stderr, m.container.Config.OpenStdin)

		m.container.LogEvent("start")

		if exitStatus, err = m.container.daemon.Run(m.container, pipes, m.callback); err != nil {
			utils.Errorf("Error running container: %s", err)
		}

		// we still wait to set the state as stopped and ensure that the locks were released
		m.container.State.SetStopped(exitStatus)

		// pass if we exited successfully
		m.reset(err == nil && exitStatus == 0)

		if m.shouldRestart(exitStatus) {
			// sleep with a small time increment between each restart to help avoid issues cased by quickly
			// restarting the container because of some types of errors ( networking cut out, etc... )
			time.Sleep(time.Duration(m.timeIncrement) * time.Millisecond)

			continue
		}

		break
	}

	return err
}

// shouldRestart checks the restart policy and applies the rules to determine if
// the container's process should be restarted
func (m *containerMonitor) shouldRestart(exitStatus int) bool {
	m.mux.Lock()
	defer m.mux.Unlock()

	// do not restart if the user or docker has requested that this container be stopped
	if m.shouldStop {
		return false
	}

	switch m.restartPolicy.Name {
	case "always":
		return true
	case "on-failure":
		// the default value of 0 for MaximumRetryCount means that we will not enforce a maximum count
		if max := m.restartPolicy.MaximumRetryCount; max != 0 && m.failureCount >= max {
			utils.Debugf("stopping restart of container %s because maximum failure could of %d has been reached", max)
			return false
		}

		return exitStatus != 0
	}

	return false
}

// callback ensures that the container's state is properly updated after we
// received ack from the execution drivers
func (m *containerMonitor) callback(command *execdriver.Command) {
	if command.Tty {
		// The callback is called after the process Start()
		// so we are in the parent process. In TTY mode, stdin/out/err is the PtySlace
		// which we close here.
		if c, ok := command.Stdout.(io.Closer); ok {
			c.Close()
		}
	}

	m.container.State.SetRunning(command.Pid())

	if err := m.container.ToDisk(); err != nil {
		utils.Debugf("%s", err)
	}
}
