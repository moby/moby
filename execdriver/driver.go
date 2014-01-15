package execdriver

import (
	"errors"
	"os/exec"
	"syscall"
)

var (
	ErrNotRunning              = errors.New("Process could not be started")
	ErrWaitTimeoutReached      = errors.New("Wait timeout reached")
	ErrDriverAlreadyRegistered = errors.New("A driver already registered this docker init function")
	ErrDriverNotFound          = errors.New("The requested docker init has not been found")
)

var dockerInitFcts map[string]InitFunc

type (
	StartCallback func(*Process)
	InitFunc      func(i *InitArgs) error
)

func RegisterInitFunc(name string, fct InitFunc) error {
	if dockerInitFcts == nil {
		dockerInitFcts = make(map[string]InitFunc)
	}
	if _, ok := dockerInitFcts[name]; ok {
		return ErrDriverAlreadyRegistered
	}
	dockerInitFcts[name] = fct
	return nil
}

func GetInitFunc(name string) (InitFunc, error) {
	fct, ok := dockerInitFcts[name]
	if !ok {
		return nil, ErrDriverNotFound
	}
	return fct, nil
}

// Args provided to the init function for a driver
type InitArgs struct {
	User       string
	Gateway    string
	Ip         string
	WorkDir    string
	Privileged bool
	Env        []string
	Args       []string
	Mtu        int
	Driver     string
}

// Driver specific information based on
// processes registered with the driver
type Info interface {
	IsRunning() bool
}

type Driver interface {
	Run(c *Process, startCallback StartCallback) (int, error) // Run executes the process and blocks until the process exits and returns the exit code
	Kill(c *Process, sig int) error
	Wait(id string) error // Wait on an out of process...process - lxc ghosts
	Version() string      // Driver version number
	Name() string         // Driver name
	Info(id string) Info  // "temporary" hack (until we move state from core to plugins)
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
