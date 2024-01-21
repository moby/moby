package types

import (
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
)

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

// ImageImportOptions holds information to import images from the client host.
//
// Deprecated: use [image.ImportOptions].
type ImageImportOptions = image.ImportOptions

// ImageCreateOptions holds information to create images.
//
// Deprecated: use [image.CreateOptions].
type ImageCreateOptions = image.CreateOptions

// ImagePullOptions holds information to pull images.
//
// Deprecated: use [image.PullOptions].
type ImagePullOptions = image.PullOptions

// ImagePushOptions holds information to push images.
//
// Deprecated: use [image.PushOptions].
type ImagePushOptions = image.PushOptions

// ImageListOptions holds parameters to list images with.
//
// Deprecated: use [image.ListOptions].
type ImageListOptions = image.ListOptions

// ImageRemoveOptions holds parameters to remove images.
//
// Deprecated: use [image.RemoveOptions].
type ImageRemoveOptions = image.RemoveOptions
