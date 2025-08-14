package container

import (
	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/client"
)

// ChangeType Kind of change
//
// Can be one of:
//
// - `0`: Modified ("C")
// - `1`: Added ("A")
// - `2`: Deleted ("D")
//
// swagger:model ChangeType
type ChangeType = container.ChangeType

const (
	// ChangeModify represents the modify operation.
	ChangeModify = container.ChangeModify
	// ChangeAdd represents the add operation.
	ChangeAdd = container.ChangeAdd
	// ChangeDelete represents the delete operation.
	ChangeDelete = container.ChangeDelete
)

// CommitResponse response for the commit API call, containing the ID of the
// image that was produced.
type CommitResponse = container.CommitResponse

// MinimumDuration puts a minimum on user configured duration.
// This is to prevent API error on time unit. For example, API may
// set 3 as healthcheck interval with intention of 3 seconds, but
// Docker interprets it as 3 nanoseconds.
const MinimumDuration = container.MinimumDuration

// StopOptions holds the options to stop or restart a container.
type StopOptions = container.StopOptions

// HealthConfig holds configuration settings for the HEALTHCHECK feature.
type HealthConfig = container.HealthConfig

// Config contains the configuration data about a container.
// It should hold only portable information about the container.
// Here, "portable" means "independent from the host we are running on".
// Non-portable information *should* appear in HostConfig.
// All fields added to this struct must be marked `omitempty` to keep getting
// predictable hashes from the old `v1Compatibility` configuration.
type Config = container.Config

// ContainerUpdateOKBody OK response to ContainerUpdate operation
//
// Deprecated: use [container.UpdateResponse]. This alias will be removed in the next release.
type ContainerUpdateOKBody = container.UpdateResponse

// ContainerTopOKBody OK response to ContainerTop operation
//
// Deprecated: use [container.TopResponse]. This alias will be removed in the next release.
type ContainerTopOKBody = container.TopResponse

// PruneReport contains the response for Engine API:
// POST "/containers/prune"
type PruneReport = container.PruneReport

// PathStat is used to encode the header from
// GET "/containers/{name:.*}/archive"
// "Name" is the file or directory name.
type PathStat = container.PathStat

// CopyToContainerOptions holds information
// about files to copy into a container
type CopyToContainerOptions = container.CopyToContainerOptions

// StatsResponseReader wraps an io.ReadCloser to read (a stream of) stats
// for a container, as produced by the GET "/stats" endpoint.
//
// The OSType field is set to the server's platform to allow
// platform-specific handling of the response.
//
// TODO(thaJeztah): remove this wrapper, and make OSType part of [StatsResponse].
type StatsResponseReader = client.StatsResponseReader

// MountPoint represents a mount point configuration inside the container.
// This is used for reporting the mountpoints in use by a container.
type MountPoint = container.MountPoint

// State stores container's running state
// it's part of ContainerJSONBase and returned by "inspect" command
type State = container.State

// Summary contains response of Engine API:
// GET "/containers/json"
type Summary = container.Summary

// ContainerJSONBase contains response of Engine API GET "/containers/{name:.*}/json"
// for API version 1.18 and older.
//
// TODO(thaJeztah): combine ContainerJSONBase and InspectResponse into a single struct.
// The split between ContainerJSONBase (ContainerJSONBase) and InspectResponse (InspectResponse)
// was done in commit 6deaa58ba5f051039643cedceee97c8695e2af74 (https://github.com/moby/moby/pull/13675).
// ContainerJSONBase contained all fields for API < 1.19, and InspectResponse
// held fields that were added in API 1.19 and up. Given that the minimum
// supported API version is now 1.24, we no longer use the separate type.
type ContainerJSONBase = container.ContainerJSONBase

// InspectResponse is the response for the GET "/containers/{name:.*}/json"
// endpoint.
type InspectResponse = container.InspectResponse

// CreateRequest is the request message sent to the server for container
// create calls. It is a config wrapper that holds the container [Config]
// (portable) and the corresponding [HostConfig] (non-portable) and
// [network.NetworkingConfig].
type CreateRequest = container.CreateRequest

// CreateResponse ContainerCreateResponse
//
// OK response to ContainerCreate operation
// swagger:model CreateResponse
type CreateResponse = container.CreateResponse

// DiskUsage contains disk usage for containers.
type DiskUsage = container.DiskUsage

// ExecCreateResponse is the response for a successful exec-create request.
// It holds the ID of the exec that was created.
//
// TODO(thaJeztah): make this a distinct type.
type ExecCreateResponse = container.ExecCreateResponse

// ExecOptions is a small subset of the Config struct that holds the configuration
// for the exec feature of docker.
type ExecOptions = container.ExecOptions

// ExecStartOptions is a temp struct used by execStart
// Config fields is part of ExecConfig in runconfig package
type ExecStartOptions = container.ExecStartOptions

