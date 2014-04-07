package foreground

import (
	"fmt"
	"github.com/dotcloud/docker/pkg/sysinfo"
	"github.com/dotcloud/docker/runtime/execdriver"
	"github.com/dotcloud/docker/runtime/execdriver/execdrivers"
	"github.com/dotcloud/docker/utils"
	"io/ioutil"
	"net"
	"net/rpc"
	"os"
	"path"
	"syscall"
)

// This is a copy of execdriver.Command, which unfortunately doesn't can't be marshalled
// since it contains public os.File members that have no exported fields
// This could be done nicer if we change Command to not inherit from exec.Cmd
type CommandWrapper struct {
	// From exec.Cmd:
	Path        string
	Args        []string
	Env         []string
	Dir         string
	SysProcAttr *syscall.SysProcAttr

	// From execdriver.Command:
	ID         string
	Privileged bool
	User       string
	Rootfs     string
	InitPath   string
	Entrypoint string
	Arguments  []string
	WorkingDir string
	ConfigPath string
	Tty        bool
	Network    *execdriver.Network
	Config     map[string][]string
	Resources  *execdriver.Resources
	Mounts     []execdriver.Mount

	// Extra info
	RealDriver        string
	DockerRoot        string
	DockerSysInitPath string
}

func (wrapper *CommandWrapper) Unwrap() *execdriver.Command {
	d := &execdriver.Command{
		// From execdriver.Command:
		ID:         wrapper.ID,
		Privileged: wrapper.Privileged,
		User:       wrapper.User,
		Rootfs:     wrapper.Rootfs,
		InitPath:   wrapper.InitPath,
		Entrypoint: wrapper.Entrypoint,
		Arguments:  wrapper.Arguments,
		WorkingDir: wrapper.WorkingDir,
		ConfigPath: wrapper.ConfigPath,
		Tty:        wrapper.Tty,
		Network:    wrapper.Network,
		Config:     wrapper.Config,
		Resources:  wrapper.Resources,
		Mounts:     wrapper.Mounts,
	}

	// From exec.Cmd:
	d.Path = wrapper.Path
	d.Args = wrapper.Args
	d.Env = wrapper.Env
	d.Dir = wrapper.Dir
	d.SysProcAttr = wrapper.SysProcAttr

	return d
}

func WrapCommand(cmd *execdriver.Command) *CommandWrapper {
	return &CommandWrapper{
		// From exec.Cmd:
		Path:        cmd.Path,
		Args:        cmd.Args,
		Env:         cmd.Env,
		Dir:         cmd.Dir,
		SysProcAttr: cmd.SysProcAttr,

		// From execdriver.Command:
		ID:         cmd.ID,
		Privileged: cmd.Privileged,
		User:       cmd.User,
		Rootfs:     cmd.Rootfs,
		InitPath:   cmd.InitPath,
		Entrypoint: cmd.Entrypoint,
		Arguments:  cmd.Arguments,
		WorkingDir: cmd.WorkingDir,
		ConfigPath: cmd.ConfigPath,
		Tty:        cmd.Tty,
		Network:    cmd.Network,
		Config:     cmd.Config,
		Resources:  cmd.Resources,
		Mounts:     cmd.Mounts,
	}
}

type CmdDriver struct {
	Address    string
	socketDir  string
	stdin      bool
	listener   *net.UnixListener
	realDriver execdriver.Driver
	numClients *Refcount

	cmd         *execdriver.Command
	startedLock chan int
	exitedLock  chan int
	err         error // if exit code is -1
	exitCode    int
}

func NewCmdDriver(stdin bool) (*CmdDriver, error) {
	baseDir := "/var/run/docker-client"
	if err := os.MkdirAll(baseDir, 0600); err != nil {
		return nil, err
	}

	socketDir, err := ioutil.TempDir(baseDir, "cli")
	if err != nil {
		return nil, err
	}

	address := path.Join(socketDir, "socket")
	addr := &net.UnixAddr{Net: "unix", Name: address}
	listener, err := net.ListenUnix("unix", addr)
	if err != nil {
		return nil, err
	}

	d := &CmdDriver{
		Address:     address,
		socketDir:   socketDir,
		stdin:       stdin,
		listener:    listener,
		startedLock: make(chan int),
		exitedLock:  make(chan int),
		numClients:  NewRefcount(),
	}

	if err := rpc.Register(d); err != nil {
		return nil, err
	}

	return d, nil
}

