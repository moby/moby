package types

import (
	"net"

	"github.com/moby/moby/api/types"
)

// NewHijackedResponse initializes a [HijackedResponse] type.
func NewHijackedResponse(conn net.Conn, mediaType string) types.HijackedResponse {
	return types.NewHijackedResponse(conn, mediaType)
}

// HijackedResponse holds connection information for a hijacked request.
type HijackedResponse = types.HijackedResponse

// CloseWriter is an interface that implements structs
// that close input streams to prevent from writing.
type CloseWriter = types.CloseWriter

// PluginRemoveOptions holds parameters to remove plugins.
type PluginRemoveOptions = types.PluginRemoveOptions

// PluginEnableOptions holds parameters to enable plugins.
type PluginEnableOptions = types.PluginEnableOptions

// PluginDisableOptions holds parameters to disable plugins.
type PluginDisableOptions = types.PluginDisableOptions

// PluginInstallOptions holds parameters to install a plugin.
type PluginInstallOptions = types.PluginInstallOptions

// PluginCreateOptions hold all options to plugin create.
type PluginCreateOptions = types.PluginCreateOptions

// ErrorResponse Represents an error.
type ErrorResponse = types.ErrorResponse

// Plugin A plugin for the Engine API
type Plugin = types.Plugin

// PluginConfig The config of a plugin.
type PluginConfig = types.PluginConfig

// PluginConfigArgs plugin config args
type PluginConfigArgs = types.PluginConfigArgs

// PluginConfigInterface The interface between Docker and the plugin
type PluginConfigInterface = types.PluginConfigInterface

// PluginConfigLinux plugin config linux
type PluginConfigLinux = types.PluginConfigLinux

// PluginConfigNetwork plugin config network
type PluginConfigNetwork = types.PluginConfigNetwork

// PluginConfigRootfs plugin config rootfs
type PluginConfigRootfs = types.PluginConfigRootfs

// PluginConfigUser plugin config user
type PluginConfigUser = types.PluginConfigUser

// PluginSettings Settings that can be modified by users.
type PluginSettings = types.PluginSettings

// PluginDevice plugin device
type PluginDevice = types.PluginDevice

// PluginEnv plugin env
type PluginEnv = types.PluginEnv

// PluginInterfaceType plugin interface type
type PluginInterfaceType = types.PluginInterfaceType

// PluginMount plugin mount
type PluginMount = types.PluginMount

// PluginsListResponse contains the response for the Engine API
type PluginsListResponse = types.PluginsListResponse

// PluginPrivilege describes a permission the user has to accept
// upon installing a plugin.
type PluginPrivilege = types.PluginPrivilege

// PluginPrivileges is a list of PluginPrivilege
type PluginPrivileges = types.PluginPrivileges

const (
	// MediaTypeRawStream is vendor specific MIME-Type set for raw TTY streams
	MediaTypeRawStream = types.MediaTypeRawStream

	// MediaTypeMultiplexedStream is vendor specific MIME-Type set for stdin/stdout/stderr multiplexed streams
	MediaTypeMultiplexedStream = types.MediaTypeMultiplexedStream
)

// Ping contains response of Engine API:
// GET "/_ping"
type Ping = types.Ping

// ComponentVersion describes the version information for a specific component.
type ComponentVersion = types.ComponentVersion

// Version contains response of Engine API:
// GET "/version"
type Version = types.Version

// DiskUsageObject represents an object type used for disk usage query filtering.
type DiskUsageObject = types.DiskUsageObject

const (
	// ContainerObject represents a container DiskUsageObject.
	ContainerObject = types.ContainerObject
	// ImageObject represents an image DiskUsageObject.
	ImageObject = types.ImageObject
	// VolumeObject represents a volume DiskUsageObject.
	VolumeObject = types.VolumeObject
	// BuildCacheObject represents a build-cache DiskUsageObject.
	BuildCacheObject = types.BuildCacheObject
)

// DiskUsageOptions holds parameters for system disk usage query.
type DiskUsageOptions = types.DiskUsageOptions

// DiskUsage contains response of Engine API:
// GET "/system/df"
type DiskUsage = types.DiskUsage

// PushResult contains the tag, manifest digest, and manifest size from the
// push. It's used to signal this information to the trust code in the client
// so it can sign the manifest if necessary.
type PushResult = types.PushResult
