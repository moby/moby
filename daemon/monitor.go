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

// containerMonitor monitors the execution of a container's main process.
// If a restart policy is specified for the cotnainer the monitor will ensure that the
// process is restarted based on the rules of the policy.  When the container is finally stopped
// the monitor will reset and cleanup any of the container resources such as networking allocations
// and the rootfs
type containerMonitor struct {
	mux sync.Mutex

	container     *Container
	restartPolicy runconfig.RestartPolicy
	failureCount  int
	shouldStop    bool
}

func newContainerMonitor(container *Container, policy runconfig.RestartPolicy) *containerMonitor {
	return &containerMonitor{
		container:     container,
		restartPolicy: policy,
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

	if m.container.daemon != nil && m.container.daemon.srv != nil && m.container.daemon.srv.IsRunning() {
		// FIXME: here is race condition between two RUN instructions in Dockerfile
		// because they share same runconfig and change image. Must be fixed
		// in builder/builder.go
		if err := m.container.toDisk(); err != nil {
			utils.Errorf("Error dumping container %s state to disk: %s\n", m.container.ID, err)

			return err
		}
	}

	return nil
}

// reset resets the container's IO and ensures that the command is able to be executed again
// by copying the data into a new struct
func (m *containerMonitor) reset() {
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

	if container.daemon != nil && container.daemon.srv != nil {
		container.daemon.srv.LogEvent("die", container.ID, container.daemon.repositories.ImageName(container.Image))
	}

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
}

// Start starts the containers process and monitors it according to the restart policy
func (m *containerMonitor) Start() error {
	var (
		err      error
		exitCode int
	)
	defer m.Close()

	// reset the restart count
	m.container.RestartCount = -1

	for !m.shouldStop {
		m.container.RestartCount++
		if err := m.container.startLoggingToDisk(); err != nil {
			m.reset()

			return err
		}

		pipes := execdriver.NewPipes(m.container.stdin, m.container.stdout, m.container.stderr, m.container.Config.OpenStdin)

		if exitCode, err = m.container.daemon.Run(m.container, pipes, m.callback); err != nil {
			m.failureCount++

			if m.failureCount == m.restartPolicy.MaximumRetryCount {
				m.ExitOnNext()
			}

			utils.Errorf("Error running container: %s", err)
		}

		// We still wait to set the state as stopped and ensure that the locks were released
		m.container.State.SetStopped(exitCode)

		m.reset()

		if m.shouldRestart(exitCode) {
			time.Sleep(1 * time.Second)

			continue
		}

		break
	}

	return err
}

func (m *containerMonitor) shouldRestart(exitCode int) bool {
	m.mux.Lock()

	shouldRestart := (m.restartPolicy.Name == "always" ||
		(m.restartPolicy.Name == "on-failure" && exitCode != 0)) &&
		!m.shouldStop

	m.mux.Unlock()

	return shouldRestart
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
