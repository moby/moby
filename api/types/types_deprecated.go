package types

import (
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/network"
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

// NetworkCreateResponse is the response message sent by the server for network create call.
//
// Deprecated: use [network.CreateResponse].
type NetworkCreateResponse = network.CreateResponse

// NetworkInspectOptions holds parameters to inspect network.
//
// Deprecated: use [network.InspectOptions].
type NetworkInspectOptions = network.InspectOptions

// NetworkConnect represents the data to be used to connect a container to the network
//
// Deprecated: use [network.ConnectOptions].
type NetworkConnect = network.ConnectOptions

// NetworkDisconnect represents the data to be used to disconnect a container from the network
//
// Deprecated: use [network.DisconnectOptions].
type NetworkDisconnect = network.DisconnectOptions

// EndpointResource contains network resources allocated and used for a container in a network.
//
// Deprecated: use [network.EndpointResource].
type EndpointResource = network.EndpointResource