// ExecAttachOptions is a temp struct used by execAttach.
//
// TODO(thaJeztah): make this a separate type; ContainerExecAttach does not use the Detach option, and cannot run detached.
type ExecAttachOptions = container.ExecAttachOptions

// ExecInspect holds information returned by exec inspect.
type ExecInspect = container.ExecInspect

// FilesystemChange Change in the container's filesystem.
type FilesystemChange = container.FilesystemChange

// HealthStatus is a string representation of the container's health.
//
// It currently is an alias for string, but may become a distinct type in future.
type HealthStatus = container.HealthStatus

// Health states
const (
	NoHealthcheck = container.NoHealthcheck // Indicates there is no healthcheck
	Starting      = container.Starting      // Starting indicates that the container is not yet ready
	Healthy       = container.Healthy       // Healthy indicates that the container is running correctly
	Unhealthy     = container.Unhealthy     // Unhealthy indicates that the container has a problem
)

// Health stores information about the container's healthcheck results
type Health = container.Health

// HealthcheckResult stores information about a single run of a healthcheck probe
type HealthcheckResult = container.HealthcheckResult

// ValidateHealthStatus checks if the provided string is a valid
// container [container.HealthStatus].
func ValidateHealthStatus(s container.HealthStatus) error {
	return container.ValidateHealthStatus(s)
}

// CgroupnsMode represents the cgroup namespace mode of the container
type CgroupnsMode = container.CgroupnsMode

// cgroup namespace modes for containers
const (
	CgroupnsModeEmpty   = container.CgroupnsModeEmpty
	CgroupnsModePrivate = container.CgroupnsModePrivate
	CgroupnsModeHost    = container.CgroupnsModeHost
)

// Isolation represents the isolation technology of a container. The supported
// values are platform specific
type Isolation = container.Isolation

// Isolation modes for containers
const (
	IsolationEmpty   = container.IsolationEmpty   // IsolationEmpty is unspecified (same behavior as default)
	IsolationDefault = container.IsolationDefault // IsolationDefault is the default isolation mode on current daemon
	IsolationProcess = container.IsolationProcess // IsolationProcess is process isolation mode
	IsolationHyperV  = container.IsolationHyperV  // IsolationHyperV is HyperV isolation mode
)

// IpcMode represents the container ipc stack.
type IpcMode = container.IpcMode

// IpcMode constants
const (
	IPCModeNone      = container.IPCModeNone
	IPCModeHost      = container.IPCModeHost
	IPCModeContainer = container.IPCModeContainer
	IPCModePrivate   = container.IPCModePrivate
	IPCModeShareable = container.IPCModeShareable
)

// NetworkMode represents the container network stack.
type NetworkMode = container.NetworkMode

// UsernsMode represents userns mode in the container.
type UsernsMode = container.UsernsMode

// CgroupSpec represents the cgroup to use for the container.
type CgroupSpec = container.CgroupSpec

// UTSMode represents the UTS namespace of the container.
type UTSMode = container.UTSMode

// PidMode represents the pid namespace of the container.
type PidMode = container.PidMode

// DeviceRequest represents a request for devices from a device driver.
// Used by GPU device drivers.
type DeviceRequest = container.DeviceRequest

// DeviceMapping represents the device mapping between the host and the container.
type DeviceMapping = container.DeviceMapping

// RestartPolicy represents the restart policies of the container.
type RestartPolicy = container.RestartPolicy

type RestartPolicyMode = container.RestartPolicyMode

const (
	RestartPolicyDisabled      = container.RestartPolicyDisabled
	RestartPolicyAlways        = container.RestartPolicyAlways
	RestartPolicyOnFailure     = container.RestartPolicyOnFailure
	RestartPolicyUnlessStopped = container.RestartPolicyUnlessStopped
)

// ValidateRestartPolicy validates the given RestartPolicy.
func ValidateRestartPolicy(policy container.RestartPolicy) error {
	return container.ValidateRestartPolicy(policy)
}

// LogMode is a type to define the available modes for logging
// These modes affect how logs are handled when log messages start piling up.
type LogMode = container.LogMode

// Available logging modes
const (
	LogModeUnset    = container.LogModeUnset
	LogModeBlocking = container.LogModeBlocking
	LogModeNonBlock = container.LogModeNonBlock
)

// LogConfig represents the logging configuration of the container.
type LogConfig = container.LogConfig

// Ulimit is an alias for [units.Ulimit], which may be moving to a different
// location or become a local type. This alias is to help transitioning.
//
// Users are recommended to use this alias instead of using [units.Ulimit] directly.
type Ulimit = container.Ulimit

// Resources contains container's resources (cgroups config, ulimits...)
type Resources = container.Resources

// UpdateConfig holds the mutable attributes of a Container.
// Those attributes can be updated at runtime.
type UpdateConfig = container.UpdateConfig

// HostConfig the non-portable Config structure of a container.
// Here, "non-portable" means "dependent of the host we are running on".
// Portable information *should* appear in Config.
type HostConfig = container.HostConfig

// NetworkSettings exposes the network settings in the api
type NetworkSettings = container.NetworkSettings

