package types

import (
	"os"
	"time"

	"github.com/docker/engine-api/types/container"
	"github.com/docker/engine-api/types/network"
	"github.com/docker/engine-api/types/registry"
	"github.com/docker/go-connections/nat"
)

// ContainerCreateResponse contains the information returned to a client on the
// creation of a new container.
type ContainerCreateResponse struct {
	// ID is the ID of the created container.
	ID string `json:"Id,omitempty"`

	// Warnings are any warnings encountered during the creation of the container.
	Warnings []string `json:"Warnings,omitempty"`
}

// ContainerExecCreateResponse contains response of Remote API:
// POST "/containers/{name:.*}/exec"
type ContainerExecCreateResponse struct {
	// ID is the exec ID.
	ID string `json:"Id,omitempty"`
}

// ContainerUpdateResponse contains response of Remote API:
// POST /containers/{name:.*}/update
type ContainerUpdateResponse struct {
	// Warnings are any warnings encountered during the updating of the container.
	Warnings []string `json:"Warnings,omitempty"`
}

// AuthResponse contains response of Remote API:
// POST "/auth"
type AuthResponse struct {
	// Status is the authentication status
	Status string `json:"Status,omitempty"`
}

// ContainerWaitResponse contains response of Remote API:
// POST "/containers/"+containerID+"/wait"
type ContainerWaitResponse struct {
	// StatusCode is the status code of the wait job
	StatusCode int `json:"StatusCode,omitempty"`
}

// ContainerCommitResponse contains response of Remote API:
// POST "/commit?container="+containerID
type ContainerCommitResponse struct {
	ID string `json:"Id,omitempty"`
}

// ContainerChange contains response of Remote API:
// GET "/containers/{name:.*}/changes"
type ContainerChange struct {
	Kind int    `json:",omitempty"`
	Path string `json:",omitempty"`
}

// ImageHistory contains response of Remote API:
// GET "/images/{name:.*}/history"
type ImageHistory struct {
	ID        string   `json:"Id,omitempty"`
	Created   int64    `json:",omitempty"`
	CreatedBy string   `json:",omitempty"`
	Tags      []string `json:",omitempty"`
	Size      int64    `json:",omitempty"`
	Comment   string   `json:",omitempty"`
}

// ImageDelete contains response of Remote API:
// DELETE "/images/{name:.*}"
type ImageDelete struct {
	Untagged string `json:",omitempty"`
	Deleted  string `json:",omitempty"`
}

// Image contains response of Remote API:
// GET "/images/json"
type Image struct {
	ID          string            `json:"Id,omitempty"`
	ParentID    string            `json:"ParentId,omitempty"`
	RepoTags    []string          `json:",omitempty"`
	RepoDigests []string          `json:",omitempty"`
	Created     int64             `json:",omitempty"`
	Size        int64             `json:",omitempty"`
	VirtualSize int64             `json:",omitempty"`
	Labels      map[string]string `json:",omitempty"`
}

// GraphDriverData returns Image's graph driver config info
// when calling inspect command
type GraphDriverData struct {
	Name string            `json:",omitempty"`
	Data map[string]string `json:",omitempty"`
}

// ImageInspect contains response of Remote API:
// GET "/images/{name:.*}/json"
type ImageInspect struct {
	ID              string            `json:"Id,omitempty"`
	RepoTags        []string          `json:",omitempty"`
	RepoDigests     []string          `json:",omitempty"`
	Parent          string            `json:",omitempty"`
	Comment         string            `json:",omitempty"`
	Created         string            `json:",omitempty"`
	Container       string            `json:",omitempty"`
	ContainerConfig *container.Config `json:",omitempty"`
	DockerVersion   string            `json:",omitempty"`
	Author          string            `json:",omitempty"`
	Config          *container.Config `json:",omitempty"`
	Architecture    string            `json:",omitempty"`
	Os              string            `json:",omitempty"`
	Size            int64             `json:",omitempty"`
	VirtualSize     int64             `json:",omitempty"`
	GraphDriver     GraphDriverData   `json:",omitempty"`
}

