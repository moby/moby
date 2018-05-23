package types // import "github.com/docker/docker/api/types"

import (
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/errors"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/api/types/plugins"
	"github.com/docker/docker/api/types/swarm"
	"github.com/docker/docker/api/types/system"
	"github.com/docker/docker/api/types/volume"
)

// RootFS returns Image's RootFS description including the layer IDs.
type RootFS = image.RootFS

// ImageInspect contains response of Engine API:
// GET "/images/{name:.*}/json"
type ImageInspect = image.Inspect

// ImageMetadata contains engine-local data about the image
type ImageMetadata = image.Metadata

// Container contains response of Engine API:
// GET "/containers/json"
type Container = container.Container

// CopyConfig contains request body of Engine API:
// POST "/containers/"+containerID+"/copy"
type CopyConfig = container.CopyConfig

// ContainerPathStat is used to encode the header from
// GET "/containers/{name:.*}/archive"
// "Name" is the file or directory name.
type ContainerPathStat = container.PathStat

// ContainerStats contains response of Engine API:
// GET "/stats"
type ContainerStats = container.Stats

// Ping contains response of Engine API:
// GET "/_ping"
type Ping = system.Ping

// ComponentVersion describes the version information for a specific component.
type ComponentVersion = system.ComponentVersion

// Version contains response of Engine API:
// GET "/version"
type Version = system.Version

// Info contains response of Engine API:
// GET "/info"
type Info = system.Info

// KeyValue holds a key/value pair
type KeyValue = container.KeyValue

// SecurityOpt contains the name and options of a security option
type SecurityOpt = container.SecurityOpt

// DecodeSecurityOptions decodes a security options string slice to a type safe
// SecurityOpt
func DecodeSecurityOptions(opts []string) ([]SecurityOpt, error) {
	return container.DecodeSecurityOptions(opts)
}

// PluginsInfo is a temp struct holding Plugins name
// registered with docker daemon. It is used by Info struct
type PluginsInfo = plugins.Info

// ExecStartCheck is a temp struct used by execStart
// Config fields is part of ExecConfig in runconfig package
type ExecStartCheck = container.ExecStartCheck

// HealthcheckResult stores information about a single run of a healthcheck probe
type HealthcheckResult = container.HealthcheckResult

// Health states
const (
	NoHealthcheck = container.NoHealthcheck // Indicates there is no healthcheck
	Starting      = container.Starting      // Starting indicates that the container is not yet ready
	Healthy       = container.Healthy       // Healthy indicates that the container is running correctly
	Unhealthy     = container.Unhealthy     // Unhealthy indicates that the container has a problem
)

// Health stores information about the container's healthcheck results
type Health = container.Health

// ContainerState stores container's running state
// it's part of ContainerJSONBase and will return by "inspect" command
type ContainerState = container.State

// ContainerNode stores information about the node that a container
// is running on.  It's only available in Docker Swarm
type ContainerNode = container.Node

// ContainerJSONBase contains response of Engine API:
// GET "/containers/{name:.*}/json"
type ContainerJSONBase = container.ContainerJSONBase

// ContainerJSON is newly used struct along with MountPoint
type ContainerJSON = container.JSON

// NetworkSettings exposes the network settings in the api
type NetworkSettings = container.NetworkSettings

// NetworkSettingsBase holds basic information about networks
type NetworkSettingsBase = container.NetworkSettingsBase

// DefaultNetworkSettings holds network information
// during the 2 release deprecation period.
// It will be removed in Docker 1.11.
type DefaultNetworkSettings = container.DefaultNetworkSettings

// SummaryNetworkSettings provides a summary of container's networks
// in /containers/json
type SummaryNetworkSettings = container.SummaryNetworkSettings

// MountPoint represents a mount point configuration inside the container.
// This is used for reporting the mountpoints in use by a container.
type MountPoint = container.MountPoint

// NetworkResource is the body of the "get network" http response message
type NetworkResource = network.Resource

// EndpointResource contains network resources allocated and used for a container in a network
type EndpointResource = network.EndpointResource

// NetworkCreate is the expected body of the "create network" http request message
type NetworkCreate = network.NetworkCreate

// NetworkCreateRequest is the request message sent to the server for network create call.
type NetworkCreateRequest = network.CreateRequest

// NetworkCreateResponse is the response message sent by the server for network create call
type NetworkCreateResponse = network.CreateResponse

// NetworkConnect represents the data to be used to connect a container to the network
type NetworkConnect = network.Connect

// NetworkDisconnect represents the data to be used to disconnect a container from the network
type NetworkDisconnect = network.Disconnect

// Checkpoint represents the details of a checkpoint
type Checkpoint = container.Checkpoint

// Commit holds the Git-commit (SHA1) that a binary was built from, as reported
// in the version-string of external tools, such as containerd, or runC.
type Commit = system.Commit

// Runtime describes an OCI runtime
type Runtime = system.Runtime

// DiskUsage contains response of Engine API:
// GET "/system/df"
type DiskUsage = system.DiskUsage

// ContainersPruneReport contains the response for Engine API:
// POST "/containers/prune"
type ContainersPruneReport = container.PruneReport

// VolumesPruneReport contains the response for Engine API:
// POST "/volumes/prune"
type VolumesPruneReport = volume.PruneReport

// ImagesPruneReport contains the response for Engine API:
// POST "/images/prune"
type ImagesPruneReport = image.PruneReport

// BuildCachePruneReport contains the response for Engine API:
// POST "/build/prune"
type BuildCachePruneReport = image.BuildCachePruneReport

// NetworksPruneReport contains the response for Engine API:
// POST "/networks/prune"
type NetworksPruneReport = network.PruneReport

// SecretCreateResponse contains the information returned to a client
// on the creation of a new secret.
type SecretCreateResponse = swarm.SecretCreateResponse

// ConfigCreateResponse contains the information returned to a client
// on the creation of a new config.
type ConfigCreateResponse = swarm.ConfigCreateResponse

// PushResult contains the tag, manifest digest, and manifest size from the
// push. It's used to signal this information to the trust code in the client
// so it can sign the manifest if necessary.
type PushResult = image.PushResult

// BuildResult contains the image id of a successful build
type BuildResult = image.BuildResult

// ErrorResponse Represents an error.
type ErrorResponse = errors.Response
