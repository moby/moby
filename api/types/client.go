package types // import "github.com/docker/docker/api/types"

import (
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/swarm"
	"github.com/docker/docker/client"
)

// SecretListOptions holds parameters to list secrets
type SecretListOptions = client.SecretListOptions

// ConfigListOptions holds parameters to list configs
type ConfigListOptions = client.ConfigListOptions

// NetworkInspectOptions holds parameters to inspect network
type NetworkInspectOptions = client.NetworkInspectOptions

// CheckpointCreateOptions holds parameters to create a checkpoint from a container
type CheckpointCreateOptions = client.CheckpointCreateOptions

// CheckpointListOptions holds parameters to list checkpoints for a container
type CheckpointListOptions = client.CheckpointListOptions

// CheckpointDeleteOptions holds parameters to delete a checkpoint from a container
type CheckpointDeleteOptions = client.CheckpointDeleteOptions

// ContainerAttachOptions holds parameters to attach to a container.
type ContainerAttachOptions = client.ContainerAttachOptions

// ContainerCommitOptions holds parameters to commit changes into a container.
type ContainerCommitOptions = client.ContainerCommitOptions

// ContainerListOptions holds parameters to list containers with.
type ContainerListOptions = client.ContainerListOptions

// ContainerLogsOptions holds parameters to filter logs with.
type ContainerLogsOptions = client.ContainerLogsOptions

// ContainerRemoveOptions holds parameters to remove containers.
type ContainerRemoveOptions = client.ContainerRemoveOptions

// ContainerStartOptions holds parameters to start containers.
type ContainerStartOptions = client.ContainerStartOptions

// CopyToContainerOptions holds information about files to copy into a container
type CopyToContainerOptions = client.CopyToContainerOptions

// EventsOptions holds parameters to filter events with.
type EventsOptions = client.EventsOptions

// NetworkListOptions holds parameters to filter the list of networks with.
type NetworkListOptions = client.NetworkListOptions

// HijackedResponse holds connection information for a hijacked request.
type HijackedResponse = client.HijackedResponse

// CloseWriter is an interface that implements structs
// that close input streams to prevent from writing.
type CloseWriter = client.CloseWriter

// ImageBuildOptions holds the information necessary to build images.
type ImageBuildOptions = client.ImageBuildOptions

// ImageCreateOptions holds information to create images.
type ImageCreateOptions = client.ImageCreateOptions

// ImageImportOptions holds information to import images from the client host.
type ImageImportOptions = client.ImageImportOptions

// ImageListOptions holds parameters to filter the list of images with.
type ImageListOptions = client.ImageListOptions

// ImagePullOptions holds information to pull images.
type ImagePullOptions = client.ImagePullOptions

// RequestPrivilegeFunc is a function interface that
// clients can supply to retry operations after
// getting an authorization error.
type RequestPrivilegeFunc = client.RequestPrivilegeFunc

//ImagePushOptions holds information to push images.
type ImagePushOptions = client.ImagePushOptions

// ImageRemoveOptions holds parameters to remove images.
type ImageRemoveOptions = client.ImageRemoveOptions

// ImageSearchOptions holds parameters to search images with.
type ImageSearchOptions = client.ImageSearchOptions

// ResizeOptions holds parameters to resize a tty.
type ResizeOptions = client.ResizeOptions

// NodeListOptions holds parameters to list nodes with.
type NodeListOptions = client.NodeListOptions

// NodeRemoveOptions holds parameters to remove nodes with.
type NodeRemoveOptions = client.NodeRemoveOptions

// ServiceCreateOptions contains the options to use when creating a service.
type ServiceCreateOptions = client.ServiceCreateOptions

// ServiceUpdateOptions contains the options to be used for updating services.
type ServiceUpdateOptions = client.ServiceUpdateOptions

// ServiceListOptions holds parameters to list services with.
type ServiceListOptions = client.ServiceListOptions

// ServiceInspectOptions holds parameters related to the "service inspect" operation.
type ServiceInspectOptions = client.ServiceInspectOptions

// TaskListOptions holds parameters to list tasks with.
type TaskListOptions = client.TaskListOptions

// PluginRemoveOptions holds parameters to remove plugins.
type PluginRemoveOptions = client.PluginRemoveOptions

// PluginEnableOptions holds parameters to enable plugins.
type PluginEnableOptions = client.PluginEnableOptions

// PluginDisableOptions holds parameters to disable plugins.
type PluginDisableOptions = client.PluginDisableOptions

// PluginInstallOptions holds parameters to install a plugin.
type PluginInstallOptions = client.PluginInstallOptions

// PluginCreateOptions hold all options to plugin create.
type PluginCreateOptions = client.PluginCreateOptions

// ContainerExecInspect holds information returned by exec inspect.
type ContainerExecInspect = container.ExecInspect

// ImageBuildResponse holds information
// returned by a server after building
// an image.
type ImageBuildResponse = image.BuildResponse

// ImageImportSource holds source information for ImageImport
type ImageImportSource = image.ImportSource

// ImageLoadResponse returns information to the client about a load process.
type ImageLoadResponse = image.LoadResponse

// ServiceCreateResponse contains the information returned to a client
// on the creation of a new service.
type ServiceCreateResponse = swarm.ServiceCreateResponse

// Values for RegistryAuthFrom in ServiceUpdateOptions
const (
	RegistryAuthFromSpec         = "spec"
	RegistryAuthFromPreviousSpec = "previous-spec"
)

// SwarmUnlockKeyResponse contains the response for Engine API:
// GET /swarm/unlockkey
type SwarmUnlockKeyResponse = swarm.UnlockKeyResponse
