// build linux

package daemon

import (
	"fmt"
	"io"
	"io/ioutil"

	"github.com/docker/docker/daemon/execdriver"
	"github.com/docker/docker/engine"
	"github.com/docker/docker/pkg/broadcastwriter"
	"github.com/docker/docker/pkg/ioutils"
	"github.com/docker/docker/pkg/log"
	"github.com/docker/docker/runconfig"
	"github.com/docker/docker/utils"
)

type ExecConfig struct {
	ProcessConfig execdriver.ProcessConfig
	StreamConfig
	OpenStdin bool
}

func (d *Daemon) ContainerExec(job *engine.Job) engine.Status {
	if len(job.Args) != 1 {
		return job.Errorf("Usage: %s container_id command", job.Name)
	}

	var (
		cStdin           io.ReadCloser
		cStdout, cStderr io.Writer
		cStdinCloser     io.Closer
		name             = job.Args[0]
	)

	container := d.Get(name)

	if container == nil {
		return job.Errorf("No such container: %s", name)
	}

	if !container.State.IsRunning() {
		return job.Errorf("Container %s is not not running", name)
	}

	config := runconfig.ExecConfigFromJob(job)

	if config.AttachStdin {
		r, w := io.Pipe()
		go func() {
			defer w.Close()
			io.Copy(w, job.Stdin)
		}()
		cStdin = r
		cStdinCloser = job.Stdin
	}
	if config.AttachStdout {
		cStdout = job.Stdout
	}
	if config.AttachStderr {
		cStderr = job.Stderr
	}

	entrypoint, args := d.getEntrypointAndArgs(nil, config.Cmd)

	processConfig := execdriver.ProcessConfig{
		Privileged: config.Privileged,
		User:       config.User,
		Tty:        config.Tty,
		Entrypoint: entrypoint,
		Arguments:  args,
	}

	execConfig := &ExecConfig{
		OpenStdin:     config.AttachStdin,
		StreamConfig:  StreamConfig{},
		ProcessConfig: processConfig,
	}

	execConfig.StreamConfig.stderr = broadcastwriter.New()
	execConfig.StreamConfig.stdout = broadcastwriter.New()
	// Attach to stdin
	if execConfig.OpenStdin {
		execConfig.StreamConfig.stdin, execConfig.StreamConfig.stdinPipe = io.Pipe()
	} else {
		execConfig.StreamConfig.stdinPipe = ioutils.NopWriteCloser(ioutil.Discard) // Silently drop stdin
	}

	var execErr, attachErr chan error
	go func() {
		attachErr = d.Attach(&execConfig.StreamConfig, config.AttachStdin, false, config.Tty, cStdin, cStdinCloser, cStdout, cStderr)
	}()

	go func() {
		err := container.Exec(execConfig)
		if err != nil {
			err = fmt.Errorf("Cannot run in container %s: %s", name, err)
		}
		execErr <- err
	}()

	select {
	case err := <-attachErr:
		return job.Errorf("attach failed with error: %s", err)
	case err := <-execErr:
		return job.Error(err)
	}

	return engine.StatusOK
}

func (daemon *Daemon) Exec(c *Container, execConfig *ExecConfig, pipes *execdriver.Pipes, startCallback execdriver.StartCallback) (int, error) {
	return daemon.execDriver.Exec(c.command, &execConfig.ProcessConfig, pipes, startCallback)
}

func (container *Container) Exec(execConfig *ExecConfig) error {
	container.Lock()
	defer container.Unlock()

	waitStart := make(chan struct{})

	callback := func(processConfig *execdriver.ProcessConfig, pid int) {
		if processConfig.Tty {
			// The callback is called after the process Start()
			// so we are in the parent process. In TTY mode, stdin/out/err is the PtySlace
			// which we close here.
			if c, ok := processConfig.Stdout.(io.Closer); ok {
				c.Close()
			}
		}
		close(waitStart)
	}

	// We use a callback here instead of a goroutine and an chan for
	// syncronization purposes
	cErr := utils.Go(func() error { return container.monitorExec(execConfig, callback) })

	// Exec should not return until the process is actually running
	select {
	case <-waitStart:
	case err := <-cErr:
		return err
	}

	return nil
}

func (container *Container) monitorExec(execConfig *ExecConfig, callback execdriver.StartCallback) error {
	var (
		err      error
		exitCode int
	)

	pipes := execdriver.NewPipes(execConfig.StreamConfig.stdin, execConfig.StreamConfig.stdout, execConfig.StreamConfig.stderr, execConfig.OpenStdin)
	exitCode, err = container.daemon.Exec(container, execConfig, pipes, callback)
	if err != nil {
		log.Errorf("Error running command in existing container %s: %s", container.ID, err)
	}

	log.Debugf("Exec task in container %s exited with code %d", container.ID, exitCode)
	if execConfig.OpenStdin {
		if err := execConfig.StreamConfig.stdin.Close(); err != nil {
			log.Errorf("Error closing stdin while running in %s: %s", container.ID, err)
		}
	}
	if err := execConfig.StreamConfig.stdout.Clean(); err != nil {
		log.Errorf("Error closing stdout while running in %s: %s", container.ID, err)
	}
	if err := execConfig.StreamConfig.stderr.Clean(); err != nil {
		log.Errorf("Error closing stderr while running in %s: %s", container.ID, err)
	}
	if execConfig.ProcessConfig.Terminal != nil {
		if err := execConfig.ProcessConfig.Terminal.Close(); err != nil {
			log.Errorf("Error closing terminal while running in container %s: %s", container.ID, err)
		}
	}

	return err
}
