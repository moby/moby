package execdriver

import (
	"os/exec"
	"syscall"
	"time"
)

type StartCallback func(*Process)

type Driver interface {
	Run(c *Process, startCallback StartCallback) (int, error) // Run executes the process and blocks until the process exits and returns the exit code
	Kill(c *Process, sig int) error
	// TODO: @crosbymichael @creack wait should probably return the exit code
	Wait(id string, duration time.Duration) error // Wait on an out of process...process - lxc ghosts
	Version() string
	String() string
}

// Network settings of the container
type Network struct {
	Gateway     string
	IPAddress   string
	IPPrefixLen int
	Mtu         int
}

// Process wrapps an os/exec.Cmd to add more metadata
type Process struct {
	exec.Cmd

	ID          string
	Privileged  bool
	User        string
	Rootfs      string // root fs of the container
	InitPath    string // dockerinit
	Entrypoint  string
	Arguments   []string
	WorkingDir  string
	ConfigPath  string
	Tty         bool
	Network     *Network // if network is nil then networking is disabled
	SysInitPath string
}

func (c *Process) Pid() int {
	return c.Process.Pid
}

func (c *Process) GetExitCode() int {
	if c.ProcessState == nil {
		return -1
	}
	return c.ProcessState.Sys().(syscall.WaitStatus).ExitStatus()
}
