package execdriver

import (
	"encoding/json"
	"errors"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/docker/docker/daemon/execdriver/native/template"
	"github.com/docker/docker/pkg/ulimit"
	"github.com/docker/libcontainer"
	"github.com/docker/libcontainer/cgroups/fs"
	"github.com/docker/libcontainer/configs"
)

// Context is a generic key value pair that allows
// arbatrary data to be sent
type Context map[string]string

var (
	ErrNotRunning              = errors.New("Container is not running")
	ErrWaitTimeoutReached      = errors.New("Wait timeout reached")
	ErrDriverAlreadyRegistered = errors.New("A driver already registered this docker init function")
	ErrDriverNotFound          = errors.New("The requested docker init has not been found")
)

type StartCallback func(*ProcessConfig, int)

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
	Master() libcontainer.Console
}

// ExitStatus provides exit reasons for a container.
type ExitStatus struct {
	// The exit code with which the container exited.
	ExitCode int

	// Whether the container encountered an OOM.
	OOMKilled bool
}

type Driver interface {
	Run(c *Command, pipes *Pipes, startCallback StartCallback) (ExitStatus, error) // Run executes the process and blocks until the process exits and returns the exit code
	// Exec executes the process in an existing container, blocks until the process exits and returns the exit code
	Exec(c *Command, processConfig *ProcessConfig, pipes *Pipes, startCallback StartCallback) (int, error)
	Kill(c *Command, sig int) error
	Pause(c *Command) error
	Unpause(c *Command) error
	Name() string                                 // Driver name
	Info(id string) Info                          // "temporary" hack (until we move state from core to plugins)
	GetPidsForContainer(id string) ([]int, error) // Returns a list of pids for the given container.
	Terminate(c *Command) error                   // kill it with fire
	Clean(id string) error                        // clean all traces of container exec
	Stats(id string) (*ResourceStats, error)      // Get resource stats for a running container
}

// Network settings of the container
type Network struct {
	Interface      *NetworkInterface `json:"interface"` // if interface is nil then networking is disabled
	Mtu            int               `json:"mtu"`
	ContainerID    string            `json:"container_id"` // id of the container to join network.
	HostNetworking bool              `json:"host_networking"`
}

// IPC settings of the container
type Ipc struct {
	ContainerID string `json:"container_id"` // id of the container to join ipc.
	HostIpc     bool   `json:"host_ipc"`
}

// PID settings of the container
type Pid struct {
	HostPid bool `json:"host_pid"`
}

type NetworkInterface struct {
	Gateway              string `json:"gateway"`
	IPAddress            string `json:"ip"`
	IPPrefixLen          int    `json:"ip_prefix_len"`
	MacAddress           string `json:"mac"`
	Bridge               string `json:"bridge"`
	GlobalIPv6Address    string `json:"global_ipv6"`
	LinkLocalIPv6Address string `json:"link_local_ipv6"`
	GlobalIPv6PrefixLen  int    `json:"global_ipv6_prefix_len"`
	IPv6Gateway          string `json:"ipv6_gateway"`
}

type Resources struct {
	Memory     int64            `json:"memory"`
	MemorySwap int64            `json:"memory_swap"`
	CpuShares  int64            `json:"cpu_shares"`
	CpusetCpus string           `json:"cpuset_cpus"`
	CpusetMems string           `json:"cpuset_mems"`
	CpuQuota   int64            `json:"cpu_quota"`
	Rlimits    []*ulimit.Rlimit `json:"rlimits"`
}

type ResourceStats struct {
	*libcontainer.Stats
	Read        time.Time `json:"read"`
	MemoryLimit int64     `json:"memory_limit"`
	SystemUsage uint64    `json:"system_usage"`
}

type Mount struct {
	Source      string `json:"source"`
	Destination string `json:"destination"`
	Writable    bool   `json:"writable"`
	Private     bool   `json:"private"`
	Slave       bool   `json:"slave"`
}

// Describes a process that will be run inside a container.
type ProcessConfig struct {
	exec.Cmd `json:"-"`

	Privileged bool     `json:"privileged"`
	User       string   `json:"user"`
	Tty        bool     `json:"tty"`
	Entrypoint string   `json:"entrypoint"`
	Arguments  []string `json:"arguments"`
	Terminal   Terminal `json:"-"` // standard or tty terminal
	Console    string   `json:"-"` // dev/console path
}

// Process wrapps an os/exec.Cmd to add more metadata
type Command struct {
	ID                 string            `json:"id"`
	Rootfs             string            `json:"rootfs"` // root fs of the container
	ReadonlyRootfs     bool              `json:"readonly_rootfs"`
	InitPath           string            `json:"initpath"` // dockerinit
	WorkingDir         string            `json:"working_dir"`
	ConfigPath         string            `json:"config_path"` // this should be able to be removed when the lxc template is moved into the driver
	Network            *Network          `json:"network"`
	Ipc                *Ipc              `json:"ipc"`
	Pid                *Pid              `json:"pid"`
	Resources          *Resources        `json:"resources"`
	Mounts             []Mount           `json:"mounts"`
	AllowedDevices     []*configs.Device `json:"allowed_devices"`
	AutoCreatedDevices []*configs.Device `json:"autocreated_devices"`
	CapAdd             []string          `json:"cap_add"`
	CapDrop            []string          `json:"cap_drop"`
	ContainerPid       int               `json:"container_pid"`  // the pid for the process inside a container
	ProcessConfig      ProcessConfig     `json:"process_config"` // Describes the init process of the container.
	ProcessLabel       string            `json:"process_label"`
	MountLabel         string            `json:"mount_label"`
	LxcConfig          []string          `json:"lxc_config"`
	AppArmorProfile    string            `json:"apparmor_profile"`
	CgroupParent       string            `json:"cgroup_parent"` // The parent cgroup for this command.
}