// NetworkSettingsBase holds networking state for a container when inspecting it.
type NetworkSettingsBase = container.NetworkSettingsBase

// DefaultNetworkSettings holds network information
// during the 2 release deprecation period.
// It will be removed in Docker 1.11.
type DefaultNetworkSettings = container.DefaultNetworkSettings

// NetworkSettingsSummary provides a summary of container's networks
// in /containers/json
type NetworkSettingsSummary = container.NetworkSettingsSummary

// ResizeOptions holds parameters to resize a TTY.
// It can be used to resize container TTYs and
// exec process TTYs too.
type ResizeOptions = container.ResizeOptions

// AttachOptions holds parameters to attach to a container.
type AttachOptions = container.AttachOptions

// CommitOptions holds parameters to commit changes into a container.
type CommitOptions = container.CommitOptions

// RemoveOptions holds parameters to remove containers.
type RemoveOptions = container.RemoveOptions

// StartOptions holds parameters to start containers.
type StartOptions = container.StartOptions

// ListOptions holds parameters to list containers with.
type ListOptions = container.ListOptions

// LogsOptions holds parameters to filter logs with.
type LogsOptions = container.LogsOptions

// Port An open port on a container
type Port = container.Port

// ContainerState is a string representation of the container's current state.
//
// It currently is an alias for string, but may become a distinct type in the future.
type ContainerState = container.ContainerState

const (
	StateCreated    = container.StateCreated    // StateCreated indicates the container is created, but not (yet) started.
	StateRunning    = container.StateRunning    // StateRunning indicates that the container is running.
	StatePaused     = container.StatePaused     // StatePaused indicates that the container's current state is paused.
	StateRestarting = container.StateRestarting // StateRestarting indicates that the container is currently restarting.
	StateRemoving   = container.StateRemoving   // StateRemoving indicates that the container is being removed.
	StateExited     = container.StateExited     // StateExited indicates that the container exited.
	StateDead       = container.StateDead       // StateDead indicates that the container failed to be deleted. Containers in this state are attempted to be cleaned up when the daemon restarts.
)

// ValidateContainerState checks if the provided string is a valid
// container [container.ContainerState].
func ValidateContainerState(s container.ContainerState) error {
	return container.ValidateContainerState(s)
}

// ThrottlingData stores CPU throttling stats of one running container.
// Not used on Windows.
type ThrottlingData = container.ThrottlingData

// CPUUsage stores All CPU stats aggregated since container inception.
type CPUUsage = container.CPUUsage

// CPUStats aggregates and wraps all CPU related info of container
type CPUStats = container.CPUStats

// MemoryStats aggregates all memory stats since container inception on Linux.
// Windows returns stats for commit and private working set only.
type MemoryStats = container.MemoryStats

// BlkioStatEntry is one small entity to store a piece of Blkio stats
// Not used on Windows.
type BlkioStatEntry = container.BlkioStatEntry

// BlkioStats stores All IO service stats for data read and write.
// This is a Linux specific structure as the differences between expressing
// block I/O on Windows and Linux are sufficiently significant to make
// little sense attempting to morph into a combined structure.
type BlkioStats = container.BlkioStats

// StorageStats is the disk I/O stats for read/write on Windows.
type StorageStats = container.StorageStats

// NetworkStats aggregates the network stats of one container
type NetworkStats = container.NetworkStats

// PidsStats contains the stats of a container's pids
type PidsStats = container.PidsStats

// Stats is Ultimate struct aggregating all types of stats of one container
//
// Deprecated: use [StatsResponse] instead. This type will be removed in the next release.
type Stats = StatsResponse

// StatsResponse aggregates all types of stats of one container.
type StatsResponse = container.StatsResponse

// TopResponse ContainerTopResponse
//
// Container "top" response.
type TopResponse = container.TopResponse

// UpdateResponse ContainerUpdateResponse
//
// Response for a successful container-update.
type UpdateResponse = container.UpdateResponse

// WaitExitError container waiting error, if any
type WaitExitError = container.WaitExitError

// WaitResponse ContainerWaitResponse
//
// OK response to ContainerWait operation
// swagger:model WaitResponse
type WaitResponse = container.WaitResponse

// WaitCondition is a type used to specify a container state for which
// to wait.
type WaitCondition = container.WaitCondition

// Possible WaitCondition Values.
//
// WaitConditionNotRunning (default) is used to wait for any of the non-running
// states: "created", "exited", "dead", "removing", or "removed".
//
// WaitConditionNextExit is used to wait for the next time the state changes
// to a non-running state. If the state is currently "created" or "exited",
// this would cause Wait() to block until either the container runs and exits
// or is removed.
//
// WaitConditionRemoved is used to wait for the container to be removed.
const (
	WaitConditionNotRunning = container.WaitConditionNotRunning
	WaitConditionNextExit   = container.WaitConditionNextExit
	WaitConditionRemoved    = container.WaitConditionRemoved
)
