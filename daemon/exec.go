// build linux

package daemon

import (
	"fmt"
	"io"
	"io/ioutil"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/docker/docker/daemon/execdriver"
	"github.com/docker/docker/daemon/execdriver/lxc"
	"github.com/docker/docker/engine"
	"github.com/docker/docker/pkg/broadcastwriter"
	"github.com/docker/docker/pkg/ioutils"
	"github.com/docker/docker/pkg/promise"
	"github.com/docker/docker/pkg/signal"
	"github.com/docker/docker/runconfig"
	"github.com/docker/docker/utils"
)

type execConfig struct {
	sync.Mutex
	ID            string
	Running       bool
	ExitCode      int
	ProcessConfig execdriver.ProcessConfig
	StreamConfig
	OpenStdin  bool
	OpenStderr bool
	OpenStdout bool
	Container  *Container
	waitChan   chan struct{}
}

type execStore struct {
	s map[string]*execConfig
	sync.Mutex
}

func newExecStore() *execStore {
	return &execStore{s: make(map[string]*execConfig, 0)}
}

func (e *execStore) Add(id string, execConfig *execConfig) {
	e.Lock()
	e.s[id] = execConfig
	e.Unlock()
}

func (e *execStore) Get(id string) *execConfig {
	e.Lock()
	res := e.s[id]
	e.Unlock()
	return res
}

func (e *execStore) Delete(id string) {
	e.Lock()
	delete(e.s, id)
	e.Unlock()
}

func (execConfig *execConfig) Resize(h, w int) error {
	return execConfig.ProcessConfig.Terminal.Resize(h, w)
}

func (c *execConfig) isRunning() bool {
	c.Lock()
	r := c.Running
	c.Unlock()
	return r
}

func (c *execConfig) setRunning() {
	c.Lock()
	c.Running = true
	c.Unlock()
}

func (c *execConfig) waitStop(timeout time.Duration) error {
	c.Lock()
	if !c.Running {
		c.Unlock()
		return nil
	}
	waitChan := c.waitChan
	c.Unlock()
	return wait(waitChan, timeout)
}

func (c *execConfig) killSig(sig int) error {
	p := c.ProcessConfig.Process

	// If no signal is passed, or SIGKILL, perform regular Kill (SIGKILL + wait())
	if sig == 0 || syscall.Signal(sig) == syscall.SIGKILL {
		if err := p.Kill(); err != nil {
			return fmt.Errorf("Cannot kill exec process: %s", err)
		}
		// wait for the process to die
		if err := c.waitStop(-1 * time.Second); err != nil {
			return fmt.Errorf("Cannot kill exec process: %s", err)
		}
	} else {
		// Otherwise, just send the requested signal
		if err := p.Signal(syscall.Signal(sig)); err != nil {
			return fmt.Errorf("failed to send signal %d: %s", sig, err)
		}
	}
	return nil
}

func (c *execConfig) stop(timeout int) error {
	p := c.ProcessConfig.Process

	// send a sigterm for now, if failed, then sigkill
	if err := p.Signal(syscall.Signal(15)); err != nil {
		log.Infof("Failed to send SIGTERM to the exec process %v, force killing", p)
		if err := p.Kill(); err != nil {
			return err
		}
	}

	if err := c.waitStop(time.Duration(timeout) * time.Second); err != nil {
		log.Infof("Exec %s failed to exit within %d seconds of SIGTERM - using the force", c.ID, timeout)
		if err := p.Kill(); err != nil {
			return err
		}
		if err := c.waitStop(-1 * time.Second); err != nil {
			return err
		}
	}
	return nil
}

func (d *Daemon) registerExecCommand(execConfig *execConfig) {
	// Storing execs in container inorder to kill them gracefully whenever the container is stopped or removed.
	execConfig.Container.execCommands.Add(execConfig.ID, execConfig)
	// Storing execs in daemon for easy access via remote API.
	d.execCommands.Add(execConfig.ID, execConfig)
}