func Serve(d *CmdDriver) error {
	for {
		conn, err := d.listener.AcceptUnix()
		if err != nil {
			utils.Debugf("rpc socket accept error: %s", err)
			continue
		}

		d.numClients.Ref()

		go func() {
			rpc.ServeConn(conn)
			conn.Close()
			d.numClients.Unref()
		}()
	}

	return nil
}

// Can't expose this as a public method, because it does not fit
// the rpc method profile
func WaitForExit(d *CmdDriver) (int, error) {
	exitCode, err := d.waitForExit()

	// Wait for all existing rpc clients to finish
	// so that we allow time for the exit code to
	// be returned from the Wait call
	d.numClients.WaitForZero()

	os.RemoveAll(d.socketDir)

	return exitCode, err
}

func (d *CmdDriver) waitForExit() (int, error) {
	// block on exited
	<-d.exitedLock

	if d.err != nil {
		return -1, d.err
	}

	return d.exitCode, nil
}

func (d *CmdDriver) started() {
	close(d.startedLock)
}

func (d *CmdDriver) exited() {
	close(d.exitedLock)
}

func (d *CmdDriver) Start(wrapper *CommandWrapper, res *int) error {
	if d.realDriver != nil {
		return fmt.Errorf("Can't start container twice\n")
	}

	var err error
	d.realDriver, err = execdrivers.NewDriver(wrapper.RealDriver, wrapper.DockerRoot, wrapper.DockerSysInitPath, sysinfo.New(false))
	if err != nil {
		return err
	}

	cmd := wrapper.Unwrap()

	d.cmd = cmd

	// We want neither a separate session nor a
	// tty that we control in forground mode, so
	// just always set these to false and inherit
	// std* and tty from the caller
	cmd.SysProcAttr.Setctty = false
	cmd.SysProcAttr.Setsid = false
	cmd.Tty = false
	cmd.Console = ""

	// We manually set Stdin here, to avoid SetTerminal overriding it with a pipe
	// as we really want to inherit *the* stdin fd
	if d.stdin {
		cmd.Stdin = os.Stdin
	}
	pipes := execdriver.NewPipes(nil, os.Stdout, os.Stderr, false)

	go func() {
		d.exitCode, d.err = d.realDriver.Run(cmd, pipes, func(*execdriver.Command) {
			d.started()
		})
		d.exited()

		if d.err != nil {
			fmt.Fprintf(os.Stderr, "Can't start container: %s\n", d.err)
		}
	}()

	// block on started or exited (error)
	select {
	case <-d.startedLock:
	case <-d.exitedLock:
	}

	*res = -1
	if cmd.Process != nil {
		*res = cmd.Process.Pid
	}

	return nil
}

func (d *CmdDriver) Wait(_ int, res *int) error {
	exitCode, err := d.waitForExit()
	*res = exitCode
	return err
}

func (d *CmdDriver) Kill(sig int, dummy *int) error {
	if d.cmd == nil {
		return fmt.Errorf("container not started yet")
	}

	return d.realDriver.Kill(d.cmd, sig)
}

func (d *CmdDriver) Terminate(dummy1 int, dummy2 *int) error {
	if d.cmd == nil {
		return fmt.Errorf("container not started yet")
	}

	return d.realDriver.Terminate(d.cmd)
}

func (d *CmdDriver) GetPids(dummy int, res *[]int) error {
	if d.cmd == nil {
		return fmt.Errorf("container not started yet")
	}

	pids, err := d.realDriver.GetPidsForContainer(d.cmd.ID)
	if err != nil {
		return err
	}
	*res = pids
	return nil
}

func (d *CmdDriver) IsRunning(_ int, res *bool) error {
	*res = true

	// block on started or exited (error)
	select {
	case <-d.exitedLock:
		*res = false
	}

	return nil
}
