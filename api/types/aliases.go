package types

import (
	"net"

	"github.com/moby/moby/api/types"
	"github.com/moby/moby/api/types/common"
	"github.com/moby/moby/api/types/plugin"
	"github.com/moby/moby/api/types/system"
	"github.com/moby/moby/client"
)

// NewHijackedResponse initializes a [HijackedResponse] type.
func NewHijackedResponse(conn net.Conn, mediaType string) client.HijackedResponse {
	return client.NewHijackedResponse(conn, mediaType)
}

// HijackedResponse holds connection information for a hijacked request.
type HijackedResponse = client.HijackedResponse

// CloseWriter is an interface that implements structs
// that close input streams to prevent from writing.
type CloseWriter = client.CloseWriter

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

// ErrorResponse Represents an error.
type ErrorResponse = common.ErrorResponse

// Plugin A plugin for the Engine API
type Plugin = plugin.Plugin

// PluginConfig The config of a plugin.
type PluginConfig = plugin.Config

// PluginConfigArgs plugin config args
type PluginConfigArgs = plugin.Args

// PluginConfigInterface The interface between Docker and the plugin
type PluginConfigInterface = plugin.Interface

// PluginConfigLinux plugin config linux
type PluginConfigLinux = plugin.LinuxConfig

// PluginConfigNetwork plugin config network
type PluginConfigNetwork = plugin.NetworkConfig

// PluginConfigRootfs plugin config rootfs
type PluginConfigRootfs = plugin.RootFS

// PluginConfigUser plugin config user
type PluginConfigUser = plugin.User

// PluginSettings Settings that can be modified by users.
type PluginSettings = plugin.Settings

// PluginDevice plugin device
type PluginDevice = plugin.Device

// PluginEnv plugin env
type PluginEnv = plugin.Env

// PluginInterfaceType plugin interface type
type PluginInterfaceType = plugin.CapabilityID

// PluginMount plugin mount
type PluginMount = plugin.Mount

// PluginsListResponse contains the response for the Engine API
type PluginsListResponse = plugin.ListResponse

// PluginPrivilege describes a permission the user has to accept
// upon installing a plugin.
type PluginPrivilege = plugin.Privilege

// PluginPrivileges is a list of PluginPrivilege
type PluginPrivileges = plugin.Privileges

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
type DiskUsageObject = system.DiskUsageObject

const (
	// ContainerObject represents a container DiskUsageObject.
	ContainerObject = system.ContainerObject
	// ImageObject represents an image DiskUsageObject.
	ImageObject = system.ImageObject
	// VolumeObject represents a volume DiskUsageObject.
	VolumeObject = system.VolumeObject
	// BuildCacheObject represents a build-cache DiskUsageObject.
	BuildCacheObject = system.BuildCacheObject
)

// DiskUsageOptions holds parameters for system disk usage query.
type DiskUsageOptions = client.DiskUsageOptions

// DiskUsage contains response of Engine API:
// GET "/system/df"
type DiskUsage = system.DiskUsage

// PushResult contains the tag, manifest digest, and manifest size from the
// push. It's used to signal this information to the trust code in the client
// so it can sign the manifest if necessary.
type PushResult = types.PushResult