// Port stores open ports info of container
// e.g. {"PrivatePort": 8080, "PublicPort": 80, "Type": "tcp"}
type Port struct {
	IP          string `json:",omitempty"`
	PrivatePort int    `json:",omitempty"`
	PublicPort  int    `json:",omitempty"`
	Type        string `json:",omitempty"`
}

// Container contains response of Remote API:
// GET  "/containers/json"
type Container struct {
	ID         string `json:"Id"`
	Names      []string
	Image      string
	ImageID    string `json:",omitempty"`
	Command    string
	Created    int64
	Ports      []Port
	SizeRw     int64 `json:",omitempty"`
	SizeRootFs int64 `json:",omitempty"`
	Labels     map[string]string
	Status     string
	HostConfig struct {
		NetworkMode string `json:",omitempty"`
	}
	NetworkSettings *SummaryNetworkSettings
}

// CopyConfig contains request body of Remote API:
// POST "/containers/"+containerID+"/copy"
type CopyConfig struct {
	Resource string
}

// ContainerPathStat is used to encode the header from
// GET "/containers/{name:.*}/archive"
// "Name" is the file or directory name.
type ContainerPathStat struct {
	Name       string      `json:"name,omitempty"`
	Size       int64       `json:"size,omitempty"`
	Mode       os.FileMode `json:"mode,omitempty"`
	Mtime      time.Time   `json:"mtime,omitempty"`
	LinkTarget string      `json:"linkTarget,omitempty"`
}

// ContainerProcessList contains response of Remote API:
// GET "/containers/{name:.*}/top"
type ContainerProcessList struct {
	Processes [][]string `json:",omitempty"`
	Titles    []string   `json:",omitempty"`
}

// Version contains response of Remote API:
// GET "/version"
type Version struct {
	Version       string `json:",omitempty"`
	APIVersion    string `json:"ApiVersion,omitempty"`
	GitCommit     string `json:",omitempty"`
	GoVersion     string `json:",omitempty"`
	Os            string `json:",omitempty"`
	Arch          string `json:",omitempty"`
	KernelVersion string `json:",omitempty"`
	Experimental  bool   `json:",omitempty"`
	BuildTime     string `json:",omitempty"`
}

// Info contains response of Remote API:
// GET "/info"
type Info struct {
	ID                 string `json:",omitempty"`
	Containers         int
	ContainersRunning  int
	ContainersPaused   int
	ContainersStopped  int
	Images             int
	Driver             string
	DriverStatus       [][2]string `json:",omitempty"`
	Plugins            PluginsInfo `json:",omitempty"`
	MemoryLimit        bool        `json:",omitempty"`
	SwapLimit          bool        `json:",omitempty"`
	CPUCfsPeriod       bool        `json:"CpuCfsPeriod,omitempty"`
	CPUCfsQuota        bool        `json:"CpuCfsQuota,omitempty"`
	CPUShares          bool        `json:",omitempty"`
	CPUSet             bool        `json:",omitempty"`
	IPv4Forwarding     bool        `json:",omitempty"`
	BridgeNfIptables   bool        `json:",omitempty"`
	BridgeNfIP6tables  bool        `json:"BridgeNfIp6tables,omitempty"`
	Debug              bool        `json:",omitempty"`
	NFd                int         `json:",omitempty"`
	OomKillDisable     bool        `json:",omitempty"`
	NGoroutines        int         `json:",omitempty"`
	SystemTime         string      `json:",omitempty"`
	ExecutionDriver    string
	LoggingDriver      string
	NEventsListener    int `json:",omitempty"`
	KernelVersion      string
	OperatingSystem    string
	OSType             string
	Architecture       string
	IndexServerAddress string                  `json:",omitempty"`
	RegistryConfig     *registry.ServiceConfig `json:",omitempty"`
	InitSha1           string                  `json:",omitempty"`
	InitPath           string                  `json:",omitempty"`
	NCPU               int
	MemTotal           int64
	DockerRootDir      string   `json:",omitempty"`
	HTTPProxy          string   `json:"HttpProxy,omitempty"`
	HTTPSProxy         string   `json:"HttpsProxy,omitempty"`
	NoProxy            string   `json:",omitempty"`
	Name               string   `json:",omitempty"`
	Labels             []string `json:",omitempty"`
	ExperimentalBuild  bool     `json:",omitempty"`
	ServerVersion      string
	ClusterStore       string `json:",omitempty"`
	ClusterAdvertise   string `json:",omitempty"`
}

