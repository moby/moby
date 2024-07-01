package types

import (
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/events"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/api/types/registry"
	"github.com/docker/docker/api/types/storage"
)

// NetworkCreate is the expected body of the "create network" http request message
//
// Deprecated: use [network.CreateOptions].
type NetworkCreate = network.CreateOptions

// NetworkListOptions holds parameters to filter the list of networks with.
//
// Deprecated: use [network.ListOptions].
type NetworkListOptions = network.ListOptions

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

// NetworkResource is the body of the "get network" http response message/
//
// Deprecated: use [network.Inspect] or [network.Summary] (for list operations).
type NetworkResource = network.Inspect

// NetworksPruneReport contains the response for Engine API:
// POST "/networks/prune"
//
// Deprecated: use [network.PruneReport].
type NetworksPruneReport = network.PruneReport

// ExecConfig is a small subset of the Config struct that holds the configuration
// for the exec feature of docker.
//
// Deprecated: use [container.ExecOptions].
type ExecConfig = container.ExecOptions

// ExecStartCheck is a temp struct used by execStart
// Config fields is part of ExecConfig in runconfig package
//
// Deprecated: use [container.ExecStartOptions] or [container.ExecAttachOptions].
type ExecStartCheck = container.ExecStartOptions

// ContainerExecInspect holds information returned by exec inspect.
//
// Deprecated: use [container.ExecInspect].
type ContainerExecInspect = container.ExecInspect

// ContainersPruneReport contains the response for Engine API:
// POST "/containers/prune"
//
// Deprecated: use [container.PruneReport].
type ContainersPruneReport = container.PruneReport

// ContainerPathStat is used to encode the header from
// GET "/containers/{name:.*}/archive"
// "Name" is the file or directory name.
//
// Deprecated: use [container.PathStat].
type ContainerPathStat = container.PathStat

// CopyToContainerOptions holds information
// about files to copy into a container.
//
// Deprecated: use [container.CopyToContainerOptions],
type CopyToContainerOptions = container.CopyToContainerOptions

// ContainerStats contains response of Engine API:
// GET "/stats"
//
// Deprecated: use [container.StatsResponseReader].
type ContainerStats = container.StatsResponseReader

// ThrottlingData stores CPU throttling stats of one running container.
// Not used on Windows.
//
// Deprecated: use [container.ThrottlingData].
type ThrottlingData = container.ThrottlingData

// CPUUsage stores All CPU stats aggregated since container inception.
//
// Deprecated: use [container.CPUUsage].
type CPUUsage = container.CPUUsage

// CPUStats aggregates and wraps all CPU related info of container
//
// Deprecated: use [container.CPUStats].
type CPUStats = container.CPUStats

// MemoryStats aggregates all memory stats since container inception on Linux.
// Windows returns stats for commit and private working set only.
//
// Deprecated: use [container.MemoryStats].
type MemoryStats = container.MemoryStats

// BlkioStatEntry is one small entity to store a piece of Blkio stats
// Not used on Windows.
//
// Deprecated: use [container.BlkioStatEntry].
type BlkioStatEntry = container.BlkioStatEntry

// BlkioStats stores All IO service stats for data read and write.
// This is a Linux specific structure as the differences between expressing
// block I/O on Windows and Linux are sufficiently significant to make
// little sense attempting to morph into a combined structure.
//
// Deprecated: use [container.BlkioStats].
type BlkioStats = container.BlkioStats

// StorageStats is the disk I/O stats for read/write on Windows.
//
// Deprecated: use [container.StorageStats].
type StorageStats = container.StorageStats

// NetworkStats aggregates the network stats of one container
//
// Deprecated: use [container.NetworkStats].
type NetworkStats = container.NetworkStats

// PidsStats contains the stats of a container's pids
//
// Deprecated: use [container.PidsStats].
type PidsStats = container.PidsStats

// Stats is Ultimate struct aggregating all types of stats of one container
//
// Deprecated: use [container.Stats].
type Stats = container.Stats

// StatsJSON is newly used Networks
//
// Deprecated: use [container.StatsResponse].
type StatsJSON = container.StatsResponse

// EventsOptions holds parameters to filter events with.
//
// Deprecated: use [events.ListOptions].
type EventsOptions = events.ListOptions

// ImageSearchOptions holds parameters to search images with.
//
// Deprecated: use [registry.SearchOptions].
type ImageSearchOptions = registry.SearchOptions

// ImageImportSource holds source information for ImageImport
//
// Deprecated: use [image.ImportSource].
type ImageImportSource image.ImportSource

// ImageLoadResponse returns information to the client about a load process.
//
// Deprecated: use [image.LoadResponse].
type ImageLoadResponse = image.LoadResponse

// ContainerJSONBase contains response of Engine API GET "/containers/{name:.*}/json"
// for API version 1.18 and older.
//
// Deprecated: use [container.InspectResponse] or [container.InspectBase]. It will be removed in the next release.
type ContainerJSONBase = container.InspectBase

// ContainerJSON is the response for the GET "/containers/{name:.*}/json"
// endpoint.
//
// Deprecated: use [container.InspectResponse]. It will be removed in the next release.
type ContainerJSON = container.InspectResponse

// ContainerNode stores information about the node that a container
// is running on.  It's only used by the Docker Swarm standalone API.
//
// Deprecated: ContainerNode was used for the classic Docker Swarm standalone API. It will be removed in the next release.
type ContainerNode = container.ContainerNode //nolint:staticcheck // Ignore SA1019: container.ContainerNode is deprecated.

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