func (d *Daemon) getExecConfig(name string) (*execConfig, error) {
	if execConfig := d.execCommands.Get(name); execConfig != nil {
		if !execConfig.Container.IsRunning() {
			return nil, fmt.Errorf("Container %s is not running", execConfig.Container.ID)
		}
		return execConfig, nil
	}

	return nil, fmt.Errorf("No such exec instance '%s' found in daemon", name)
}

func (d *Daemon) unregisterExecCommand(execConfig *execConfig) {
	execConfig.Container.execCommands.Delete(execConfig.ID)
	d.execCommands.Delete(execConfig.ID)
}

func (d *Daemon) getActiveContainer(name string) (*Container, error) {
	container := d.Get(name)

	if container == nil {
		return nil, fmt.Errorf("No such container: %s", name)
	}

	if !container.IsRunning() {
		return nil, fmt.Errorf("Container %s is not running", name)
	}

	return container, nil
}

func (d *Daemon) ContainerExecCreate(job *engine.Job) engine.Status {
	if len(job.Args) != 1 {
		return job.Errorf("Usage: %s [options] container command [args]", job.Name)
	}

	if strings.HasPrefix(d.execDriver.Name(), lxc.DriverName) {
		return job.Error(lxc.ErrExec)
	}

	var name = job.Args[0]

	container, err := d.getActiveContainer(name)
	if err != nil {
		return job.Error(err)
	}

	config := runconfig.ExecConfigFromJob(job)

	entrypoint, args := d.getEntrypointAndArgs(nil, config.Cmd)

	processConfig := execdriver.ProcessConfig{
		Privileged: config.Privileged,
		User:       config.User,
		Tty:        config.Tty,
		Entrypoint: entrypoint,
		Arguments:  args,
	}

	execConfig := &execConfig{
		ID:            utils.GenerateRandomID(),
		OpenStdin:     config.AttachStdin,
		OpenStdout:    config.AttachStdout,
		OpenStderr:    config.AttachStderr,
		StreamConfig:  StreamConfig{},
		ProcessConfig: processConfig,
		Container:     container,
		Running:       false,
		waitChan:      make(chan struct{}),
	}

	d.registerExecCommand(execConfig)

	job.Printf("%s\n", execConfig.ID)

	return engine.StatusOK
}

func (d *Daemon) ContainerExecStart(job *engine.Job) engine.Status {
	if len(job.Args) != 1 {
		return job.Errorf("Usage: %s [options] exec", job.Name)
	}

	var (
		cStdin           io.ReadCloser
		cStdout, cStderr io.Writer
		execName         = job.Args[0]
	)

	execConfig, err := d.getExecConfig(execName)
	if err != nil {
		return job.Error(err)
	}

	if execConfig.isRunning() {
		return job.Errorf("Error: Exec command %s is already running", execName)
	}
	execConfig.setRunning()

	log.Debugf("starting exec command %s in container %s", execConfig.ID, execConfig.Container.ID)
	container := execConfig.Container

	if execConfig.OpenStdin {
		r, w := io.Pipe()
		go func() {
			defer w.Close()
			defer log.Debugf("Closing buffered stdin pipe")
			io.Copy(w, job.Stdin)
		}()
		cStdin = r
	}
	if execConfig.OpenStdout {
		cStdout = job.Stdout
	}
	if execConfig.OpenStderr {
		cStderr = job.Stderr
	}

	execConfig.StreamConfig.stderr = broadcastwriter.New()
	execConfig.StreamConfig.stdout = broadcastwriter.New()
	// Attach to stdin
	if execConfig.OpenStdin {
		execConfig.StreamConfig.stdin, execConfig.StreamConfig.stdinPipe = io.Pipe()
	} else {
		execConfig.StreamConfig.stdinPipe = ioutils.NopWriteCloser(ioutil.Discard) // Silently drop stdin
	}

	attachErr := d.attach(&execConfig.StreamConfig, execConfig.OpenStdin, true, execConfig.ProcessConfig.Tty, cStdin, cStdout, cStderr)

	execErr := make(chan error)

	// Note, the execConfig data will be removed when the container
	// itself is deleted.  This allows us to query it (for things like
	// the exitStatus) even after the cmd is done running.

	go func() {
		err := container.Exec(execConfig)
		if err != nil {
			execErr <- fmt.Errorf("Cannot run exec command %s in container %s: %s", execName, container.ID, err)
		}
	}()

	select {
	case err := <-attachErr:
		if err != nil {
			return job.Errorf("attach failed with error: %s", err)
		}
		break
	case err := <-execErr:
		return job.Error(err)
	}

	return engine.StatusOK
}

