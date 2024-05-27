package types // import "github.com/docker/docker/api/types"

import (
	"io"
	"os"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/api/types/swarm"
	"github.com/docker/docker/api/types/volume"
	"github.com/docker/go-connections/nat"
)

const (
	// MediaTypeRawStream is vendor specific MIME-Type set for raw TTY streams
	MediaTypeRawStream = "application/vnd.docker.raw-stream"

	// MediaTypeMultiplexedStream is vendor specific MIME-Type set for stdin/stdout/stderr multiplexed streams
	MediaTypeMultiplexedStream = "application/vnd.docker.multiplexed-stream"
)

// RootFS returns Image's RootFS description including the layer IDs.
type RootFS struct {
	Type   string   `json:",omitempty"`
	Layers []string `json:",omitempty"`
}

// ImageInspect contains response of Engine API:
// GET "/images/{name:.*}/json"
type ImageInspect struct {
	// ID is the content-addressable ID of an image.
	//
	// This identifier is a content-addressable digest calculated from the
	// image's configuration (which includes the digests of layers used by
	// the image).
	//
	// Note that this digest differs from the `RepoDigests` below, which
	// holds digests of image manifests that reference the image.
	ID string `json:"Id"`

	// RepoTags is a list of image names/tags in the local image cache that
	// reference this image.
	//
	// Multiple image tags can refer to the same image, and this list may be
	// empty if no tags reference the image, in which case the image is
	// "untagged", in which case it can still be referenced by its ID.
	RepoTags []string

	// RepoDigests is a list of content-addressable digests of locally available
	// image manifests that the image is referenced from. Multiple manifests can
	// refer to the same image.
	//
	// These digests are usually only available if the image was either pulled
	// from a registry, or if the image was pushed to a registry, which is when
	// the manifest is generated and its digest calculated.
	RepoDigests []string

	// Parent is the ID of the parent image.
	//
	// Depending on how the image was created, this field may be empty and
	// is only set for images that were built/created locally. This field
	// is empty if the image was pulled from an image registry.
	Parent string

	// Comment is an optional message that can be set when committing or
	// importing the image.
	Comment string

	// Created is the date and time at which the image was created, formatted in
	// RFC 3339 nano-seconds (time.RFC3339Nano).
	//
	// This information is only available if present in the image,
	// and omitted otherwise.
	Created string `json:",omitempty"`

	// Container is the ID of the container that was used to create the image.
	//
	// Depending on how the image was created, this field may be empty.
	//
	// Deprecated: this field is omitted in API v1.45, but kept for backward compatibility.
	Container string `json:",omitempty"`

	// ContainerConfig is an optional field containing the configuration of the
	// container that was last committed when creating the image.
	//
	// Previous versions of Docker builder used this field to store build cache,
	// and it is not in active use anymore.
	//
	// Deprecated: this field is omitted in API v1.45, but kept for backward compatibility.
	ContainerConfig *container.Config `json:",omitempty"`

	// DockerVersion is the version of Docker that was used to build the image.
	//
	// Depending on how the image was created, this field may be empty.
	DockerVersion string

	// Author is the name of the author that was specified when committing the
	// image, or as specified through MAINTAINER (deprecated) in the Dockerfile.
	Author string
	Config *container.Config

	// Architecture is the hardware CPU architecture that the image runs on.
	Architecture string

	// Variant is the CPU architecture variant (presently ARM-only).
	Variant string `json:",omitempty"`

	// OS is the Operating System the image is built to run on.
	Os string

	// OsVersion is the version of the Operating System the image is built to
	// run on (especially for Windows).
	OsVersion string `json:",omitempty"`

	// Size is the total size of the image including all layers it is composed of.
	Size int64

	// VirtualSize is the total size of the image including all layers it is
	// composed of.
	//
	// Deprecated: this field is omitted in API v1.44, but kept for backward compatibility. Use Size instead.
	VirtualSize int64 `json:"VirtualSize,omitempty"`

	// GraphDriver holds information about the storage driver used to store the
	// container's and image's filesystem.
	GraphDriver GraphDriverData

	// RootFS contains information about the image's RootFS, including the
	// layer IDs.
	RootFS RootFS

	// Metadata of the image in the local cache.
	//
	// This information is local to the daemon, and not part of the image itself.
	Metadata image.Metadata
}

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

