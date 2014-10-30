// +build linux,cgo

package native

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"

	log "github.com/Sirupsen/logrus"
	"github.com/docker/docker/daemon/execdriver"
	"github.com/docker/docker/pkg/term"
	"github.com/docker/libcontainer"
	"github.com/docker/libcontainer/apparmor"
	"github.com/docker/libcontainer/cgroups/fs"
	"github.com/docker/libcontainer/cgroups/systemd"
	consolepkg "github.com/docker/libcontainer/console"
	"github.com/docker/libcontainer/namespaces"
	_ "github.com/docker/libcontainer/namespaces/nsenter"
	"github.com/docker/libcontainer/system"
)

const (
	DriverName = "native"
	Version    = "0.2"
)

type activeContainer struct {
	container *libcontainer.Config
	cmd       *exec.Cmd
}

type driver struct {
	root             string
	initPath         string
	activeContainers map[string]*activeContainer
	sync.Mutex
}

func NewDriver(root, initPath string) (*driver, error) {
	if err := os.MkdirAll(root, 0700); err != nil {
		return nil, err
	}

	// native driver root is at docker_root/execdriver/native. Put apparmor at docker_root
	if err := apparmor.InstallDefaultProfile(); err != nil {
		return nil, err
	}

	return &driver{
		root:             root,
		initPath:         initPath,
		activeContainers: make(map[string]*activeContainer),
	}, nil
}

func (d *driver) notifyOnOOM(config *libcontainer.Config) (<-chan struct{}, error) {
	return fs.NotifyOnOOM(config.Cgroups)
}

type execOutput struct {
	exitCode int
	err      error
}

func (d *driver) Run(c *execdriver.Command, pipes *execdriver.Pipes, startCallback execdriver.StartCallback) (execdriver.ExitStatus, error) {
	// take the Command and populate the libcontainer.Config from it
	container, err := d.createContainer(c)
	if err != nil {
		return execdriver.ExitStatus{-1, false}, err
	}

	var term execdriver.Terminal

	if c.ProcessConfig.Tty {
		term, err = NewTtyConsole(&c.ProcessConfig, pipes)
	} else {
		term, err = execdriver.NewStdConsole(&c.ProcessConfig, pipes)
	}
	if err != nil {
		return execdriver.ExitStatus{-1, false}, err
	}
	c.ProcessConfig.Terminal = term

	d.Lock()
	d.activeContainers[c.ID] = &activeContainer{
		container: container,
		cmd:       &c.ProcessConfig.Cmd,
	}
	d.Unlock()

	var (
		dataPath = filepath.Join(d.root, c.ID)
		args     = append([]string{c.ProcessConfig.Entrypoint}, c.ProcessConfig.Arguments...)
	)

	if err := d.createContainerRoot(c.ID); err != nil {
		return execdriver.ExitStatus{-1, false}, err
	}
	defer d.cleanContainer(c.ID)

	if err := d.writeContainerFile(container, c.ID); err != nil {
		return execdriver.ExitStatus{-1, false}, err
	}

	execOutputChan := make(chan execOutput, 1)
	waitForStart := make(chan struct{})

	go func() {
		exitCode, err := namespaces.Exec(container, c.ProcessConfig.Stdin, c.ProcessConfig.Stdout, c.ProcessConfig.Stderr, c.ProcessConfig.Console, dataPath, args, func(container *libcontainer.Config, console, dataPath, init string, child *os.File, args []string) *exec.Cmd {
			c.ProcessConfig.Path = d.initPath
			c.ProcessConfig.Args = append([]string{
				DriverName,
				"-console", console,
				"-pipe", "3",
				"-root", filepath.Join(d.root, c.ID),
				"--",
			}, args...)

			// set this to nil so that when we set the clone flags anything else is reset
			c.ProcessConfig.SysProcAttr = &syscall.SysProcAttr{
				Cloneflags: uintptr(namespaces.GetNamespaceFlags(container.Namespaces)),
			}
			c.ProcessConfig.ExtraFiles = []*os.File{child}

			c.ProcessConfig.Env = container.Env
			c.ProcessConfig.Dir = container.RootFs

			return &c.ProcessConfig.Cmd
		}, func() {
			close(waitForStart)
			if startCallback != nil {
				c.ContainerPid = c.ProcessConfig.Process.Pid
				startCallback(&c.ProcessConfig, c.ContainerPid)
			}
		})
		execOutputChan <- execOutput{exitCode, err}
	}()

	select {
	case execOutput := <-execOutputChan:
		return execdriver.ExitStatus{execOutput.exitCode, false}, execOutput.err
	case <-waitForStart:
		break
	}

	oomKill := false
	oomKillNotification, err := d.notifyOnOOM(container)
	if err == nil {
		_, oomKill = <-oomKillNotification
	} else {
		log.Warnf("WARNING: Your kernel does not support OOM notifications: %s", err)
	}
	// wait for the container to exit.
	execOutput := <-execOutputChan

	return execdriver.ExitStatus{execOutput.exitCode, oomKill}, execOutput.err
}

func (d *driver) Kill(p *execdriver.Command, sig int) error {
	return syscall.Kill(p.ProcessConfig.Process.Pid, syscall.Signal(sig))
}

