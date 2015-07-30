package daemon

import (
	"fmt"
	"io"
	"io/ioutil"
	"strings"
	"sync"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/docker/docker/daemon/execdriver"
	"github.com/docker/docker/pkg/broadcastwriter"
	"github.com/docker/docker/pkg/ioutils"
	"github.com/docker/docker/pkg/pools"
	"github.com/docker/docker/pkg/stringid"
	"github.com/docker/docker/runconfig"
)

// ExecConfig holds the configurations for execs. The Daemon keeps
// track of both running and finished execs so that they can be
// examined both during and after completion.
type ExecConfig struct {
	sync.Mutex
	ID            string
	Running       bool
	ExitCode      int
	ProcessConfig *execdriver.ProcessConfig
	streamConfig
	OpenStdin  bool
	OpenStderr bool
	OpenStdout bool
	Container  *Container
	canRemove  bool

	// waitStart will be closed immediately after the exec is really started.
	waitStart chan struct{}
}

type execStore struct {
	s map[string]*ExecConfig
	sync.RWMutex
}

func newExecStore() *execStore {
	return &execStore{s: make(map[string]*ExecConfig, 0)}
}

func (e *execStore) Add(id string, ExecConfig *ExecConfig) {
	e.Lock()
	e.s[id] = ExecConfig
	e.Unlock()
}

func (e *execStore) Get(id string) *ExecConfig {
	e.RLock()
	res := e.s[id]
	e.RUnlock()
	return res
}

func (e *execStore) Delete(id string) {
	e.Lock()
	delete(e.s, id)
	e.Unlock()
}

func (e *execStore) List() []string {
	var IDs []string
	e.RLock()
	for id := range e.s {
		IDs = append(IDs, id)
	}
	e.RUnlock()
	return IDs
}

func (ExecConfig *ExecConfig) resize(h, w int) error {
	select {
	case <-ExecConfig.waitStart:
	case <-time.After(time.Second):
		return fmt.Errorf("Exec %s is not running, so it can not be resized.", ExecConfig.ID)
	}
	return ExecConfig.ProcessConfig.Terminal.Resize(h, w)
}

func (d *Daemon) registerExecCommand(ExecConfig *ExecConfig) {
	// Storing execs in container in order to kill them gracefully whenever the container is stopped or removed.
	ExecConfig.Container.execCommands.Add(ExecConfig.ID, ExecConfig)
	// Storing execs in daemon for easy access via remote API.
	d.execCommands.Add(ExecConfig.ID, ExecConfig)
}

func (d *Daemon) getExecConfig(name string) (*ExecConfig, error) {
	ExecConfig := d.execCommands.Get(name)

	// If the exec is found but its container is not in the daemon's list of
	// containers then it must have been delete, in which case instead of
	// saying the container isn't running, we should return a 404 so that
	// the user sees the same error now that they will after the
	// 5 minute clean-up loop is run which erases old/dead execs.

	if ExecConfig != nil && d.containers.Get(ExecConfig.Container.ID) != nil {

		if !ExecConfig.Container.IsRunning() {
			return nil, fmt.Errorf("Container %s is not running", ExecConfig.Container.ID)
		}
		return ExecConfig, nil
	}

	return nil, fmt.Errorf("No such exec instance '%s' found in daemon", name)
}

func (d *Daemon) unregisterExecCommand(ExecConfig *ExecConfig) {
	ExecConfig.Container.execCommands.Delete(ExecConfig.ID)
	d.execCommands.Delete(ExecConfig.ID)
}

func (d *Daemon) getActiveContainer(name string) (*Container, error) {
	container, err := d.Get(name)
	if err != nil {
		return nil, err
	}

	if !container.IsRunning() {
		return nil, fmt.Errorf("Container %s is not running", name)
	}
	if container.isPaused() {
		return nil, fmt.Errorf("Container %s is paused, unpause the container before exec", name)
	}
	return container, nil
}

// ContainerExecCreate sets up an exec in a running container.
func (d *Daemon) ContainerExecCreate(config *runconfig.ExecConfig) (string, error) {
	// Not all drivers support Exec (LXC for example)
	if err := checkExecSupport(d.execDriver.Name()); err != nil {
		return "", err
	}

	container, err := d.getActiveContainer(config.Container)
	if err != nil {
		return "", err
	}

	cmd := runconfig.NewCommand(config.Cmd...)
	entrypoint, args := d.getEntrypointAndArgs(runconfig.NewEntrypoint(), cmd)

	user := config.User
	if len(user) == 0 {
		user = container.Config.User
	}

	processConfig := &execdriver.ProcessConfig{
		Tty:        config.Tty,
		Entrypoint: entrypoint,
		Arguments:  args,
		User:       user,
		Privileged: config.Privileged,
	}

	ExecConfig := &ExecConfig{
		ID:            stringid.GenerateNonCryptoID(),
		OpenStdin:     config.AttachStdin,
		OpenStdout:    config.AttachStdout,
		OpenStderr:    config.AttachStderr,
		streamConfig:  streamConfig{},
		ProcessConfig: processConfig,
		Container:     container,
		Running:       false,
		waitStart:     make(chan struct{}),
	}

	d.registerExecCommand(ExecConfig)

	container.logEvent("exec_create: " + ExecConfig.ProcessConfig.Entrypoint + " " + strings.Join(ExecConfig.ProcessConfig.Arguments, " "))

	return ExecConfig.ID, nil
}

