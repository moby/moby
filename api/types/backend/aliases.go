// Package backend includes types to send information to server backends.
package backend

import "github.com/moby/moby/api/types/backend"

// ContainerCreateConfig is the parameter set to ContainerCreate()
type ContainerCreateConfig = backend.ContainerCreateConfig

// ContainerRmConfig holds arguments for the container remove
// operation. This struct is used to tell the backend what operations
// to perform.
type ContainerRmConfig = backend.ContainerRmConfig

// ContainerAttachConfig holds the streams to use when connecting to a container to view logs.
type ContainerAttachConfig = backend.ContainerAttachConfig

// PartialLogMetaData provides meta data for a partial log message. Messages
// exceeding a predefined size are split into chunks with this metadata. The
// expectation is for the logger endpoints to assemble the chunks using this
// metadata.
type PartialLogMetaData = backend.PartialLogMetaData

// LogMessage is datastructure that represents piece of output produced by some
// container.  The Line member is a slice of an array whose contents can be
// changed after a log driver's Log() method returns.
type LogMessage = backend.LogMessage

// LogAttr is used to hold the extra attributes available in the log message.
type LogAttr = backend.LogAttr

// LogSelector is a list of services and tasks that should be returned as part
// of a log stream. It is similar to swarmapi.LogSelector, with the difference
// that the names don't have to be resolved to IDs; this is mostly to avoid
// accidents later where a swarmapi LogSelector might have been incorrectly
// used verbatim (and to avoid the handler having to import swarmapi types)
type LogSelector = backend.LogSelector

// ContainerStatsConfig holds information for configuring the runtime
// behavior of a backend.ContainerStats() call.
type ContainerStatsConfig = backend.ContainerStatsConfig

// ContainerInspectOptions defines options for the backend.ContainerInspect
// call.
type ContainerInspectOptions = backend.ContainerInspectOptions

// ExecStartConfig holds the options to start container's exec.
type ExecStartConfig = backend.ExecStartConfig

// ExecInspect holds information about a running process started
// with docker exec.
type ExecInspect = backend.ExecInspect

// ExecProcessConfig holds information about the exec process
// running on the host.
type ExecProcessConfig = backend.ExecProcessConfig

// CreateImageConfig is the configuration for creating an image from a
// container.
type CreateImageConfig = backend.CreateImageConfig

// GetImageOpts holds parameters to retrieve image information
// from the backend.
type GetImageOpts = backend.GetImageOpts

// ImageInspectOpts holds parameters to inspect an image.
type ImageInspectOpts = backend.ImageInspectOpts

// CommitConfig is the configuration for creating an image as part of a build.
type CommitConfig = backend.CommitConfig

// PluginRmConfig holds arguments for plugin remove.
type PluginRmConfig = backend.PluginRmConfig

// PluginEnableConfig holds arguments for plugin enable
type PluginEnableConfig = backend.PluginEnableConfig

// PluginDisableConfig holds arguments for plugin disable.
type PluginDisableConfig = backend.PluginDisableConfig

// NetworkListConfig stores the options available for listing networks
type NetworkListConfig = backend.NetworkListConfig

// PullOption defines different modes for accessing images
type PullOption = backend.PullOption

const (
	// PullOptionNoPull only returns local images
	PullOptionNoPull = backend.PullOptionNoPull
	// PullOptionForcePull always tries to pull a ref from the registry first
	PullOptionForcePull = backend.PullOptionForcePull
	// PullOptionPreferLocal uses local image if it exists, otherwise pulls
	PullOptionPreferLocal = backend.PullOptionPreferLocal
)

// ProgressWriter is a data object to transport progress streams to the client
type ProgressWriter = backend.ProgressWriter

// AuxEmitter is an interface for emitting aux messages during build progress
type AuxEmitter = backend.AuxEmitter

// BuildConfig is the configuration used by a BuildManager to start a build
type BuildConfig = backend.BuildConfig

// GetImageAndLayerOptions are the options supported by GetImageAndReleasableLayer
type GetImageAndLayerOptions = backend.GetImageAndLayerOptions
