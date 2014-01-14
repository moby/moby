package execdriver

import (
	"errors"
	"os/exec"
	"syscall"
	"time"
)

var (
	ErrNotRunning         = errors.New("Process could not be started")
	ErrWaitTimeoutReached = errors.New("Wait timeout reached")
)

type StartCallback func(*Process)

type Driver interface {
	Run(c *Process, startCallback StartCallback) (int, error) // Run executes the process and blocks until the process exits and returns the exit code
	Kill(c *Process, sig int) error
	// TODO: @crosbymichael @creack wait should probably return the exit code
	Wait(id string, duration time.Duration) error // Wait on an out of process...process - lxc ghosts
	Version() string
	Name() string
}

// Network settings of the container
type Network struct {
	Gateway     string `json:"gateway"`
	IPAddress   string `json:"ip"`
	IPPrefixLen int    `json:"ip_prefix_len"`
	Mtu         int    `json:"mtu"`
}

// Process wrapps an os/exec.Cmd to add more metadata
type Process struct {
	exec.Cmd

	ID         string   `json:"id"`
	Privileged bool     `json:"privileged"`
	User       string   `json:"user"`
	Rootfs     string   `json:"rootfs"`   // root fs of the container
	InitPath   string   `json:"initpath"` // dockerinit
	Entrypoint string   `json:"entrypoint"`
	Arguments  []string `json:"arguments"`
	WorkingDir string   `json:"working_dir"`
	ConfigPath string   `json:"config_path"` // This should be able to be removed when the lxc template is moved into the driver
	Tty        bool     `json:"tty"`
	Network    *Network `json:"network"` // if network is nil then networking is disabled
}

// Return the pid of the process
// If the process is nil -1 will be returned
func (c *Process) Pid() int {
	if c.Process == nil {
		return -1
	}
	return c.Process.Pid
}

// Return the exit code of the process
// if the process has not exited -1 will be returned
func (c *Process) GetExitCode() int {
	if c.ProcessState == nil {
		return -1
	}
	return c.ProcessState.Sys().(syscall.WaitStatus).ExitStatus()
}
