package execdriver

import (
	"errors"
	"io"
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
	Name       string // unique name for the conatienr
	Privileged bool
	User       string
	Dir        string // root fs of the container
	InitPath   string // dockerinit
	Entrypoint string
	Args       []string
	//	Environment map[string]string // we don't use this right now because we use an env file
	WorkingDir string
	ConfigPath string
	Tty        bool
	Network    *Network // if network is nil then networking is disabled
	Stdin      io.Reader
	Stdout     io.Writer
	Stderr     io.Writer

	cmd *exec.Cmd
}

func (c *Process) SetCmd(cmd *exec.Cmd) error {
	c.cmd = cmd
	cmd.Stdout = c.Stdout
	cmd.Stderr = c.Stderr
	cmd.Stdin = c.Stdin
	return nil
}

func (c *Process) Pid() int {
	return c.cmd.Process.Pid
}

func (c *Process) StdinPipe() (io.WriteCloser, error) {
	return c.cmd.StdinPipe()
}

func (c *Process) StderrPipe() (io.ReadCloser, error) {
	return c.cmd.StderrPipe()
}

func (c *Process) StdoutPipe() (io.ReadCloser, error) {
	return c.cmd.StdoutPipe()
}

func (c *Process) GetExitCode() int {
	if c.cmd != nil {
		return c.cmd.ProcessState.Sys().(syscall.WaitStatus).ExitStatus()
	}
	return -1
}

func (c *Process) Wait() error {
	if c.cmd != nil {
		return c.cmd.Wait()
	}
	return ErrCommandIsNil
}