// PluginsInfo is temp struct holds Plugins name
// registered with docker daemon. It used by Info struct
type PluginsInfo struct {
	// List of Volume plugins registered
	Volume []string `json:",omitempty"`
	// List of Network plugins registered
	Network []string `json:",omitempty"`
	// List of Authorization plugins registered
	Authorization []string `json:",omitempty"`
}

// ExecStartCheck is a temp struct used by execStart
// Config fields is part of ExecConfig in runconfig package
type ExecStartCheck struct {
	// ExecStart will first check if it's detached
	Detach bool `json:",omitempty"`
	// Check if there's a tty
	Tty bool `json:",omitempty"`
}

// ContainerState stores container's running state
// it's part of ContainerJSONBase and will return by "inspect" command
type ContainerState struct {
	Status     string `json:",omitempty"`
	Running    bool   `json:",omitempty"`
	Paused     bool   `json:",omitempty"`
	Restarting bool   `json:",omitempty"`
	OOMKilled  bool   `json:",omitempty"`
	Dead       bool   `json:",omitempty"`
	Pid        int    `json:",omitempty"`
	ExitCode   int    `json:",omitempty"`
	Error      string `json:",omitempty"`
	StartedAt  string `json:",omitempty"`
	FinishedAt string `json:",omitempty"`
}

// ContainerJSONBase contains response of Remote API:
// GET "/containers/{name:.*}/json"
type ContainerJSONBase struct {
	ID              string `json:"Id,omitempty"`
	Created         string `json:",omitempty"`
	Path            string `json:",omitempty"`
	Args            []string
	State           *ContainerState `json:",omitempty"`
	Image           string          `json:",omitempty"`
	ResolvConfPath  string          `json:",omitempty"`
	HostnamePath    string          `json:",omitempty"`
	HostsPath       string          `json:",omitempty"`
	LogPath         string          `json:",omitempty"`
	Name            string          `json:",omitempty"`
	RestartCount    int             `json:",omitempty"`
	Driver          string          `json:",omitempty"`
	MountLabel      string
	ProcessLabel    string
	AppArmorProfile string                `json:",omitempty"`
	ExecIDs         []string              `json:",omitempty"`
	HostConfig      *container.HostConfig `json:",omitempty"`
	GraphDriver     GraphDriverData       `json:",omitempty"`
	SizeRw          *int64                `json:",omitempty"`
	SizeRootFs      *int64                `json:",omitempty"`
}

// ContainerJSON is newly used struct along with MountPoint
type ContainerJSON struct {
	*ContainerJSONBase `json:",omitempty"`
	Mounts             []MountPoint      `json:",omitempty"`
	Config             *container.Config `json:",omitempty"`
	NetworkSettings    *NetworkSettings  `json:",omitempty"`
}

// NetworkSettings exposes the network settings in the api
type NetworkSettings struct {
	NetworkSettingsBase    `json:",omitempty"`
	DefaultNetworkSettings `json:",omitempty"`
	Networks               map[string]*network.EndpointSettings `json:",omitempty"`
}

// SummaryNetworkSettings provides a summary of container's networks
// in /containers/json
type SummaryNetworkSettings struct {
	Networks map[string]*network.EndpointSettings `json:",omitempty"`
}

// NetworkSettingsBase holds basic information about networks
type NetworkSettingsBase struct {
	Bridge                 string            `json:",omitempty"`
	SandboxID              string            `json:",omitempty"`
	HairpinMode            bool              `json:",omitempty"`
	LinkLocalIPv6Address   string            `json:",omitempty"`
	LinkLocalIPv6PrefixLen int               `json:",omitempty"`
	Ports                  nat.PortMap       `json:",omitempty"`
	SandboxKey             string            `json:",omitempty"`
	SecondaryIPAddresses   []network.Address `json:",omitempty"`
	SecondaryIPv6Addresses []network.Address `json:",omitempty"`
}