// CopyConfig contains request body of Engine API:
// POST "/containers/"+containerID+"/copy"
type CopyConfig struct {
	Resource string
}

// ContainerPathStat is used to encode the header from
// GET "/containers/{name:.*}/archive"
// "Name" is the file or directory name.
type ContainerPathStat struct {
	Name       string      `json:"name"`
	Size       int64       `json:"size"`
	Mode       os.FileMode `json:"mode"`
	Mtime      time.Time   `json:"mtime"`
	LinkTarget string      `json:"linkTarget"`
}

// ContainerStats contains response of Engine API:
// GET "/stats"
type ContainerStats struct {
	Body   io.ReadCloser `json:"body"`
	OSType string        `json:"ostype"`
}

// Ping contains response of Engine API:
// GET "/_ping"
type Ping struct {
	APIVersion     string
	OSType         string
	Experimental   bool
	BuilderVersion BuilderVersion

	// SwarmStatus provides information about the current swarm status of the
	// engine, obtained from the "Swarm" header in the API response.
	//
	// It can be a nil struct if the API version does not provide this header
	// in the ping response, or if an error occurred, in which case the client
	// should use other ways to get the current swarm status, such as the /swarm
	// endpoint.
	SwarmStatus *swarm.Status
}

// ComponentVersion describes the version information for a specific component.
type ComponentVersion struct {
	Name    string
	Version string
	Details map[string]string `json:",omitempty"`
}

// Version contains response of Engine API:
// GET "/version"
type Version struct {
	Platform   struct{ Name string } `json:",omitempty"`
	Components []ComponentVersion    `json:",omitempty"`

	// The following fields are deprecated, they relate to the Engine component and are kept for backwards compatibility

	Version       string
	APIVersion    string `json:"ApiVersion"`
	MinAPIVersion string `json:"MinAPIVersion,omitempty"`
	GitCommit     string
	GoVersion     string
	Os            string
	Arch          string
	KernelVersion string `json:",omitempty"`
	Experimental  bool   `json:",omitempty"`
	BuildTime     string `json:",omitempty"`
}

