package client

import (
	"context"
	"io"
	"net"
)

// APIClient is an interface that clients that talk with a docker server must implement.
type APIClient interface {
	stableAPIClient
	CheckpointAPIClient // CheckpointAPIClient is still experimental.
}

type stableAPIClient interface {
	ConfigAPIClient
	ContainerAPIClient
	DistributionAPIClient
	RegistrySearchClient
	ExecAPIClient
	ImageBuildAPIClient
	ImageAPIClient
	NetworkAPIClient
	PluginAPIClient
	SystemAPIClient
	VolumeAPIClient
	ClientVersion() string
	DaemonHost() string
	ServerVersion(ctx context.Context, options ServerVersionOptions) (ServerVersionResult, error)
	HijackDialer
	Dialer() func(context.Context) (net.Conn, error)
	Close() error
	SwarmManagementAPIClient
}

// SwarmManagementAPIClient defines all methods for managing Swarm-specific
// objects.
type SwarmManagementAPIClient interface {
	SwarmAPIClient
	NodeAPIClient
	ServiceAPIClient
	TaskAPIClient
	SecretAPIClient
	ConfigAPIClient
}

// HijackDialer defines methods for a hijack dialer.
type HijackDialer interface {
	DialHijack(ctx context.Context, url, proto string, meta map[string][]string) (net.Conn, error)
}

// CheckpointAPIClient defines API client methods for the checkpoints.
//
// Experimental: checkpoint and restore is still an experimental feature,
// and only available if the daemon is running with experimental features
// enabled.
type CheckpointAPIClient interface {
	CheckpointCreate(ctx context.Context, container string, options CheckpointCreateOptions) (CheckpointCreateResult, error)
	CheckpointRemove(ctx context.Context, container string, options CheckpointRemoveOptions) (CheckpointRemoveResult, error)
	CheckpointList(ctx context.Context, container string, options CheckpointListOptions) (CheckpointListResult, error)
}

// ContainerAPIClient defines API client methods for the containers
type ContainerAPIClient interface {
	ContainerCreate(ctx context.Context, options ContainerCreateOptions) (ContainerCreateResult, error)
	ContainerInspect(ctx context.Context, container string, options ContainerInspectOptions) (ContainerInspectResult, error)
	ContainerList(ctx context.Context, options ContainerListOptions) (ContainerListResult, error)
	ContainerUpdate(ctx context.Context, container string, updateConfig ContainerUpdateOptions) (ContainerUpdateResult, error)
	ContainerRemove(ctx context.Context, container string, options ContainerRemoveOptions) (ContainerRemoveResult, error)
	ContainerPrune(ctx context.Context, opts ContainerPruneOptions) (ContainerPruneResult, error)

	ContainerLogs(ctx context.Context, container string, options ContainerLogsOptions) (ContainerLogsResult, error)

	ContainerStart(ctx context.Context, container string, options ContainerStartOptions) (ContainerStartResult, error)
	ContainerStop(ctx context.Context, container string, options ContainerStopOptions) (ContainerStopResult, error)
	ContainerRestart(ctx context.Context, container string, options ContainerRestartOptions) (ContainerRestartResult, error)
	ContainerPause(ctx context.Context, container string, options ContainerPauseOptions) (ContainerPauseResult, error)
	ContainerUnpause(ctx context.Context, container string, options ContainerUnpauseOptions) (ContainerUnpauseResult, error)
	ContainerWait(ctx context.Context, container string, options ContainerWaitOptions) ContainerWaitResult
	ContainerKill(ctx context.Context, container string, options ContainerKillOptions) (ContainerKillResult, error)

	ContainerRename(ctx context.Context, container string, options ContainerRenameOptions) (ContainerRenameResult, error)
	ContainerResize(ctx context.Context, container string, options ContainerResizeOptions) (ContainerResizeResult, error)
	ContainerAttach(ctx context.Context, container string, options ContainerAttachOptions) (ContainerAttachResult, error)
	ContainerCommit(ctx context.Context, container string, options ContainerCommitOptions) (ContainerCommitResult, error)
	ContainerDiff(ctx context.Context, container string, options ContainerDiffOptions) (ContainerDiffResult, error)
	ContainerExport(ctx context.Context, container string, options ContainerExportOptions) (ContainerExportResult, error)

	ContainerStats(ctx context.Context, container string, options ContainerStatsOptions) (ContainerStatsResult, error)
	ContainerTop(ctx context.Context, container string, options ContainerTopOptions) (ContainerTopResult, error)

	ContainerStatPath(ctx context.Context, container string, options ContainerStatPathOptions) (ContainerStatPathResult, error)
	CopyFromContainer(ctx context.Context, container string, options CopyFromContainerOptions) (CopyFromContainerResult, error)
	CopyToContainer(ctx context.Context, container string, options CopyToContainerOptions) (CopyToContainerResult, error)
}

