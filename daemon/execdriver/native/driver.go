// +build linux,cgo

package native

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/docker/docker/daemon/execdriver"
	"github.com/docker/docker/daemon/execdriver/native/template"
	"github.com/docker/docker/pkg/parsers"
	"github.com/docker/docker/pkg/pools"
	"github.com/docker/docker/pkg/reexec"
	sysinfo "github.com/docker/docker/pkg/system"
	"github.com/docker/docker/pkg/term"
	"github.com/docker/docker/utils"
	"github.com/opencontainers/runc/libcontainer"
	"github.com/opencontainers/runc/libcontainer/apparmor"
	"github.com/opencontainers/runc/libcontainer/cgroups/systemd"
	"github.com/opencontainers/runc/libcontainer/configs"
	"github.com/opencontainers/runc/libcontainer/system"
	"github.com/opencontainers/runc/libcontainer/utils"
)

// Define constants for native driver
const (
	DriverName = "native"
	Version    = "0.2"
)

// Driver contains all information for native driver,
// it implements execdriver.Driver.
type Driver struct {
	root             string
	activeContainers map[string]libcontainer.Container
	machineMemory    int64
	factory          libcontainer.Factory
	sync.Mutex
}

// NewDriver returns a new native driver, called from NewDriver of execdriver.
func NewDriver(root string, options []string) (*Driver, error) {
	meminfo, err := sysinfo.ReadMemInfo()
	if err != nil {
		return nil, err
	}

	if err := sysinfo.MkdirAll(root, 0700); err != nil {
		return nil, err
	}

	if apparmor.IsEnabled() {
		if err := installAppArmorProfile(); err != nil {
			apparmorProfiles := []string{"docker-default"}

			// Allow daemon to run if loading failed, but are active
			// (possibly through another run, manually, or via system startup)
			for _, policy := range apparmorProfiles {
				if err := hasAppArmorProfileLoaded(policy); err != nil {
					return nil, fmt.Errorf("AppArmor enabled on system but the %s profile could not be loaded.", policy)
				}
			}
		}
	}

	// choose cgroup manager
	// this makes sure there are no breaking changes to people
	// who upgrade from versions without native.cgroupdriver opt
	cgm := libcontainer.Cgroupfs

	// parse the options
	for _, option := range options {
		key, val, err := parsers.ParseKeyValueOpt(option)
		if err != nil {
			return nil, err
		}
		key = strings.ToLower(key)
		switch key {
		case "native.cgroupdriver":
			// override the default if they set options
			switch val {
			case "systemd":
				if systemd.UseSystemd() {
					cgm = libcontainer.SystemdCgroups
					template.SystemdCgroups = true
				} else {
					// warn them that they chose the wrong driver
					logrus.Warn("You cannot use systemd as native.cgroupdriver, using cgroupfs instead")
				}
			case "cgroupfs":
				cgm = libcontainer.Cgroupfs
			default:
				return nil, fmt.Errorf("Unknown native.cgroupdriver given %q. try cgroupfs or systemd", val)
			}
		default:
			return nil, fmt.Errorf("Unknown option %s\n", key)
		}
	}

	f, err := libcontainer.New(
		root,
		cgm,
		libcontainer.InitPath(reexec.Self(), DriverName),
	)
	if err != nil {
		return nil, err
	}

	return &Driver{
		root:             root,
		activeContainers: make(map[string]libcontainer.Container),
		machineMemory:    meminfo.MemTotal,
		factory:          f,
	}, nil
}

type execOutput struct {
	exitCode int
	err      error
}

