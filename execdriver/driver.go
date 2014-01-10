package execdriver

import (
	"errors"
	"os/exec"
	"syscall"
	"time"
)

var (
	ErrCommandIsNil = errors.New("Process's cmd is nil")
)

type Driver interface {
	Start(c *Process) error
	Stop(c *Process) error
	Kill(c *Process, sig int) error
	Wait(c *Process, duration time.Duration) error
}

// Network settings of the container
type Network struct {
	Gateway     string
	IPAddress   string
	IPPrefixLen int
	Mtu         int
}

type Process struct {
	exec.Cmd

	Name       string // unique name for the conatienr
	Privileged bool
	User       string
	Rootfs     string // root fs of the container
	InitPath   string // dockerinit
	Entrypoint string
	Arguments  []string
	WorkingDir string
	ConfigPath string
	Tty        bool
	Network    *Network // if network is nil then networking is disabled
}

func (c *Process) Pid() int {
	return c.Process.Pid
}

func (c *Process) GetExitCode() int {
	return c.ProcessState.Sys().(syscall.WaitStatus).ExitStatus()
	return -1
}