func (d *Daemon) ContainerExecStop(job *engine.Job) engine.Status {
	if len(job.Args) != 1 {
		return job.Errorf("Usage: %s [options] exec", job.Name)
	}

	var (
		execName = job.Args[0]
		t        int
	)
	if job.EnvExists("t") {
		t = job.GetenvInt("t")
	}
	if t == 0 {
		t = 10 // if t is not set, use 10 seconds as the default
	}

	execConfig, err := d.getExecConfig(execName)
	if err != nil {
		return job.Error(err)
	}

	if !execConfig.isRunning() {
		return job.Errorf("Exec process is stopped")
	}

	log.Debugf("stopping exec command %s in container %s", execConfig.ID, execConfig.Container.ID)

	if err := execConfig.stop(t); err != nil {
		return job.Error(err)
	}

	return engine.StatusOK
}

func (d *Daemon) ContainerExecKill(job *engine.Job) engine.Status {
	if len(job.Args) != 1 {
		return job.Errorf("Usage: %s [options] exec", job.Name)
	}

	var (
		execName = job.Args[0]
		sig      uint64
	)

	// If we have a signal, look at it. Otherwise, do nothing
	if len(job.Args) == 2 && job.Args[1] != "" {
		// Check if we passed the signal as a number:
		// The largest legal signal is 31, so let's parse on 5 bits
		sig, err := strconv.ParseUint(job.Args[1], 10, 5)
		if err != nil {
			// The signal is not a number, treat it as a string (either like "KILL" or like "SIGKILL")
			sig = uint64(signal.SignalMap[strings.TrimPrefix(job.Args[1], "SIG")])
		}

		if sig == 0 {
			return job.Errorf("Invalid signal: %s", job.Args[1])
		}
	}

	execConfig, err := d.getExecConfig(execName)
	if err != nil {
		return job.Error(err)
	}

	if !execConfig.isRunning() {
		return job.Errorf("Exec process is stopped")
	}

	log.Debugf("kill exec command %s in container %s", execConfig.ID, execConfig.Container.ID)

	if err := execConfig.killSig(int(sig)); err != nil {
		return job.Error(err)
	}

	return engine.StatusOK
}

func (d *Daemon) Exec(c *Container, execConfig *execConfig, pipes *execdriver.Pipes, startCallback execdriver.StartCallback) (int, error) {
	exitStatus, err := d.execDriver.Exec(c.command, &execConfig.ProcessConfig, pipes, startCallback)

	// On err, make sure we don't leave ExitCode at zero
	if err != nil && exitStatus == 0 {
		exitStatus = 128
	}

	execConfig.ExitCode = exitStatus
	execConfig.Running = false
	close(execConfig.waitChan) // fire waiters for stop

	return exitStatus, err
}

func (container *Container) Exec(execConfig *execConfig) error {
	container.Lock()
	defer container.Unlock()

	waitStart := make(chan struct{})

	callback := func(processConfig *execdriver.ProcessConfig, pid int) {
		if processConfig.Tty {
			// The callback is called after the process Start()
			// so we are in the parent process. In TTY mode, stdin/out/err is the PtySlave
			// which we close here.
			if c, ok := processConfig.Stdout.(io.Closer); ok {
				c.Close()
			}
		}
		close(waitStart)
	}

	// We use a callback here instead of a goroutine and an chan for
	// syncronization purposes
	cErr := promise.Go(func() error { return container.monitorExec(execConfig, callback) })

	// Exec should not return until the process is actually running
	select {
	case <-waitStart:
	case err := <-cErr:
		return err
	}

	return nil
}

func (container *Container) monitorExec(execConfig *execConfig, callback execdriver.StartCallback) error {
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