// ContainerExecStart starts a previously set up exec instance. The
// std streams are set up.
func (d *Daemon) ContainerExecStart(execName string, stdin io.ReadCloser, stdout io.Writer, stderr io.Writer) error {
	var (
		cStdin           io.ReadCloser
		cStdout, cStderr io.Writer
	)

	ExecConfig, err := d.getExecConfig(execName)
	if err != nil {
		return err
	}

	func() {
		ExecConfig.Lock()
		defer ExecConfig.Unlock()
		if ExecConfig.Running {
			err = fmt.Errorf("Error: Exec command %s is already running", execName)
		}
		ExecConfig.Running = true
	}()
	if err != nil {
		return err
	}

	logrus.Debugf("starting exec command %s in container %s", ExecConfig.ID, ExecConfig.Container.ID)
	container := ExecConfig.Container

	container.logEvent("exec_start: " + ExecConfig.ProcessConfig.Entrypoint + " " + strings.Join(ExecConfig.ProcessConfig.Arguments, " "))

	if ExecConfig.OpenStdin {
		r, w := io.Pipe()
		go func() {
			defer w.Close()
			defer logrus.Debugf("Closing buffered stdin pipe")
			pools.Copy(w, stdin)
		}()
		cStdin = r
	}
	if ExecConfig.OpenStdout {
		cStdout = stdout
	}
	if ExecConfig.OpenStderr {
		cStderr = stderr
	}

	ExecConfig.streamConfig.stderr = broadcastwriter.New()
	ExecConfig.streamConfig.stdout = broadcastwriter.New()
	// Attach to stdin
	if ExecConfig.OpenStdin {
		ExecConfig.streamConfig.stdin, ExecConfig.streamConfig.stdinPipe = io.Pipe()
	} else {
		ExecConfig.streamConfig.stdinPipe = ioutils.NopWriteCloser(ioutil.Discard) // Silently drop stdin
	}

	attachErr := attach(&ExecConfig.streamConfig, ExecConfig.OpenStdin, true, ExecConfig.ProcessConfig.Tty, cStdin, cStdout, cStderr)

	execErr := make(chan error)

	// Note, the ExecConfig data will be removed when the container
	// itself is deleted.  This allows us to query it (for things like
	// the exitStatus) even after the cmd is done running.

	go func() {
		if err := container.exec(ExecConfig); err != nil {
			execErr <- fmt.Errorf("Cannot run exec command %s in container %s: %s", execName, container.ID, err)
		}
	}()
	select {
	case err := <-attachErr:
		if err != nil {
			return fmt.Errorf("attach failed with error: %s", err)
		}
		return nil
	case err := <-execErr:
		if err == nil {
			return nil
		}

		// Maybe the container stopped while we were trying to exec
		if !container.IsRunning() {
			return fmt.Errorf("container stopped while running exec")
		}
		return err
	}
}

// Exec calls the underlying exec driver to run
func (d *Daemon) Exec(c *Container, ExecConfig *ExecConfig, pipes *execdriver.Pipes, startCallback execdriver.StartCallback) (int, error) {
	exitStatus, err := d.execDriver.Exec(c.command, ExecConfig.ProcessConfig, pipes, startCallback)

	// On err, make sure we don't leave ExitCode at zero
	if err != nil && exitStatus == 0 {
		exitStatus = 128
	}

	ExecConfig.ExitCode = exitStatus
	ExecConfig.Running = false

	return exitStatus, err
}

// execCommandGC runs a ticker to clean up the daemon references
// of exec configs that are no longer part of the container.
func (d *Daemon) execCommandGC() {
	for range time.Tick(5 * time.Minute) {
		var (
			cleaned          int
			liveExecCommands = d.containerExecIds()
		)
		for id, config := range d.execCommands.s {
			if config.canRemove {
				cleaned++
				d.execCommands.Delete(id)
			} else {
				if _, exists := liveExecCommands[id]; !exists {
					config.canRemove = true
				}
			}
		}
		if cleaned > 0 {
			logrus.Debugf("clean %d unused exec commands", cleaned)
		}
	}
}

// containerExecIds returns a list of all the current exec ids that are in use
// and running inside a container.
func (d *Daemon) containerExecIds() map[string]struct{} {
	ids := map[string]struct{}{}
	for _, c := range d.containers.List() {
		for _, id := range c.execCommands.List() {
			ids[id] = struct{}{}
		}
	}
	return ids
}
