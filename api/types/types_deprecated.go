package types

import (
	"github.com/docker/docker/api/types/checkpoint"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/swarm"
	"github.com/docker/docker/api/types/system"
)

// CheckpointCreateOptions holds parameters to create a checkpoint from a container.
//
// Deprecated: use [checkpoint.CreateOptions].
type CheckpointCreateOptions = checkpoint.CreateOptions

// CheckpointListOptions holds parameters to list checkpoints for a container
//
// Deprecated: use [checkpoint.ListOptions].
type CheckpointListOptions = checkpoint.ListOptions

// CheckpointDeleteOptions holds parameters to delete a checkpoint from a container
//
// Deprecated: use [checkpoint.DeleteOptions].
type CheckpointDeleteOptions = checkpoint.DeleteOptions

// Checkpoint represents the details of a checkpoint when listing endpoints.
//
// Deprecated: use [checkpoint.Summary].
type Checkpoint = checkpoint.Summary

// Info contains response of Engine API:
// GET "/info"
//
// Deprecated: use [system.Info].
type Info = system.Info

// Commit holds the Git-commit (SHA1) that a binary was built from, as reported
// in the version-string of external tools, such as containerd, or runC.
//
// Deprecated: use [system.Commit].
type Commit = system.Commit

// PluginsInfo is a temp struct holding Plugins name
// registered with docker daemon. It is used by [system.Info] struct
//
// Deprecated: use [system.PluginsInfo].
type PluginsInfo = system.PluginsInfo

// NetworkAddressPool is a temp struct used by [system.Info] struct.
//
// Deprecated: use [system.NetworkAddressPool].
type NetworkAddressPool = system.NetworkAddressPool

// Runtime describes an OCI runtime.
//
// Deprecated: use [system.Runtime].
type Runtime = system.Runtime

// SecurityOpt contains the name and options of a security option.
//
// Deprecated: use [system.SecurityOpt].
type SecurityOpt = system.SecurityOpt

// KeyValue holds a key/value pair.
//
// Deprecated: use [system.KeyValue].
type KeyValue = system.KeyValue

// ImageDeleteResponseItem image delete response item.
//
// Deprecated: use [image.DeleteResponse].
type ImageDeleteResponseItem = image.DeleteResponse

// ImageSummary image summary.
//
// Deprecated: use [image.Summary].
type ImageSummary = image.Summary

// ImageMetadata contains engine-local data about the image.
//
// Deprecated: use [image.Metadata].
type ImageMetadata = image.Metadata

// ServiceCreateResponse contains the information returned to a client
// on the creation of a new service.
//
// Deprecated: use [swarm.ServiceCreateResponse].
type ServiceCreateResponse = swarm.ServiceCreateResponse

// ServiceUpdateResponse service update response.
//
// Deprecated: use [swarm.ServiceUpdateResponse].
type ServiceUpdateResponse = swarm.ServiceUpdateResponse

// ContainerStartOptions holds parameters to start containers.
//
// Deprecated: use [container.StartOptions].
type ContainerStartOptions = container.StartOptions

// ResizeOptions holds parameters to resize a TTY.
// It can be used to resize container TTYs and
// exec process TTYs too.
//
// Deprecated: use [container.ResizeOptions].
type ResizeOptions = container.ResizeOptions

// ContainerAttachOptions holds parameters to attach to a container.
//
// Deprecated: use [container.AttachOptions].
type ContainerAttachOptions = container.AttachOptions

// ContainerCommitOptions holds parameters to commit changes into a container.
//
// Deprecated: use [container.CommitOptions].
type ContainerCommitOptions = container.CommitOptions

// ContainerListOptions holds parameters to list containers with.
//
// Deprecated: use [container.ListOptions].
type ContainerListOptions = container.ListOptions

// ContainerLogsOptions holds parameters to filter logs with.
//
// Deprecated: use [container.LogsOptions].
type ContainerLogsOptions = container.LogsOptions

// ContainerRemoveOptions holds parameters to remove containers.
//
// Deprecated: use [container.RemoveOptions].
type ContainerRemoveOptions = container.RemoveOptions

// DecodeSecurityOptions decodes a security options string slice to a type safe
// [system.SecurityOpt].
//
// Deprecated: use [system.DecodeSecurityOptions].
func DecodeSecurityOptions(opts []string) ([]system.SecurityOpt, error) {
	return system.DecodeSecurityOptions(opts)
}
