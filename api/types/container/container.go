package container

import (
	"time"

	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/go-connections/nat"
)

// Container contains response of Engine API:
// GET "/containers/json"
type Container struct {
	ID         string `json:"Id"`
	Names      []string
	Image      string
	ImageID    string
	Command    string
	Created    int64
	Ports      []Port
	SizeRw     int64 `json:",omitempty"`
	SizeRootFs int64 `json:",omitempty"`
	Labels     map[string]string
	State      string
	Status     string
	HostConfig struct {
		NetworkMode string `json:",omitempty"`
	}
	NetworkSettings *SummaryNetworkSettings
	Mounts          []MountPoint
}

// SummaryNetworkSettings provides a summary of container's networks
// in /containers/json
type SummaryNetworkSettings struct {
	Networks map[string]*network.EndpointSettings
}

// MountPoint represents a mount point configuration inside the container.
// This is used for reporting the mountpoints in use by a container.
type MountPoint struct {
	Type        mount.Type `json:",omitempty"`
	Name        string     `json:",omitempty"`
	Source      string
	Destination string
	Driver      string `json:",omitempty"`
	Mode        string
	RW          bool
	Propagation mount.Propagation
}

// GraphDriverData Information about a container's graph driver.
// swagger:model GraphDriverData
type GraphDriverData struct {

	// data
	// Required: true
	Data map[string]string `json:"Data"`

	// name
	// Required: true
	Name string `json:"Name"`
}

// Port An open port on a container
// swagger:model Port
type Port struct {

	// Host IP address that the container's port is mapped to
	IP string `json:"IP,omitempty"`

	// Port on the container
	// Required: true
	PrivatePort uint16 `json:"PrivatePort"`

	// Port exposed on the host
	PublicPort uint16 `json:"PublicPort,omitempty"`

	// type
	// Required: true
	Type string `json:"Type"`
}

// ContainerJSONBase contains response of Engine API:
// GET "/containers/{name:.*}/json"
type ContainerJSONBase struct {
	ID              string `json:"Id"`
	Created         string
	Path            string
	Args            []string
	State           *State
	Image           string
	ResolvConfPath  string
	HostnamePath    string
	HostsPath       string
	LogPath         string
	Node            *Node `json:",omitempty"`
	Name            string
	RestartCount    int
	Driver          string
	Platform        string
	MountLabel      string
	ProcessLabel    string
	AppArmorProfile string
	ExecIDs         []string
	HostConfig      *HostConfig
	GraphDriver     GraphDriverData
	SizeRw          *int64 `json:",omitempty"`
	SizeRootFs      *int64 `json:",omitempty"`
}

// JSON is newly used struct along with MountPoint
type JSON struct {
	*ContainerJSONBase
	Mounts          []MountPoint
	Config          *Config
	NetworkSettings *NetworkSettings
}

// State stores container's running state
// it's part of ContainerJSONBase and will return by "inspect" command
type State struct {
	Status     string // String representation of the container state. Can be one of "created", "running", "paused", "restarting", "removing", "exited", or "dead"
	Running    bool
	Paused     bool
	Restarting bool
	OOMKilled  bool
	Dead       bool
	Pid        int
	ExitCode   int
	Error      string
	StartedAt  string
	FinishedAt string
	Health     *Health `json:",omitempty"`
}

// Node stores information about the node that a container
// is running on.  It's only available in Docker Swarm
type Node struct {
	ID        string
	IPAddress string `json:"IP"`
	Addr      string
	Name      string
	Cpus      int
	Memory    int64
	Labels    map[string]string
}

// HealthcheckResult stores information about a single run of a healthcheck probe
type HealthcheckResult struct {
	Start    time.Time // Start is the time this check started
	End      time.Time // End is the time this check ended
	ExitCode int       // ExitCode meanings: 0=healthy, 1=unhealthy, 2=reserved (considered unhealthy), else=error running probe
	Output   string    // Output from last check
}

// Health states
const (
	NoHealthcheck = "none"      // Indicates there is no healthcheck
	Starting      = "starting"  // Starting indicates that the container is not yet ready
	Healthy       = "healthy"   // Healthy indicates that the container is running correctly
	Unhealthy     = "unhealthy" // Unhealthy indicates that the container has a problem
)

// Health stores information about the container's healthcheck results
type Health struct {
	Status        string               // Status is one of Starting, Healthy or Unhealthy
	FailingStreak int                  // FailingStreak is the number of consecutive failures
	Log           []*HealthcheckResult // Log contains the last few results (oldest first)
}

// NetworkSettings exposes the network settings in the api
type NetworkSettings struct {
	NetworkSettingsBase
	DefaultNetworkSettings
	Networks map[string]*network.EndpointSettings
}

// NetworkSettingsBase holds basic information about networks
type NetworkSettingsBase struct {
	Bridge                 string      // Bridge is the Bridge name the network uses(e.g. `docker0`)
	SandboxID              string      // SandboxID uniquely represents a container's network stack
	HairpinMode            bool        // HairpinMode specifies if hairpin NAT should be enabled on the virtual interface
	LinkLocalIPv6Address   string      // LinkLocalIPv6Address is an IPv6 unicast address using the link-local prefix
	LinkLocalIPv6PrefixLen int         // LinkLocalIPv6PrefixLen is the prefix length of an IPv6 unicast address
	Ports                  nat.PortMap // Ports is a collection of PortBinding indexed by Port
	SandboxKey             string      // SandboxKey identifies the sandbox
	SecondaryIPAddresses   []network.Address
	SecondaryIPv6Addresses []network.Address
}

// DefaultNetworkSettings holds network information
// during the 2 release deprecation period.
// It will be removed in Docker 1.11.
type DefaultNetworkSettings struct {
	EndpointID          string // EndpointID uniquely represents a service endpoint in a Sandbox
	Gateway             string // Gateway holds the gateway address for the network
	GlobalIPv6Address   string // GlobalIPv6Address holds network's global IPv6 address
	GlobalIPv6PrefixLen int    // GlobalIPv6PrefixLen represents mask length of network's global IPv6 address
	IPAddress           string // IPAddress holds the IPv4 address for the network
	IPPrefixLen         int    // IPPrefixLen represents mask length of network's IPv4 address
	IPv6Gateway         string // IPv6Gateway holds gateway address specific for IPv6
	MacAddress          string // MacAddress holds the MAC address for the network
}

// IDResponse Response to an API call that returns just an Id
// swagger:model IdResponse
type IDResponse struct {

	// The id of the newly created object.
	// Required: true
	ID string `json:"Id"`
}