// Run implements the exec driver Driver interface,
// it calls libcontainer APIs to run a container.
func (d *Driver) Run(c *execdriver.Command, pipes *execdriver.Pipes, hooks execdriver.Hooks) (execdriver.ExitStatus, error) {
	destroyed := false
	var err error
	c.TmpDir, err = ioutil.TempDir("", c.ID)
	if err != nil {
		return execdriver.ExitStatus{ExitCode: -1}, err
	}
	defer os.RemoveAll(c.TmpDir)

	// take the Command and populate the libcontainer.Config from it
	container, err := d.createContainer(c, hooks)
	if err != nil {
		return execdriver.ExitStatus{ExitCode: -1}, err
	}

	p := &libcontainer.Process{
		Args: append([]string{c.ProcessConfig.Entrypoint}, c.ProcessConfig.Arguments...),
		Env:  c.ProcessConfig.Env,
		Cwd:  c.WorkingDir,
		User: c.ProcessConfig.User,
	}

	if err := setupPipes(container, &c.ProcessConfig, p, pipes); err != nil {
		return execdriver.ExitStatus{ExitCode: -1}, err
	}

	cont, err := d.factory.Create(c.ID, container)
	if err != nil {
		return execdriver.ExitStatus{ExitCode: -1}, err
	}
	d.Lock()
	d.activeContainers[c.ID] = cont
	d.Unlock()
	defer func() {
		if !destroyed {
			cont.Destroy()
		}
		d.cleanContainer(c.ID)
	}()

	if err := cont.Start(p); err != nil {
		return execdriver.ExitStatus{ExitCode: -1}, err
	}

	oom := notifyOnOOM(cont)
	if hooks.Start != nil {
		pid, err := p.Pid()
		if err != nil {
			p.Signal(os.Kill)
			p.Wait()
			return execdriver.ExitStatus{ExitCode: -1}, err
		}
		hooks.Start(&c.ProcessConfig, pid, oom)
	}

	waitF := p.Wait
	if nss := cont.Config().Namespaces; !nss.Contains(configs.NEWPID) {
		// we need such hack for tracking processes with inherited fds,
		// because cmd.Wait() waiting for all streams to be copied
		waitF = waitInPIDHost(p, cont)
	}
	ps, err := waitF()
	if err != nil {
		execErr, ok := err.(*exec.ExitError)
		if !ok {
			return execdriver.ExitStatus{ExitCode: -1}, err
		}
		ps = execErr.ProcessState
	}
	cont.Destroy()
	destroyed = true
	_, oomKill := <-oom
	return execdriver.ExitStatus{ExitCode: utils.ExitStatus(ps.Sys().(syscall.WaitStatus)), OOMKilled: oomKill}, nil
}

// notifyOnOOM returns a channel that signals if the container received an OOM notification
// for any process. If it is unable to subscribe to OOM notifications then a closed
// channel is returned as it will be non-blocking and return the correct result when read.
func notifyOnOOM(container libcontainer.Container) <-chan struct{} {
	oom, err := container.NotifyOOM()
	if err != nil {
		logrus.Warnf("Your kernel does not support OOM notifications: %s", err)
		c := make(chan struct{})
		close(c)
		return c
	}
	return oom
}

func killCgroupProcs(c libcontainer.Container) {
	var procs []*os.Process
	if err := c.Pause(); err != nil {
		logrus.Warn(err)
	}
	pids, err := c.Processes()
	if err != nil {
		// don't care about childs if we can't get them, this is mostly because cgroup already deleted
		logrus.Warnf("Failed to get processes from container %s: %v", c.ID(), err)
	}
	for _, pid := range pids {
		if p, err := os.FindProcess(pid); err == nil {
			procs = append(procs, p)
			if err := p.Kill(); err != nil {
				logrus.Warn(err)
			}
		}
	}
	if err := c.Resume(); err != nil {
		logrus.Warn(err)
	}
	for _, p := range procs {
		if _, err := p.Wait(); err != nil {
			logrus.Warn(err)
		}
	}
}

func waitInPIDHost(p *libcontainer.Process, c libcontainer.Container) func() (*os.ProcessState, error) {
	return func() (*os.ProcessState, error) {
		pid, err := p.Pid()
		if err != nil {
			return nil, err
		}

		process, err := os.FindProcess(pid)
		s, err := process.Wait()
		if err != nil {
			execErr, ok := err.(*exec.ExitError)
			if !ok {
				return s, err
			}
			s = execErr.ProcessState
		}
		killCgroupProcs(c)
		p.Wait()
		return s, err
	}
}

// Kill implements the exec driver Driver interface.
func (d *Driver) Kill(c *execdriver.Command, sig int) error {
	d.Lock()
	active := d.activeContainers[c.ID]
	d.Unlock()
	if active == nil {
		return fmt.Errorf("active container for %s does not exist", c.ID)
	}
	state, err := active.State()
	if err != nil {
		return err
	}
	return syscall.Kill(state.InitProcessPid, syscall.Signal(sig))
}