func InitContainer(c *Command) *configs.Config {
	container := template.New()

	container.Hostname = getEnv("HOSTNAME", c.ProcessConfig.Env)
	container.Cgroups.Name = c.ID
	container.Cgroups.AllowedDevices = c.AllowedDevices
	container.Devices = c.AutoCreatedDevices
	container.Rootfs = c.Rootfs
	container.Readonlyfs = c.ReadonlyRootfs

	// check to see if we are running in ramdisk to disable pivot root
	container.NoPivotRoot = os.Getenv("DOCKER_RAMDISK") != ""

	// Default parent cgroup is "docker". Override if required.
	if c.CgroupParent != "" {
		container.Cgroups.Parent = c.CgroupParent
	}
	return container
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

func SetupCgroups(container *configs.Config, c *Command) error {
	if c.Resources != nil {
		container.Cgroups.CpuShares = c.Resources.CpuShares
		container.Cgroups.Memory = c.Resources.Memory
		container.Cgroups.MemoryReservation = c.Resources.Memory
		container.Cgroups.MemorySwap = c.Resources.MemorySwap
		container.Cgroups.CpusetCpus = c.Resources.CpusetCpus
		container.Cgroups.CpusetMems = c.Resources.CpusetMems
		container.Cgroups.CpuQuota = c.Resources.CpuQuota
	}

	return nil
}

// Returns the network statistics for the network interfaces represented by the NetworkRuntimeInfo.
func getNetworkInterfaceStats(interfaceName string) (*libcontainer.NetworkInterface, error) {
	out := &libcontainer.NetworkInterface{Name: interfaceName}
	// This can happen if the network runtime information is missing - possible if the
	// container was created by an old version of libcontainer.
	if interfaceName == "" {
		return out, nil
	}
	type netStatsPair struct {
		// Where to write the output.
		Out *uint64
		// The network stats file to read.
		File string
	}
	// Ingress for host veth is from the container. Hence tx_bytes stat on the host veth is actually number of bytes received by the container.
	netStats := []netStatsPair{
		{Out: &out.RxBytes, File: "tx_bytes"},
		{Out: &out.RxPackets, File: "tx_packets"},
		{Out: &out.RxErrors, File: "tx_errors"},
		{Out: &out.RxDropped, File: "tx_dropped"},

		{Out: &out.TxBytes, File: "rx_bytes"},
		{Out: &out.TxPackets, File: "rx_packets"},
		{Out: &out.TxErrors, File: "rx_errors"},
		{Out: &out.TxDropped, File: "rx_dropped"},
	}
	for _, netStat := range netStats {
		data, err := readSysfsNetworkStats(interfaceName, netStat.File)
		if err != nil {
			return nil, err
		}
		*(netStat.Out) = data
	}
	return out, nil
}

// Reads the specified statistics available under /sys/class/net/<EthInterface>/statistics
func readSysfsNetworkStats(ethInterface, statsFile string) (uint64, error) {
	data, err := ioutil.ReadFile(filepath.Join("/sys/class/net", ethInterface, "statistics", statsFile))
	if err != nil {
		return 0, err
	}
	return strconv.ParseUint(strings.TrimSpace(string(data)), 10, 64)
}

func Stats(containerDir string, containerMemoryLimit int64, machineMemory int64) (*ResourceStats, error) {
	f, err := os.Open(filepath.Join(containerDir, "state.json"))
	if err != nil {
		return nil, err
	}
	defer f.Close()

	type network struct {
		Type              string
		HostInterfaceName string
	}

	state := struct {
		CgroupPaths map[string]string `json:"cgroup_paths"`
		Networks    []network
	}{}

	if err := json.NewDecoder(f).Decode(&state); err != nil {
		return nil, err
	}
	now := time.Now()

	mgr := fs.Manager{Paths: state.CgroupPaths}
	cstats, err := mgr.GetStats()
	if err != nil {
		return nil, err
	}
	stats := &libcontainer.Stats{CgroupStats: cstats}
	// if the container does not have any memory limit specified set the
	// limit to the machines memory
	memoryLimit := containerMemoryLimit
	if memoryLimit == 0 {
		memoryLimit = machineMemory
	}
	for _, iface := range state.Networks {
		switch iface.Type {
		case "veth":
			istats, err := getNetworkInterfaceStats(iface.HostInterfaceName)
			if err != nil {
				return nil, err
			}
			stats.Interfaces = append(stats.Interfaces, istats)
		}
	}
	return &ResourceStats{
		Stats:       stats,
		Read:        now,
		MemoryLimit: memoryLimit,
	}, nil
}