// DefaultNetworkSettings holds network information
// during the 2 release deprecation period.
// It will be removed in Docker 1.11.
type DefaultNetworkSettings struct {
	EndpointID          string `json:",omitempty"`
	Gateway             string `json:",omitempty"`
	GlobalIPv6Address   string `json:",omitempty"`
	GlobalIPv6PrefixLen int    `json:",omitempty"`
	IPAddress           string `json:",omitempty"`
	IPPrefixLen         int    `json:",omitempty"`
	IPv6Gateway         string `json:",omitempty"`
	MacAddress          string `json:",omitempty"`
}

// MountPoint represents a mount point configuration inside the container.
type MountPoint struct {
	Name        string `json:",omitempty"`
	Source      string `json:",omitempty"`
	Destination string `json:",omitempty"`
	Driver      string `json:",omitempty"`
	Mode        string `json:",omitempty"`
	RW          bool   `json:",omitempty"`
	Propagation string `json:",omitempty"`
}

// Volume represents the configuration of a volume for the remote API
type Volume struct {
	Name       string `json:",omitempty"` // Name is the name of the volume
	Driver     string `json:",omitempty"` // Driver is the Driver name used to create the volume
	Mountpoint string `json:",omitempty"` // Mountpoint is the location on disk of the volume
}

// VolumesListResponse contains the response for the remote API:
// GET "/volumes"
type VolumesListResponse struct {
	Volumes  []*Volume `json:",omitempty"` // Volumes is the list of volumes being returned
	Warnings []string  `json:",omitempty"` // Warnings is a list of warnings that occurred when getting the list from the volume drivers
}

// VolumeCreateRequest contains the response for the remote API:
// POST "/volumes/create"
type VolumeCreateRequest struct {
	Name       string            `json:",omitempty"` // Name is the requested name of the volume
	Driver     string            `json:",omitempty"` // Driver is the name of the driver that should be used to create the volume
	DriverOpts map[string]string `json:",omitempty"` // DriverOpts holds the driver specific options to use for when creating the volume.
}

// NetworkResource is the body of the "get network" http response message
type NetworkResource struct {
	Name       string                      `json:",omitempty"`
	ID         string                      `json:"Id,omitempty"`
	Scope      string                      `json:",omitempty"`
	Driver     string                      `json:",omitempty"`
	IPAM       network.IPAM                `json:",omitempty"`
	Containers map[string]EndpointResource `json:",omitempty"`
	Options    map[string]string           `json:",omitempty"`
}

// EndpointResource contains network resources allocated and used for a container in a network
type EndpointResource struct {
	Name        string `json:",omitempty"`
	EndpointID  string `json:",omitempty"`
	MacAddress  string `json:",omitempty"`
	IPv4Address string `json:",omitempty"`
	IPv6Address string `json:",omitempty"`
}

// NetworkCreate is the expected body of the "create network" http request message
type NetworkCreate struct {
	Name           string            `json:",omitempty"`
	CheckDuplicate bool              `json:",omitempty"`
	Driver         string            `json:",omitempty"`
	IPAM           network.IPAM      `json:",omitempty"`
	Internal       bool              `json:",omitempty"`
	Options        map[string]string `json:",omitempty"`
}

// NetworkCreateResponse is the response message sent by the server for network create call
type NetworkCreateResponse struct {
	ID      string `json:"Id,omitempty"`
	Warning string `json:",omitempty"`
}

// NetworkConnect represents the data to be used to connect a container to the network
type NetworkConnect struct {
	Container      string                    `json:",omitempty"`
	EndpointConfig *network.EndpointSettings `json:",omitempty"`
}

// NetworkDisconnect represents the data to be used to disconnect a container from the network
type NetworkDisconnect struct {
	Container string `json:",omitempty"`
	Force     bool   `json:",omitempty"`
}
