package system // import "github.com/docker/docker/api/types/system"

import (
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/plugins"
	"github.com/docker/docker/api/types/registry"
	"github.com/docker/docker/api/types/swarm"
)

// Info contains response of Engine API:
// GET "/info"
type Info struct {
	ID                 string
	Containers         int
	ContainersRunning  int
	ContainersPaused   int
	ContainersStopped  int
	Images             int
	Driver             string
	DriverStatus       [][2]string
	SystemStatus       [][2]string
	Plugins            plugins.Info
	MemoryLimit        bool
	SwapLimit          bool
	KernelMemory       bool
	CPUCfsPeriod       bool `json:"CpuCfsPeriod"`
	CPUCfsQuota        bool `json:"CpuCfsQuota"`
	CPUShares          bool
	CPUSet             bool
	IPv4Forwarding     bool
	BridgeNfIptables   bool
	BridgeNfIP6tables  bool `json:"BridgeNfIp6tables"`
	Debug              bool
	NFd                int
	OomKillDisable     bool
	NGoroutines        int
	SystemTime         string
	LoggingDriver      string
	CgroupDriver       string
	NEventsListener    int
	KernelVersion      string
	OperatingSystem    string
	OSType             string
	Architecture       string
	IndexServerAddress string
	RegistryConfig     *registry.ServiceConfig
	NCPU               int
	MemTotal           int64
	GenericResources   []swarm.GenericResource
	DockerRootDir      string
	HTTPProxy          string `json:"HttpProxy"`
	HTTPSProxy         string `json:"HttpsProxy"`
	NoProxy            string
	Name               string
	Labels             []string
	ExperimentalBuild  bool
	ServerVersion      string
	ClusterStore       string
	ClusterAdvertise   string
	Runtimes           map[string]Runtime
	DefaultRuntime     string
	Swarm              swarm.Info
	// LiveRestoreEnabled determines whether containers should be kept
	// running when the daemon is shutdown or upon daemon start if
	// running containers are detected
	LiveRestoreEnabled bool
	Isolation          container.Isolation
	InitBinary         string
	ContainerdCommit   Commit
	RuncCommit         Commit
	InitCommit         Commit
	SecurityOptions    []string
}

// Commit holds the Git-commit (SHA1) that a binary was built from, as reported
// in the version-string of external tools, such as containerd, or runC.
type Commit struct {
	ID       string // ID is the actual commit ID of external tool.
	Expected string // Expected is the commit ID of external tool expected by dockerd as set at build time.
}

// Runtime describes an OCI runtime
type Runtime struct {
	Path string   `json:"path"`
	Args []string `json:"runtimeArgs,omitempty"`
}