// Pause implements the exec driver Driver interface,
// it calls libcontainer API to pause a container.
func (d *Driver) Pause(c *execdriver.Command) error {
	d.Lock()
	active := d.activeContainers[c.ID]
	d.Unlock()
	if active == nil {
		return fmt.Errorf("active container for %s does not exist", c.ID)
	}
	return active.Pause()
}

// Unpause implements the exec driver Driver interface,
// it calls libcontainer API to unpause a container.
func (d *Driver) Unpause(c *execdriver.Command) error {
	d.Lock()
	active := d.activeContainers[c.ID]
	d.Unlock()
	if active == nil {
		return fmt.Errorf("active container for %s does not exist", c.ID)
	}
	return active.Resume()
}

// XXX Where is the right place for the following
//     const and getCheckpointImageDir() function?
const (
	containersDir = "/var/lib/docker/containers"
	criuImgDir    = "criu_img"
)

func getCheckpointImageDir(containerId string) string {
	return filepath.Join(containersDir, containerId, criuImgDir)
}

func (d *driver) Checkpoint(c *execdriver.Command) error {
	active := d.activeContainers[c.ID]
	if active == nil {
		return fmt.Errorf("active container for %s does not exist", c.ID)
	}
	container := active.container

	// Create an image directory for this container (which
	// may already exist from a previous checkpoint).
	imageDir := getCheckpointImageDir(c.ID)
	err := os.MkdirAll(imageDir, 0700)
	if err != nil && !os.IsExist(err) {
		return err
	}

	// Copy container.json and state.json files to the CRIU
	// image directory for later use during restore.  Do this
	// before checkpointing because after checkpoint the container
	// will exit and these files will be removed.
	log.CRDbg("saving container.json and state.json before calling CRIU in %s", imageDir)
	srcFiles := []string{"container.json", "state.json"}
	for _, f := range srcFiles {
		srcFile := filepath.Join(d.root, c.ID, f)
		dstFile := filepath.Join(imageDir, f)
		if _, err := utils.CopyFile(srcFile, dstFile); err != nil {
			return err
		}
	}

	d.Lock()
	defer d.Unlock()
	err = namespaces.Checkpoint(container, imageDir, c.ProcessConfig.Process.Pid)
	if err != nil {
		return err
	}

	return nil
}

type restoreOutput struct {
	exitCode int
	err      error
}

func (d *driver) Restore(c *execdriver.Command, pipes *execdriver.Pipes, restoreCallback execdriver.RestoreCallback) (int, error) {
	imageDir := getCheckpointImageDir(c.ID)
	container, err := d.createRestoreContainer(c, imageDir)
	if err != nil {
		return 1, err
	}

	var term execdriver.Terminal

	if c.ProcessConfig.Tty {
		term, err = NewTtyConsole(&c.ProcessConfig, pipes)
	} else {
		term, err = execdriver.NewStdConsole(&c.ProcessConfig, pipes)
	}
	if err != nil {
		return -1, err
	}
	c.ProcessConfig.Terminal = term

	d.Lock()
	d.activeContainers[c.ID] = &activeContainer{
		container: container,
		cmd:       &c.ProcessConfig.Cmd,
	}
	d.Unlock()
	defer d.cleanContainer(c.ID)

	// Since the CRIU binary exits after restoring the container, we
	// need to reap its child by setting PR_SET_CHILD_SUBREAPER (36)
	// so that it'll be owned by this process (Docker daemon) after restore.
	//
	// XXX This really belongs to where the Docker daemon starts.
	if _, _, syserr := syscall.RawSyscall(syscall.SYS_PRCTL, 36, 1, 0); syserr != 0 {
		return -1, fmt.Errorf("Could not set PR_SET_CHILD_SUBREAPER (syserr %d)", syserr)
	}

	restoreOutputChan := make(chan restoreOutput, 1)
	waitForRestore := make(chan struct{})

	go func() {
		exitCode, err := namespaces.Restore(container, c.ProcessConfig.Stdin, c.ProcessConfig.Stdout, c.ProcessConfig.Stderr, c.ProcessConfig.Console, filepath.Join(d.root, c.ID), imageDir,
			func(child *os.File, args []string) *exec.Cmd {
				cmd := new(exec.Cmd)
				cmd.Path = d.initPath
				cmd.Args = append([]string{
					DriverName,
					"-restore",
					"-pipe", "3",
					"--",
				}, args...)
				cmd.ExtraFiles = []*os.File{child}
				return cmd
			},
			func(restorePid int) error {
				log.CRDbg("restorePid=%d", restorePid)
				if restorePid == 0 {
					restoreCallback(&c.ProcessConfig, 0)
					return nil
				}

				// The container.json file should be written *after* the container
				// has started because its StdFds cannot be initialized before.
				//
				// XXX How do we handle error here?
				d.writeContainerFile(container, c.ID)
				close(waitForRestore)
				if restoreCallback != nil {
					c.ProcessConfig.Process, err = os.FindProcess(restorePid)
					if err != nil {
						log.Debugf("cannot find restored process %d", restorePid)
						return err
					}
					c.ContainerPid = c.ProcessConfig.Process.Pid
					restoreCallback(&c.ProcessConfig, c.ContainerPid)
				}
				return nil
			})
		restoreOutputChan <- restoreOutput{exitCode, err}
	}()

	select {
	case restoreOutput := <-restoreOutputChan:
		// there was an error
		return restoreOutput.exitCode, restoreOutput.err
	case <-waitForRestore:
		// container restored
		break
	}

	// Wait for the container to exit.
	restoreOutput := <-restoreOutputChan
	return restoreOutput.exitCode, restoreOutput.err
}

