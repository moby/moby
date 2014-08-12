package execdriver

import (
	"errors"
	"io"
	"os"
	"os/exec"

	"github.com/docker/libcontainer/devices"
)

// Context is a generic key value pair that allows
// arbatrary data to be sent
type Context map[string]string

var (
	ErrNotRunning              = errors.New("Process could not be started")
	ErrWaitTimeoutReached      = errors.New("Wait timeout reached")
	ErrDriverAlreadyRegistered = errors.New("A driver already registered this docker init function")
	ErrDriverNotFound          = errors.New("The requested docker init has not been found")
)

type StartCallback func(*Command)

// Driver specific information based on
// processes registered with the driver
type Info interface {
	IsRunning() bool
}

// Terminal in an interface for drivers to implement
// if they want to support Close and Resize calls from
// the core
type Terminal interface {
	io.Closer
	Resize(height, width int) error
}

type TtyTerminal interface {
	Master() *os.File
}

type Driver interface {
	Run(c *Command, pipes *Pipes, startCallback StartCallback) (int, error) // Run executes the process and blocks until the process exits and returns the exit code
	Kill(c *Command, sig int) error
	Pause(c *Command) error
	Unpause(c *Command) error
	Name() string                                 // Driver name
	Info(id string) Info                          // "temporary" hack (until we move state from core to plugins)
	GetPidsForContainer(id string) ([]int, error) // Returns a list of pids for the given container.
	Terminate(c *Command) error                   // kill it with fire
}

// Network settings of the container
type Network struct {
	Interface      *NetworkInterface `json:"interface"` // if interface is nil then networking is disabled
	Mtu            int               `json:"mtu"`
	ContainerID    string            `json:"container_id"` // id of the container to join network.
	HostNetworking bool              `json:"host_networking"`
}

type NetworkInterface struct {
	Gateway     string `json:"gateway"`
	IPAddress   string `json:"ip"`
	Bridge      string `json:"bridge"`
	IPPrefixLen int    `json:"ip_prefix_len"`
}

type Resources struct {
	Memory     int64  `json:"memory"`
	MemorySwap int64  `json:"memory_swap"`
	CpuShares  int64  `json:"cpu_shares"`
	Cpuset     string `json:"cpuset"`
}

type Mount struct {
	Source      string `json:"source"`
	Destination string `json:"destination"`
	Writable    bool   `json:"writable"`
	Private     bool   `json:"private"`
}

// Process wrapps an os/exec.Cmd to add more metadata
type Command struct {
	exec.Cmd `json:"-"`

	ID                 string              `json:"id"`
	Privileged         bool                `json:"privileged"`
	User               string              `json:"user"`
	Rootfs             string              `json:"rootfs"`   // root fs of the container
	InitPath           string              `json:"initpath"` // dockerinit
	Entrypoint         string              `json:"entrypoint"`
	Arguments          []string            `json:"arguments"`
	WorkingDir         string              `json:"working_dir"`
	ConfigPath         string              `json:"config_path"` // this should be able to be removed when the lxc template is moved into the driver
	Tty                bool                `json:"tty"`
	Network            *Network            `json:"network"`
	Config             map[string][]string `json:"config"` //  generic values that specific drivers can consume
	Resources          *Resources          `json:"resources"`
	Mounts             []Mount             `json:"mounts"`
	AllowedDevices     []*devices.Device   `json:"allowed_devices"`
	AutoCreatedDevices []*devices.Device   `json:"autocreated_devices"`
	CapAdd             []string            `json:"cap_add"`
	CapDrop            []string            `json:"cap_drop"`

	Terminal     Terminal `json:"-"`             // standard or tty terminal
	Console      string   `json:"-"`             // dev/console path
	ContainerPid int      `json:"container_pid"` // the pid for the process inside a container
}

// Return the pid of the process
// If the process is nil -1 will be returned
func (c *Command) Pid() int {
	return c.ContainerPid
}
