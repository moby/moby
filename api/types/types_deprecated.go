package types

import (
	"context"

	"github.com/docker/docker/api/types/build"
	"github.com/docker/docker/api/types/common"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/storage"
	"github.com/docker/docker/api/types/swarm"
)

// IDResponse Response to an API call that returns just an Id.
//
// Deprecated: use either [container.CommitResponse] or [container.ExecCreateResponse]. It will be removed in the next release.
type IDResponse = common.IDResponse

// ContainerJSONBase contains response of Engine API GET "/containers/{name:.*}/json"
// for API version 1.18 and older.
//
// Deprecated: use [container.InspectResponse] or [container.ContainerJSONBase]. It will be removed in the next release.
type ContainerJSONBase = container.ContainerJSONBase

// ContainerJSON is the response for the GET "/containers/{name:.*}/json"
// endpoint.
//
// Deprecated: use [container.InspectResponse]. It will be removed in the next release.
type ContainerJSON = container.InspectResponse

// Container contains response of Engine API:
// GET "/containers/json"
//
// Deprecated: use [container.Summary].
type Container = container.Summary

// ContainerState stores container's running state
//
// Deprecated: use [container.State].
type ContainerState = container.State

// NetworkSettings exposes the network settings in the api.
//
// Deprecated: use [container.NetworkSettings].
type NetworkSettings = container.NetworkSettings

// NetworkSettingsBase holds networking state for a container when inspecting it.
//
// Deprecated: use [container.NetworkSettingsBase].
type NetworkSettingsBase = container.NetworkSettingsBase

// DefaultNetworkSettings holds network information
// during the 2 release deprecation period.
// It will be removed in Docker 1.11.
//
// Deprecated: use [container.DefaultNetworkSettings].
type DefaultNetworkSettings = container.DefaultNetworkSettings

// SummaryNetworkSettings provides a summary of container's networks
// in /containers/json.
//
// Deprecated: use [container.NetworkSettingsSummary].
type SummaryNetworkSettings = container.NetworkSettingsSummary

// Health states
const (
	NoHealthcheck = container.NoHealthcheck // Deprecated: use [container.NoHealthcheck].
	Starting      = container.Starting      // Deprecated: use [container.Starting].
	Healthy       = container.Healthy       // Deprecated: use [container.Healthy].
	Unhealthy     = container.Unhealthy     // Deprecated: use [container.Unhealthy].
)

// Health stores information about the container's healthcheck results.
//
// Deprecated: use [container.Health].
type Health = container.Health

// HealthcheckResult stores information about a single run of a healthcheck probe.
//
// Deprecated: use [container.HealthcheckResult].
type HealthcheckResult = container.HealthcheckResult

// MountPoint represents a mount point configuration inside the container.
// This is used for reporting the mountpoints in use by a container.
//
// Deprecated: use [container.MountPoint].
type MountPoint = container.MountPoint

// Port An open port on a container
//
// Deprecated: use [container.Port].
type Port = container.Port

// GraphDriverData Information about the storage driver used to store the container's and
// image's filesystem.
//
// Deprecated: use [storage.DriverData].
type GraphDriverData = storage.DriverData

// RootFS returns Image's RootFS description including the layer IDs.
//
// Deprecated: use [image.RootFS].
type RootFS = image.RootFS

// ImageInspect contains response of Engine API:
// GET "/images/{name:.*}/json"
//
// Deprecated: use [image.InspectResponse].
type ImageInspect = image.InspectResponse

// RequestPrivilegeFunc is a function interface that clients can supply to
// retry operations after getting an authorization error.
// This function returns the registry authentication header value in base64
// format, or an error if the privilege request fails.
//
// Deprecated: moved to [github.com/docker/docker/api/types/registry.RequestAuthConfig].
type RequestPrivilegeFunc func(context.Context) (string, error)

// SecretCreateResponse contains the information returned to a client
// on the creation of a new secret.
//
// Deprecated: use [swarm.SecretCreateResponse].
type SecretCreateResponse = swarm.SecretCreateResponse