type ExecAPIClient interface {
	ExecCreate(ctx context.Context, container string, options ExecCreateOptions) (ExecCreateResult, error)
	ExecInspect(ctx context.Context, execID string, options ExecInspectOptions) (ExecInspectResult, error)
	ExecResize(ctx context.Context, execID string, options ExecResizeOptions) (ExecResizeResult, error)

	ExecStart(ctx context.Context, execID string, options ExecStartOptions) (ExecStartResult, error)
	ExecAttach(ctx context.Context, execID string, options ExecAttachOptions) (ExecAttachResult, error)
}

// DistributionAPIClient defines API client methods for the registry
type DistributionAPIClient interface {
	DistributionInspect(ctx context.Context, image string, options DistributionInspectOptions) (DistributionInspectResult, error)
}

type RegistrySearchClient interface {
	ImageSearch(ctx context.Context, term string, options ImageSearchOptions) (ImageSearchResult, error)
}

// ImageBuildAPIClient defines API client methods for building images
// using the REST API.
type ImageBuildAPIClient interface {
	ImageBuild(ctx context.Context, context io.Reader, options ImageBuildOptions) (ImageBuildResult, error)
	BuildCachePrune(ctx context.Context, opts BuildCachePruneOptions) (BuildCachePruneResult, error)
	BuildCancel(ctx context.Context, id string, opts BuildCancelOptions) (BuildCancelResult, error)
}

// ImageAPIClient defines API client methods for the images
type ImageAPIClient interface {
	ImageImport(ctx context.Context, source ImageImportSource, ref string, options ImageImportOptions) (ImageImportResult, error)

	ImageList(ctx context.Context, options ImageListOptions) (ImageListResult, error)
	ImagePull(ctx context.Context, ref string, options ImagePullOptions) (ImagePullResponse, error)
	ImagePush(ctx context.Context, ref string, options ImagePushOptions) (ImagePushResponse, error)
	ImageRemove(ctx context.Context, image string, options ImageRemoveOptions) (ImageRemoveResult, error)
	ImageTag(ctx context.Context, options ImageTagOptions) (ImageTagResult, error)
	ImagePrune(ctx context.Context, opts ImagePruneOptions) (ImagePruneResult, error)

	ImageInspect(ctx context.Context, image string, _ ...ImageInspectOption) (ImageInspectResult, error)
	ImageHistory(ctx context.Context, image string, _ ...ImageHistoryOption) (ImageHistoryResult, error)

	ImageLoad(ctx context.Context, input io.Reader, _ ...ImageLoadOption) (ImageLoadResult, error)
	ImageSave(ctx context.Context, images []string, _ ...ImageSaveOption) (ImageSaveResult, error)
}

// NetworkAPIClient defines API client methods for the networks
type NetworkAPIClient interface {
	NetworkCreate(ctx context.Context, name string, options NetworkCreateOptions) (NetworkCreateResult, error)
	NetworkInspect(ctx context.Context, network string, options NetworkInspectOptions) (NetworkInspectResult, error)
	NetworkList(ctx context.Context, options NetworkListOptions) (NetworkListResult, error)
	NetworkRemove(ctx context.Context, network string, options NetworkRemoveOptions) (NetworkRemoveResult, error)
	NetworkPrune(ctx context.Context, opts NetworkPruneOptions) (NetworkPruneResult, error)

	NetworkConnect(ctx context.Context, network string, options NetworkConnectOptions) (NetworkConnectResult, error)
	NetworkDisconnect(ctx context.Context, network string, options NetworkDisconnectOptions) (NetworkDisconnectResult, error)
}

// NodeAPIClient defines API client methods for the nodes
type NodeAPIClient interface {
	NodeInspect(ctx context.Context, nodeID string, options NodeInspectOptions) (NodeInspectResult, error)
	NodeList(ctx context.Context, options NodeListOptions) (NodeListResult, error)
	NodeUpdate(ctx context.Context, nodeID string, options NodeUpdateOptions) (NodeUpdateResult, error)
	NodeRemove(ctx context.Context, nodeID string, options NodeRemoveOptions) (NodeRemoveResult, error)
}

// PluginAPIClient defines API client methods for the plugins
type PluginAPIClient interface {
	PluginCreate(ctx context.Context, createContext io.Reader, options PluginCreateOptions) (PluginCreateResult, error)
	PluginInstall(ctx context.Context, name string, options PluginInstallOptions) (PluginInstallResult, error)
	PluginInspect(ctx context.Context, name string, options PluginInspectOptions) (PluginInspectResult, error)
	PluginList(ctx context.Context, options PluginListOptions) (PluginListResult, error)
	PluginRemove(ctx context.Context, name string, options PluginRemoveOptions) (PluginRemoveResult, error)

	PluginEnable(ctx context.Context, name string, options PluginEnableOptions) (PluginEnableResult, error)
	PluginDisable(ctx context.Context, name string, options PluginDisableOptions) (PluginDisableResult, error)
	PluginUpgrade(ctx context.Context, name string, options PluginUpgradeOptions) (PluginUpgradeResult, error)
	PluginPush(ctx context.Context, name string, options PluginPushOptions) (PluginPushResult, error)
	PluginSet(ctx context.Context, name string, options PluginSetOptions) (PluginSetResult, error)
}