// Terminate implements the exec driver Driver interface.
func (d *Driver) Terminate(c *execdriver.Command) error {
	defer d.cleanContainer(c.ID)
	container, err := d.factory.Load(c.ID)
	if err != nil {
		return err
	}
	defer container.Destroy()
	state, err := container.State()
	if err != nil {
		return err
	}
	pid := state.InitProcessPid
	currentStartTime, err := system.GetProcessStartTime(pid)
	if err != nil {
		return err
	}
	if state.InitProcessStartTime == currentStartTime {
		err = syscall.Kill(pid, 9)
		syscall.Wait4(pid, nil, 0, nil)
	}
	return err
}

// Info implements the exec driver Driver interface.
func (d *Driver) Info(id string) execdriver.Info {
	return &info{
		ID:     id,
		driver: d,
	}
}

// Name implements the exec driver Driver interface.
func (d *Driver) Name() string {
	return fmt.Sprintf("%s-%s", DriverName, Version)
}

// GetPidsForContainer implements the exec driver Driver interface.
func (d *Driver) GetPidsForContainer(id string) ([]int, error) {
	d.Lock()
	active := d.activeContainers[id]
	d.Unlock()

	if active == nil {
		return nil, fmt.Errorf("active container for %s does not exist", id)
	}
	return active.Processes()
}

func (d *Driver) cleanContainer(id string) error {
	d.Lock()
	delete(d.activeContainers, id)
	d.Unlock()
	return os.RemoveAll(filepath.Join(d.root, id))
}

func (d *Driver) createContainerRoot(id string) error {
	return os.MkdirAll(filepath.Join(d.root, id), 0655)
}

// Clean implements the exec driver Driver interface.
func (d *Driver) Clean(id string) error {
	return os.RemoveAll(filepath.Join(d.root, id))
}

// Stats implements the exec driver Driver interface.
func (d *Driver) Stats(id string) (*execdriver.ResourceStats, error) {
	d.Lock()
	c := d.activeContainers[id]
	d.Unlock()
	if c == nil {
		return nil, execdriver.ErrNotRunning
	}
	now := time.Now()
	stats, err := c.Stats()
	if err != nil {
		return nil, err
	}
	memoryLimit := c.Config().Cgroups.Memory
	// if the container does not have any memory limit specified set the
	// limit to the machines memory
	if memoryLimit == 0 {
		memoryLimit = d.machineMemory
	}
	return &execdriver.ResourceStats{
		Stats:       stats,
		Read:        now,
		MemoryLimit: memoryLimit,
	}, nil
}

// Update updates configs for a container
func (d *Driver) Update(c *execdriver.Command) error {
	d.Lock()
	cont := d.activeContainers[c.ID]
	d.Unlock()
	if cont == nil {
		return execdriver.ErrNotRunning
	}
	config := cont.Config()
	if err := execdriver.SetupCgroups(&config, c); err != nil {
		return err
	}

	if err := cont.Set(config); err != nil {
		return err
	}

	return nil
}