// SecretListOptions holds parameters to list secrets
//
// Deprecated: use [swarm.SecretListOptions].
type SecretListOptions = swarm.SecretListOptions

// ConfigCreateResponse contains the information returned to a client
// on the creation of a new config.
//
// Deprecated: use [swarm.ConfigCreateResponse].
type ConfigCreateResponse = swarm.ConfigCreateResponse

// ConfigListOptions holds parameters to list configs
//
// Deprecated: use [swarm.ConfigListOptions].
type ConfigListOptions = swarm.ConfigListOptions

// NodeListOptions holds parameters to list nodes with.
//
// Deprecated: use [swarm.NodeListOptions].
type NodeListOptions = swarm.NodeListOptions

// NodeRemoveOptions holds parameters to remove nodes with.
//
// Deprecated: use [swarm.NodeRemoveOptions].
type NodeRemoveOptions = swarm.NodeRemoveOptions

// TaskListOptions holds parameters to list tasks with.
//
// Deprecated: use [swarm.TaskListOptions].
type TaskListOptions = swarm.TaskListOptions

// ServiceCreateOptions contains the options to use when creating a service.
//
// Deprecated: use [swarm.ServiceCreateOptions].
type ServiceCreateOptions = swarm.ServiceCreateOptions

// ServiceUpdateOptions contains the options to be used for updating services.
//
// Deprecated: use [swarm.ServiceCreateOptions].
type ServiceUpdateOptions = swarm.ServiceUpdateOptions

const (
	RegistryAuthFromSpec         = swarm.RegistryAuthFromSpec         // Deprecated: use [swarm.RegistryAuthFromSpec].
	RegistryAuthFromPreviousSpec = swarm.RegistryAuthFromPreviousSpec // Deprecated: use [swarm.RegistryAuthFromPreviousSpec].
)

// ServiceListOptions holds parameters to list services with.
//
// Deprecated: use [swarm.ServiceListOptions].
type ServiceListOptions = swarm.ServiceListOptions

// ServiceInspectOptions holds parameters related to the "service inspect"
// operation.
//
// Deprecated: use [swarm.ServiceInspectOptions].
type ServiceInspectOptions = swarm.ServiceInspectOptions

// SwarmUnlockKeyResponse contains the response for Engine API:
// GET /swarm/unlockkey
//
// Deprecated: use [swarm.UnlockKeyResponse].
type SwarmUnlockKeyResponse = swarm.UnlockKeyResponse

// BuildCache contains information about a build cache record.
//
// Deprecated: deprecated in API 1.49. Use [build.CacheRecord] instead.
type BuildCache = build.CacheRecord

// BuildCachePruneOptions hold parameters to prune the build cache
//
// Deprecated: use [build.CachePruneOptions].
type BuildCachePruneOptions = build.CachePruneOptions

// BuildCachePruneReport contains the response for Engine API:
// POST "/build/prune"
//
// Deprecated: use [build.CachePruneReport].
type BuildCachePruneReport = build.CachePruneReport

// BuildResult contains the image id of a successful build/
//
// Deprecated: use [build.Result].
type BuildResult = build.Result

// ImageBuildOptions holds the information
// necessary to build images.
//
// Deprecated: use [build.ImageBuildOptions].
type ImageBuildOptions = build.ImageBuildOptions

// ImageBuildOutput defines configuration for exporting a build result
//
// Deprecated: use [build.ImageBuildOutput].
type ImageBuildOutput = build.ImageBuildOutput

// ImageBuildResponse holds information
// returned by a server after building
// an image.
//
// Deprecated: use [build.ImageBuildResponse].
type ImageBuildResponse = build.ImageBuildResponse

// BuilderVersion sets the version of underlying builder to use
//
// Deprecated: use [build.BuilderVersion].
type BuilderVersion = build.BuilderVersion

const (
	// BuilderV1 is the first generation builder in docker daemon
	//
	// Deprecated: use [build.BuilderV1].
	BuilderV1 = build.BuilderV1
	// BuilderBuildKit is builder based on moby/buildkit project
	//
	// Deprecated: use [build.BuilderBuildKit].
	BuilderBuildKit = build.BuilderBuildKit
)
