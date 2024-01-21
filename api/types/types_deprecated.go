package types

import (
	"github.com/docker/docker/api/types/image"
)

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