// TtyConsole implements the exec driver Terminal interface.
type TtyConsole struct {
	console libcontainer.Console
}

// NewTtyConsole returns a new TtyConsole struct.
func NewTtyConsole(console libcontainer.Console, pipes *execdriver.Pipes) (*TtyConsole, error) {
	tty := &TtyConsole{
		console: console,
	}

	if err := tty.AttachPipes(pipes); err != nil {
		tty.Close()
		return nil, err
	}

	return tty, nil
}

// Resize implements Resize method of Terminal interface
func (t *TtyConsole) Resize(h, w int) error {
	return term.SetWinsize(t.console.Fd(), &term.Winsize{Height: uint16(h), Width: uint16(w)})
}

// AttachPipes attaches given pipes to TtyConsole
func (t *TtyConsole) AttachPipes(pipes *execdriver.Pipes) error {
	go func() {
		if wb, ok := pipes.Stdout.(interface {
			CloseWriters() error
		}); ok {
			defer wb.CloseWriters()
		}

		pools.Copy(pipes.Stdout, t.console)
	}()

	if pipes.Stdin != nil {
		go func() {
			pools.Copy(t.console, pipes.Stdin)

			pipes.Stdin.Close()
		}()
	}

	return nil
}

// Close implements Close method of Terminal interface
func (t *TtyConsole) Close() error {
	return t.console.Close()
}

func setupPipes(container *configs.Config, processConfig *execdriver.ProcessConfig, p *libcontainer.Process, pipes *execdriver.Pipes) error {

	rootuid, err := container.HostUID()
	if err != nil {
		return err
	}

	if processConfig.Tty {
		cons, err := p.NewConsole(rootuid)
		if err != nil {
			return err
		}
		term, err := NewTtyConsole(cons, pipes)
		if err != nil {
			return err
		}
		processConfig.Terminal = term
		return nil
	}
	// not a tty--set up stdio pipes
	term := &execdriver.StdConsole{}
	processConfig.Terminal = term

	// if we are not in a user namespace, there is no reason to go through
	// the hassle of setting up os-level pipes with proper (remapped) ownership
	// so we will do the prior shortcut for non-userns containers
	if rootuid == 0 {
		p.Stdout = pipes.Stdout
		p.Stderr = pipes.Stderr

		r, w, err := os.Pipe()
		if err != nil {
			return err
		}
		if pipes.Stdin != nil {
			go func() {
				io.Copy(w, pipes.Stdin)
				w.Close()
			}()
			p.Stdin = r
		}
		return nil
	}

	// if we have user namespaces enabled (rootuid != 0), we will set
	// up os pipes for stderr, stdout, stdin so we can chown them to
	// the proper ownership to allow for proper access to the underlying
	// fds
	var fds []int

	//setup stdout
	r, w, err := os.Pipe()
	if err != nil {
		return err
	}
	fds = append(fds, int(r.Fd()), int(w.Fd()))
	if pipes.Stdout != nil {
		go io.Copy(pipes.Stdout, r)
	}
	term.Closers = append(term.Closers, r)
	p.Stdout = w

	//setup stderr
	r, w, err = os.Pipe()
	if err != nil {
		return err
	}
	fds = append(fds, int(r.Fd()), int(w.Fd()))
	if pipes.Stderr != nil {
		go io.Copy(pipes.Stderr, r)
	}
	term.Closers = append(term.Closers, r)
	p.Stderr = w

	//setup stdin
	r, w, err = os.Pipe()
	if err != nil {
		return err
	}
	fds = append(fds, int(r.Fd()), int(w.Fd()))
	if pipes.Stdin != nil {
		go func() {
			io.Copy(w, pipes.Stdin)
			w.Close()
		}()
		p.Stdin = r
	}
	for _, fd := range fds {
		if err := syscall.Fchown(fd, rootuid, rootuid); err != nil {
			return fmt.Errorf("Failed to chown pipes fd: %v", err)
		}
	}
	return nil
}

// SupportsHooks implements the execdriver Driver interface.
// The libcontainer/runC-based native execdriver does exploit the hook mechanism
func (d *Driver) SupportsHooks() bool {
	return true
}