// ExecStartCheck is a temp struct used by execStart
// Config fields is part of ExecConfig in runconfig package
type ExecStartCheck struct {
	// ExecStart will first check if it's detached
	Detach bool
	// Check if there's a tty
	Tty bool
	// Terminal size [height, width], unused if Tty == false
	ConsoleSize *[2]uint `json:",omitempty"`
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

// ContainerState stores container's running state
// it's part of ContainerJSONBase and will return by "inspect" command
type ContainerState struct {
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

// ContainerNode stores information about the node that a container
// is running on.  It's only used by the Docker Swarm standalone API
type ContainerNode struct {
	ID        string
	IPAddress string `json:"IP"`
	Addr      string
	Name      string
	Cpus      int
	Memory    int64
	Labels    map[string]string
}

// ContainerJSONBase contains response of Engine API:
// GET "/containers/{name:.*}/json"
type ContainerJSONBase struct {
	ID              string `json:"Id"`
	Created         string
	Path            string
	Args            []string
	State           *ContainerState
	Image           string
	ResolvConfPath  string
	HostnamePath    string
	HostsPath       string
	LogPath         string
	Node            *ContainerNode `json:",omitempty"` // Node is only propagated by Docker Swarm standalone API
	Name            string
	RestartCount    int
	Driver          string
	Platform        string
	MountLabel      string
	ProcessLabel    string
	AppArmorProfile string
	ExecIDs         []string
	HostConfig      *container.HostConfig
	GraphDriver     GraphDriverData
	SizeRw          *int64 `json:",omitempty"`
	SizeRootFs      *int64 `json:",omitempty"`
}

// ContainerJSON is newly used struct along with MountPoint
type ContainerJSON struct {
	*ContainerJSONBase
	Mounts          []MountPoint
	Config          *container.Config
	NetworkSettings *NetworkSettings
}

// NetworkSettings exposes the network settings in the api
type NetworkSettings struct {
	NetworkSettingsBase
	DefaultNetworkSettings
	Networks map[string]*network.EndpointSettings
}

// SummaryNetworkSettings provides a summary of container's networks
// in /containers/json
type SummaryNetworkSettings struct {
	Networks map[string]*network.EndpointSettings
}

// NetworkSettingsBase holds networking state for a container when inspecting it.
type NetworkSettingsBase struct {
	Bridge     string      // Bridge contains the name of the default bridge interface iff it was set through the daemon --bridge flag.
	SandboxID  string      // SandboxID uniquely represents a container's network stack
	SandboxKey string      // SandboxKey identifies the sandbox
	Ports      nat.PortMap // Ports is a collection of PortBinding indexed by Port

	// HairpinMode specifies if hairpin NAT should be enabled on the virtual interface
	//
	// Deprecated: This field is never set and will be removed in a future release.
	HairpinMode bool
	// LinkLocalIPv6Address is an IPv6 unicast address using the link-local prefix
	//
	// Deprecated: This field is never set and will be removed in a future release.
	LinkLocalIPv6Address string
	// LinkLocalIPv6PrefixLen is the prefix length of an IPv6 unicast address
	//
	// Deprecated: This field is never set and will be removed in a future release.
	LinkLocalIPv6PrefixLen int
	SecondaryIPAddresses   []network.Address // Deprecated: This field is never set and will be removed in a future release.
	SecondaryIPv6Addresses []network.Address // Deprecated: This field is never set and will be removed in a future release.
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

// MountPoint represents a mount point configuration inside the container.
// This is used for reporting the mountpoints in use by a container.
type MountPoint struct {
	// Type is the type of mount, see `Type<foo>` definitions in
	// github.com/docker/docker/api/types/mount.Type
	Type mount.Type `json:",omitempty"`

	// Name is the name reference to the underlying data defined by `Source`
	// e.g., the volume name.
	Name string `json:",omitempty"`

	// Source is the source location of the mount.
	//
	// For volumes, this contains the storage location of the volume (within
	// `/var/lib/docker/volumes/`). For bind-mounts, and `npipe`, this contains
	// the source (host) part of the bind-mount. For `tmpfs` mount points, this
	// field is empty.
	Source string

	// Destination is the path relative to the container root (`/`) where the
	// Source is mounted inside the container.
	Destination string

	// Driver is the volume driver used to create the volume (if it is a volume).
	Driver string `json:",omitempty"`

	// Mode is a comma separated list of options supplied by the user when
	// creating the bind/volume mount.
	//
	// The default is platform-specific (`"z"` on Linux, empty on Windows).
	Mode string

	// RW indicates whether the mount is mounted writable (read-write).
	RW bool

	// Propagation describes how mounts are propagated from the host into the
	// mount point, and vice-versa. Refer to the Linux kernel documentation
	// for details:
	// https://www.kernel.org/doc/Documentation/filesystems/sharedsubtree.txt
	//
	// This field is not used on Windows.
	Propagation mount.Propagation
}

// NetworkResource is the body of the "get network" http response message
type NetworkResource struct {
	Name       string                         // Name is the requested name of the network
	ID         string                         `json:"Id"` // ID uniquely identifies a network on a single machine
	Created    time.Time                      // Created is the time the network created
	Scope      string                         // Scope describes the level at which the network exists (e.g. `swarm` for cluster-wide or `local` for machine level)
	Driver     string                         // Driver is the Driver name used to create the network (e.g. `bridge`, `overlay`)
	EnableIPv6 bool                           // EnableIPv6 represents whether to enable IPv6
	IPAM       network.IPAM                   // IPAM is the network's IP Address Management
	Internal   bool                           // Internal represents if the network is used internal only
	Attachable bool                           // Attachable represents if the global scope is manually attachable by regular containers from workers in swarm mode.
	Ingress    bool                           // Ingress indicates the network is providing the routing-mesh for the swarm cluster.
	ConfigFrom network.ConfigReference        // ConfigFrom specifies the source which will provide the configuration for this network.
	ConfigOnly bool                           // ConfigOnly networks are place-holder networks for network configurations to be used by other networks. ConfigOnly networks cannot be used directly to run containers or services.
	Containers map[string]EndpointResource    // Containers contains endpoints belonging to the network
	Options    map[string]string              // Options holds the network specific options to use for when creating the network
	Labels     map[string]string              // Labels holds metadata specific to the network being created
	Peers      []network.PeerInfo             `json:",omitempty"` // List of peer nodes for an overlay network
	Services   map[string]network.ServiceInfo `json:",omitempty"`
}

// EndpointResource contains network resources allocated and used for a container in a network
type EndpointResource struct {
	Name        string
	EndpointID  string
	MacAddress  string
	IPv4Address string
	IPv6Address string
}

// NetworkCreate is the expected body of the "create network" http request message
type NetworkCreate struct {
	// Deprecated: CheckDuplicate is deprecated since API v1.44, but it defaults to true when sent by the client
	// package to older daemons.
	CheckDuplicate bool                     `json:",omitempty"`
	Driver         string                   // Driver is the driver-name used to create the network (e.g. `bridge`, `overlay`)
	Scope          string                   // Scope describes the level at which the network exists (e.g. `swarm` for cluster-wide or `local` for machine level).
	EnableIPv6     bool                     // EnableIPv6 represents whether to enable IPv6.
	IPAM           *network.IPAM            // IPAM is the network's IP Address Management.
	Internal       bool                     // Internal represents if the network is used internal only.
	Attachable     bool                     // Attachable represents if the global scope is manually attachable by regular containers from workers in swarm mode.
	Ingress        bool                     // Ingress indicates the network is providing the routing-mesh for the swarm cluster.
	ConfigOnly     bool                     // ConfigOnly creates a config-only network. Config-only networks are place-holder networks for network configurations to be used by other networks. ConfigOnly networks cannot be used directly to run containers or services.
	ConfigFrom     *network.ConfigReference // ConfigFrom specifies the source which will provide the configuration for this network. The specified network must be a config-only network; see [NetworkCreate.ConfigOnly].
	Options        map[string]string        // Options specifies the network-specific options to use for when creating the network.
	Labels         map[string]string        // Labels holds metadata specific to the network being created.
}

// NetworkCreateRequest is the request message sent to the server for network create call.
type NetworkCreateRequest struct {
	NetworkCreate
	Name string // Name is the requested name of the network.
}

// NetworkCreateResponse is the response message sent by the server for network create call
type NetworkCreateResponse struct {
	ID      string `json:"Id"`
	Warning string
}

// NetworkConnect represents the data to be used to connect a container to the network
type NetworkConnect struct {
	Container      string
	EndpointConfig *network.EndpointSettings `json:",omitempty"`
}

// NetworkDisconnect represents the data to be used to disconnect a container from the network
type NetworkDisconnect struct {
	Container string
	Force     bool
}

// NetworkInspectOptions holds parameters to inspect network
type NetworkInspectOptions struct {
	Scope   string
	Verbose bool
}

// DiskUsageObject represents an object type used for disk usage query filtering.
type DiskUsageObject string

const (
	// ContainerObject represents a container DiskUsageObject.
	ContainerObject DiskUsageObject = "container"
	// ImageObject represents an image DiskUsageObject.
	ImageObject DiskUsageObject = "image"
	// VolumeObject represents a volume DiskUsageObject.
	VolumeObject DiskUsageObject = "volume"
	// BuildCacheObject represents a build-cache DiskUsageObject.
	BuildCacheObject DiskUsageObject = "build-cache"
)

// DiskUsageOptions holds parameters for system disk usage query.
type DiskUsageOptions struct {
	// Types specifies what object types to include in the response. If empty,
	// all object types are returned.
	Types []DiskUsageObject
}

// DiskUsage contains response of Engine API:
// GET "/system/df"
type DiskUsage struct {
	LayersSize  int64
	Images      []*image.Summary
	Containers  []*Container
	Volumes     []*volume.Volume
	BuildCache  []*BuildCache
	BuilderSize int64 `json:",omitempty"` // Deprecated: deprecated in API 1.38, and no longer used since API 1.40.
}

// ContainersPruneReport contains the response for Engine API:
// POST "/containers/prune"
type ContainersPruneReport struct {
	ContainersDeleted []string
	SpaceReclaimed    uint64
}

// VolumesPruneReport contains the response for Engine API:
// POST "/volumes/prune"
type VolumesPruneReport struct {
	VolumesDeleted []string
	SpaceReclaimed uint64
}

// ImagesPruneReport contains the response for Engine API:
// POST "/images/prune"
type ImagesPruneReport struct {
	ImagesDeleted  []image.DeleteResponse
	SpaceReclaimed uint64
}

// BuildCachePruneReport contains the response for Engine API:
// POST "/build/prune"
type BuildCachePruneReport struct {
	CachesDeleted  []string
	SpaceReclaimed uint64
}

// NetworksPruneReport contains the response for Engine API:
// POST "/networks/prune"
type NetworksPruneReport struct {
	NetworksDeleted []string
}

// SecretCreateResponse contains the information returned to a client
// on the creation of a new secret.
type SecretCreateResponse struct {
	// ID is the id of the created secret.
	ID string
}

// SecretListOptions holds parameters to list secrets
type SecretListOptions struct {
	Filters filters.Args
}

// ConfigCreateResponse contains the information returned to a client
// on the creation of a new config.
type ConfigCreateResponse struct {
	// ID is the id of the created config.
	ID string
}

// ConfigListOptions holds parameters to list configs
type ConfigListOptions struct {
	Filters filters.Args
}

// PushResult contains the tag, manifest digest, and manifest size from the
// push. It's used to signal this information to the trust code in the client
// so it can sign the manifest if necessary.
type PushResult struct {
	Tag    string
	Digest string
	Size   int
}

// BuildResult contains the image id of a successful build
type BuildResult struct {
	ID string
}

// BuildCache contains information about a build cache record.
type BuildCache struct {
	// ID is the unique ID of the build cache record.
	ID string
	// Parent is the ID of the parent build cache record.
	//
	// Deprecated: deprecated in API v1.42 and up, as it was deprecated in BuildKit; use Parents instead.
	Parent string `json:"Parent,omitempty"`
	// Parents is the list of parent build cache record IDs.
	Parents []string `json:" Parents,omitempty"`
	// Type is the cache record type.
	Type string
	// Description is a description of the build-step that produced the build cache.
	Description string
	// InUse indicates if the build cache is in use.
	InUse bool
	// Shared indicates if the build cache is shared.
	Shared bool
	// Size is the amount of disk space used by the build cache (in bytes).
	Size int64
	// CreatedAt is the date and time at which the build cache was created.
	CreatedAt time.Time
	// LastUsedAt is the date and time at which the build cache was last used.
	LastUsedAt *time.Time
	UsageCount int
}

// BuildCachePruneOptions hold parameters to prune the build cache
type BuildCachePruneOptions struct {
	All         bool
	KeepStorage int64
	Filters     filters.Args
}
