package types

import (
	"context"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/plugin"
	"github.com/docker/docker/api/types/storage"
)

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

// Plugin A plugin for the Engine API
//
// Deprecated: use [plugin.Plugin].
type Plugin = plugin.Plugin

// PluginConfig The config of a plugin.
//
// Deprecated: use [plugin.PluginConfig].
type PluginConfig = plugin.Config

// PluginConfigArgs plugin config args
//
// Deprecated: use [plugin.PluginConfigInterface].
type PluginConfigArgs = plugin.Args

// PluginConfigInterface The interface between Docker and the plugin
//
// Deprecated: use [plugin.Interface].
type PluginConfigInterface = plugin.Interface

// PluginInterfaceType plugin interface type
//
// Deprecated: use [plugin.InterfaceType].
type PluginInterfaceType = plugin.InterfaceType

// PluginConfigLinux plugin config linux
//
// Deprecated: use [plugin.LinuxConfig].
type PluginConfigLinux = plugin.LinuxConfig

// PluginConfigNetwork plugin config network
//
// Deprecated: use [plugin.NetworkConfig].
type PluginConfigNetwork = plugin.NetworkConfig

// PluginConfigRootfs plugin config rootfs
//
// Deprecated: use [plugin.RootFS].
type PluginConfigRootfs = plugin.RootFS

// PluginConfigUser plugin config user
//
// Deprecated: use [plugin.User].
type PluginConfigUser = plugin.User

// PluginSettings Settings that can be modified by users.
//
// Deprecated: use [plugin.Settings].
type PluginSettings = plugin.Settings

// PluginDevice plugin device
//
// Deprecated: use [plugin.Device].
type PluginDevice = plugin.Device

// PluginMount plugin mount
//
// Deprecated: use [plugin.Mount].
type PluginMount = plugin.Mount

// PluginEnv plugin env
//
// Deprecated: use [plugin.Env].
type PluginEnv = plugin.Env

// PluginListResponse contains the response for the Engine API
//
// Deprecated: use [plugin.ListResponse].
type PluginListResponse = plugin.ListResponse

// PluginPrivilege describes a permission the user has to accept
// upon installing a plugin.
//
// Deprecated: use [plugin.Privilege].
type PluginPrivilege = plugin.Privilege

// PluginPrivileges is a list of [plugin.Privileges]
//
// Deprecated: use [plugin.Privileges].
type PluginPrivileges = plugin.Privileges

// PluginRemoveOptions holds parameters to remove plugins.
//
// Deprecated: use [plugin.RemoveOptions].
type PluginRemoveOptions = plugin.RemoveOptions

// PluginEnableOptions holds parameters to enable plugins.
//
// Deprecated: use [plugin.EnableOptions].
type PluginEnableOptions = plugin.EnableOptions

// PluginDisableOptions holds parameters to disable plugins.
//
// Deprecated: use [plugin.DisableOptions].
type PluginDisableOptions = plugin.DisableOptions

// PluginInstallOptions holds parameters to install a plugin.
//
// Deprecated: use [plugin.InstallOptions].
type PluginInstallOptions = plugin.InstallOptions

// PluginCreateOptions hold all options to plugin create.
//
// Deprecated: use [plugin.CreateOptions].
type PluginCreateOptions = plugin.CreateOptions