func (d *driver) Pause(c *execdriver.Command) error {
	active := d.activeContainers[c.ID]
	if active == nil {
		return fmt.Errorf("active container for %s does not exist", c.ID)
	}
	active.container.Cgroups.Freezer = "FROZEN"
	if systemd.UseSystemd() {
		return systemd.Freeze(active.container.Cgroups, active.container.Cgroups.Freezer)
	}
	return fs.Freeze(active.container.Cgroups, active.container.Cgroups.Freezer)
}

func (d *driver) Unpause(c *execdriver.Command) error {
	active := d.activeContainers[c.ID]
	if active == nil {
		return fmt.Errorf("active container for %s does not exist", c.ID)
	}
	active.container.Cgroups.Freezer = "THAWED"
	if systemd.UseSystemd() {
		return systemd.Freeze(active.container.Cgroups, active.container.Cgroups.Freezer)
	}
	return fs.Freeze(active.container.Cgroups, active.container.Cgroups.Freezer)
}

func (d *driver) Terminate(p *execdriver.Command) error {
	// lets check the start time for the process
	state, err := libcontainer.GetState(filepath.Join(d.root, p.ID))
	if err != nil {
		if !os.IsNotExist(err) {
			return err
		}
		// TODO: Remove this part for version 1.2.0
		// This is added only to ensure smooth upgrades from pre 1.1.0 to 1.1.0
		data, err := ioutil.ReadFile(filepath.Join(d.root, p.ID, "start"))
		if err != nil {
			// if we don't have the data on disk then we can assume the process is gone
			// because this is only removed after we know the process has stopped
			if os.IsNotExist(err) {
				return nil
			}
			return err
		}
		state = &libcontainer.State{InitStartTime: string(data)}
	}

	currentStartTime, err := system.GetProcessStartTime(p.ProcessConfig.Process.Pid)
	if err != nil {
		return err
	}

	if state.InitStartTime == currentStartTime {
		err = syscall.Kill(p.ProcessConfig.Process.Pid, 9)
		syscall.Wait4(p.ProcessConfig.Process.Pid, nil, 0, nil)
	}
	d.cleanContainer(p.ID)

	return err

}

func (d *driver) Info(id string) execdriver.Info {
	return &info{
		ID:     id,
		driver: d,
	}
}

func (d *driver) Name() string {
	return fmt.Sprintf("%s-%s", DriverName, Version)
}

func (d *driver) GetPidsForContainer(id string) ([]int, error) {
	d.Lock()
	active := d.activeContainers[id]
	d.Unlock()

	if active == nil {
		return nil, fmt.Errorf("active container for %s does not exist", id)
	}
	c := active.container.Cgroups

	if systemd.UseSystemd() {
		return systemd.GetPids(c)
	}
	return fs.GetPids(c)
}

func (d *driver) writeContainerFile(container *libcontainer.Config, id string) error {
	data, err := json.Marshal(container)
	if err != nil {
		return err
	}
	return ioutil.WriteFile(filepath.Join(d.root, id, "container.json"), data, 0655)
}

func (d *driver) cleanContainer(id string) error {
	d.Lock()
	delete(d.activeContainers, id)
	d.Unlock()
	return os.RemoveAll(filepath.Join(d.root, id, "container.json"))
}

func (d *driver) createContainerRoot(id string) error {
	return os.MkdirAll(filepath.Join(d.root, id), 0655)
}

func (d *driver) Clean(id string) error {
	return os.RemoveAll(filepath.Join(d.root, id))
}

func getEnv(key string, env []string) string {
	for _, pair := range env {
		parts := strings.Split(pair, "=")
		if parts[0] == key {
			return parts[1]
		}
	}
	return ""
}

type TtyConsole struct {
	MasterPty *os.File
}

func NewTtyConsole(processConfig *execdriver.ProcessConfig, pipes *execdriver.Pipes) (*TtyConsole, error) {
	ptyMaster, console, err := consolepkg.CreateMasterAndConsole()
	if err != nil {
		return nil, err
	}

	tty := &TtyConsole{
		MasterPty: ptyMaster,
	}

	if err := tty.AttachPipes(&processConfig.Cmd, pipes); err != nil {
		tty.Close()
		return nil, err
	}

	processConfig.Console = console

	return tty, nil
}

func (t *TtyConsole) Master() *os.File {
	return t.MasterPty
}

func (t *TtyConsole) Resize(h, w int) error {
	return term.SetWinsize(t.MasterPty.Fd(), &term.Winsize{Height: uint16(h), Width: uint16(w)})
}

func (t *TtyConsole) AttachPipes(command *exec.Cmd, pipes *execdriver.Pipes) error {
	go func() {
		if wb, ok := pipes.Stdout.(interface {
			CloseWriters() error
		}); ok {
			defer wb.CloseWriters()
		}

		io.Copy(pipes.Stdout, t.MasterPty)
	}()

	if pipes.Stdin != nil {
		go func() {
			io.Copy(t.MasterPty, pipes.Stdin)

			pipes.Stdin.Close()
		}()
	}

	return nil
}

func (t *TtyConsole) Close() error {
	return t.MasterPty.Close()
}