// ServiceAPIClient defines API client methods for the services
type ServiceAPIClient interface {
	ServiceCreate(ctx context.Context, options ServiceCreateOptions) (ServiceCreateResult, error)
	ServiceInspect(ctx context.Context, serviceID string, options ServiceInspectOptions) (ServiceInspectResult, error)
	ServiceList(ctx context.Context, options ServiceListOptions) (ServiceListResult, error)
	ServiceUpdate(ctx context.Context, serviceID string, options ServiceUpdateOptions) (ServiceUpdateResult, error)
	ServiceRemove(ctx context.Context, serviceID string, options ServiceRemoveOptions) (ServiceRemoveResult, error)

	ServiceLogs(ctx context.Context, serviceID string, options ServiceLogsOptions) (ServiceLogsResult, error)
}

// TaskAPIClient defines API client methods to manage swarm tasks.
type TaskAPIClient interface {
	TaskInspect(ctx context.Context, taskID string, options TaskInspectOptions) (TaskInspectResult, error)
	TaskList(ctx context.Context, options TaskListOptions) (TaskListResult, error)

	TaskLogs(ctx context.Context, taskID string, options TaskLogsOptions) (TaskLogsResult, error)
}

// SwarmAPIClient defines API client methods for the swarm
type SwarmAPIClient interface {
	SwarmInit(ctx context.Context, options SwarmInitOptions) (SwarmInitResult, error)
	SwarmJoin(ctx context.Context, options SwarmJoinOptions) (SwarmJoinResult, error)
	SwarmInspect(ctx context.Context, options SwarmInspectOptions) (SwarmInspectResult, error)
	SwarmUpdate(ctx context.Context, options SwarmUpdateOptions) (SwarmUpdateResult, error)
	SwarmLeave(ctx context.Context, options SwarmLeaveOptions) (SwarmLeaveResult, error)

	SwarmGetUnlockKey(ctx context.Context) (SwarmGetUnlockKeyResult, error)
	SwarmUnlock(ctx context.Context, options SwarmUnlockOptions) (SwarmUnlockResult, error)
}

// SystemAPIClient defines API client methods for the system
type SystemAPIClient interface {
	Events(ctx context.Context, options EventsListOptions) EventsResult
	Info(ctx context.Context, options InfoOptions) (SystemInfoResult, error)
	RegistryLogin(ctx context.Context, auth RegistryLoginOptions) (RegistryLoginResult, error)
	DiskUsage(ctx context.Context, options DiskUsageOptions) (DiskUsageResult, error)
	Ping(ctx context.Context, options PingOptions) (PingResult, error)
}

// VolumeAPIClient defines API client methods for the volumes
type VolumeAPIClient interface {
	VolumeCreate(ctx context.Context, options VolumeCreateOptions) (VolumeCreateResult, error)
	VolumeInspect(ctx context.Context, volumeID string, options VolumeInspectOptions) (VolumeInspectResult, error)
	VolumeList(ctx context.Context, options VolumeListOptions) (VolumeListResult, error)
	VolumeUpdate(ctx context.Context, volumeID string, options VolumeUpdateOptions) (VolumeUpdateResult, error)
	VolumeRemove(ctx context.Context, volumeID string, options VolumeRemoveOptions) (VolumeRemoveResult, error)
	VolumePrune(ctx context.Context, options VolumePruneOptions) (VolumePruneResult, error)
}

// SecretAPIClient defines API client methods for secrets
type SecretAPIClient interface {
	SecretCreate(ctx context.Context, options SecretCreateOptions) (SecretCreateResult, error)
	SecretInspect(ctx context.Context, id string, options SecretInspectOptions) (SecretInspectResult, error)
	SecretList(ctx context.Context, options SecretListOptions) (SecretListResult, error)
	SecretUpdate(ctx context.Context, id string, options SecretUpdateOptions) (SecretUpdateResult, error)
	SecretRemove(ctx context.Context, id string, options SecretRemoveOptions) (SecretRemoveResult, error)
}

// ConfigAPIClient defines API client methods for configs
type ConfigAPIClient interface {
	ConfigCreate(ctx context.Context, options ConfigCreateOptions) (ConfigCreateResult, error)
	ConfigInspect(ctx context.Context, id string, options ConfigInspectOptions) (ConfigInspectResult, error)
	ConfigList(ctx context.Context, options ConfigListOptions) (ConfigListResult, error)
	ConfigUpdate(ctx context.Context, id string, options ConfigUpdateOptions) (ConfigUpdateResult, error)
	ConfigRemove(ctx context.Context, id string, options ConfigRemoveOptions) (ConfigRemoveResult, error)
}
