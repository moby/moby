package client

import (
	"context"
	"io"
	"net"

	"github.com/moby/moby/api/types"
	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/events"
	"github.com/moby/moby/api/types/network"
	"github.com/moby/moby/api/types/registry"
	"github.com/moby/moby/api/types/swarm"
	"github.com/moby/moby/api/types/system"
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
	ImageAPIClient
	NetworkAPIClient
	PluginAPIClient
	SystemAPIClient
	VolumeAPIClient
	ClientVersion() string
	DaemonHost() string
	ServerVersion(ctx context.Context) (types.Version, error)
	NegotiateAPIVersion(ctx context.Context)
	NegotiateAPIVersionPing(PingResult)
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
	SecretAPIClient
	ConfigAPIClient
}

// HijackDialer defines methods for a hijack dialer.
type HijackDialer interface {
	DialHijack(ctx context.Context, url, proto string, meta map[string][]string) (net.Conn, error)
}

// ContainerAPIClient defines API client methods for the containers
type ContainerAPIClient interface {
	ContainerAttach(ctx context.Context, container string, options ContainerAttachOptions) (ContainerAttachResult, error)
	ContainerCommit(ctx context.Context, container string, options ContainerCommitOptions) (ContainerCommitResult, error)
	ContainerCreate(ctx context.Context, options ContainerCreateOptions) (ContainerCreateResult, error)
	ContainerDiff(ctx context.Context, container string, options ContainerDiffOptions) (ContainerDiffResult, error)
	ExecAPIClient
	ContainerExport(ctx context.Context, container string) (io.ReadCloser, error)
	ContainerInspect(ctx context.Context, container string, options ContainerInspectOptions) (ContainerInspectResult, error)
	ContainerKill(ctx context.Context, container string, options ContainerKillOptions) (ContainerKillResult, error)
	ContainerList(ctx context.Context, options ContainerListOptions) ([]container.Summary, error)
	ContainerLogs(ctx context.Context, container string, options ContainerLogsOptions) (io.ReadCloser, error)
	ContainerPause(ctx context.Context, container string, options ContainerPauseOptions) (ContainerPauseResult, error)
	ContainerRemove(ctx context.Context, container string, options ContainerRemoveOptions) (ContainerRemoveResult, error)
	ContainerRename(ctx context.Context, container, newContainerName string) error
	ContainerResize(ctx context.Context, container string, options ContainerResizeOptions) (ContainerResizeResult, error)
	ContainerRestart(ctx context.Context, container string, options ContainerRestartOptions) (ContainerRestartResult, error)
	ContainerStatPath(ctx context.Context, container, path string) (container.PathStat, error)
	ContainerStats(ctx context.Context, container string, options ContainerStatsOptions) (ContainerStatsResult, error)
	ContainerStart(ctx context.Context, container string, options ContainerStartOptions) (ContainerStartResult, error)
	ContainerStop(ctx context.Context, container string, options ContainerStopOptions) (ContainerStopResult, error)
	ContainerTop(ctx context.Context, container string, arguments []string) (container.TopResponse, error)
	ContainerUnpause(ctx context.Context, container string, options ContainerUnPauseOptions) (ContainerUnPauseResult, error)
	ContainerUpdate(ctx context.Context, container string, updateConfig container.UpdateConfig) (container.UpdateResponse, error)
	ContainerWait(ctx context.Context, container string, condition container.WaitCondition) (<-chan container.WaitResponse, <-chan error)
	CopyFromContainer(ctx context.Context, container, srcPath string) (io.ReadCloser, container.PathStat, error)
	CopyToContainer(ctx context.Context, container, path string, content io.Reader, options CopyToContainerOptions) error
	ContainersPrune(ctx context.Context, opts ContainerPruneOptions) (ContainerPruneResult, error)
}

type ExecAPIClient interface {
	ExecCreate(ctx context.Context, container string, options ExecCreateOptions) (ExecCreateResult, error)
	ExecStart(ctx context.Context, execID string, options ExecStartOptions) (ExecStartResult, error)
	ExecAttach(ctx context.Context, execID string, options ExecAttachOptions) (ExecAttachResult, error)
	ExecInspect(ctx context.Context, execID string, options ExecInspectOptions) (ExecInspectResult, error)
	ExecResize(ctx context.Context, execID string, options ExecResizeOptions) (ExecResizeResult, error)
}

// DistributionAPIClient defines API client methods for the registry
type DistributionAPIClient interface {
	DistributionInspect(ctx context.Context, image string, options DistributionInspectOptions) (DistributionInspectResult, error)
}

// ImageAPIClient defines API client methods for the images
type ImageAPIClient interface {
	ImageBuild(ctx context.Context, context io.Reader, options ImageBuildOptions) (ImageBuildResult, error)
	BuildCachePrune(ctx context.Context, opts BuildCachePruneOptions) (BuildCachePruneResult, error)
	BuildCancel(ctx context.Context, id string, opts BuildCancelOptions) (BuildCancelResult, error)
	ImageCreate(ctx context.Context, parentReference string, options ImageCreateOptions) (ImageCreateResult, error)
	ImageImport(ctx context.Context, source ImageImportSource, ref string, options ImageImportOptions) (ImageImportResult, error)

	ImageList(ctx context.Context, options ImageListOptions) (ImageListResult, error)
	ImagePull(ctx context.Context, ref string, options ImagePullOptions) (ImagePullResponse, error)
	ImagePush(ctx context.Context, ref string, options ImagePushOptions) (ImagePushResponse, error)
	ImageRemove(ctx context.Context, image string, options ImageRemoveOptions) (ImageRemoveResult, error)
	ImageSearch(ctx context.Context, term string, options ImageSearchOptions) (ImageSearchResult, error)
	ImageTag(ctx context.Context, options ImageTagOptions) (ImageTagResult, error)
	ImagesPrune(ctx context.Context, opts ImagePruneOptions) (ImagePruneResult, error)

	ImageInspect(ctx context.Context, image string, _ ...ImageInspectOption) (ImageInspectResult, error)
	ImageHistory(ctx context.Context, image string, _ ...ImageHistoryOption) (ImageHistoryResult, error)
	ImageLoad(ctx context.Context, input io.Reader, _ ...ImageLoadOption) (ImageLoadResult, error)
	ImageSave(ctx context.Context, images []string, _ ...ImageSaveOption) (ImageSaveResult, error)
}

// NetworkAPIClient defines API client methods for the networks
type NetworkAPIClient interface {
	NetworkConnect(ctx context.Context, network, container string, config *network.EndpointSettings) error
	NetworkCreate(ctx context.Context, name string, options NetworkCreateOptions) (network.CreateResponse, error)
	NetworkDisconnect(ctx context.Context, network, container string, force bool) error
	NetworkInspect(ctx context.Context, network string, options NetworkInspectOptions) (NetworkInspectResult, error)
	NetworkList(ctx context.Context, options NetworkListOptions) (NetworkListResult, error)
	NetworkRemove(ctx context.Context, network string) error
	NetworksPrune(ctx context.Context, opts NetworkPruneOptions) (NetworkPruneResult, error)
}

// NodeAPIClient defines API client methods for the nodes
type NodeAPIClient interface {
	NodeInspect(ctx context.Context, nodeID string, options NodeInspectOptions) (NodeInspectResult, error)
	NodeList(ctx context.Context, options NodeListOptions) (NodeListResult, error)
	NodeRemove(ctx context.Context, nodeID string, options NodeRemoveOptions) (NodeRemoveResult, error)
	NodeUpdate(ctx context.Context, nodeID string, options NodeUpdateOptions) (NodeUpdateResult, error)
}

// PluginAPIClient defines API client methods for the plugins
type PluginAPIClient interface {
	PluginList(ctx context.Context, options PluginListOptions) (PluginListResult, error)
	PluginRemove(ctx context.Context, name string, options PluginRemoveOptions) (PluginRemoveResult, error)
	PluginEnable(ctx context.Context, name string, options PluginEnableOptions) (PluginEnableResult, error)
	PluginDisable(ctx context.Context, name string, options PluginDisableOptions) (PluginDisableResult, error)
	PluginInstall(ctx context.Context, name string, options PluginInstallOptions) (PluginInstallResult, error)
	PluginUpgrade(ctx context.Context, name string, options PluginUpgradeOptions) (PluginUpgradeResult, error)
	PluginPush(ctx context.Context, name string, options PluginPushOptions) (PluginPushResult, error)
	PluginSet(ctx context.Context, name string, options PluginSetOptions) (PluginSetResult, error)
	PluginInspect(ctx context.Context, name string, options PluginInspectOptions) (PluginInspectResult, error)
	PluginCreate(ctx context.Context, createContext io.Reader, options PluginCreateOptions) (PluginCreateResult, error)
}

// ServiceAPIClient defines API client methods for the services
type ServiceAPIClient interface {
	ServiceCreate(ctx context.Context, options ServiceCreateOptions) (ServiceCreateResult, error)
	ServiceInspect(ctx context.Context, serviceID string, options ServiceInspectOptions) (ServiceInspectResult, error)
	ServiceList(ctx context.Context, options ServiceListOptions) (ServiceListResult, error)
	ServiceRemove(ctx context.Context, serviceID string, options ServiceRemoveOptions) (ServiceRemoveResult, error)
	ServiceUpdate(ctx context.Context, serviceID string, options ServiceUpdateOptions) (ServiceUpdateResult, error)
	ServiceLogs(ctx context.Context, serviceID string, options ServiceLogsOptions) (ServiceLogsResult, error)
	TaskLogs(ctx context.Context, taskID string, options TaskLogsOptions) (TaskLogsResult, error)
	TaskInspect(ctx context.Context, taskID string, options TaskInspectOptions) (TaskInspectResult, error)
	TaskList(ctx context.Context, options TaskListOptions) (TaskListResult, error)
}

// SwarmAPIClient defines API client methods for the swarm
type SwarmAPIClient interface {
	SwarmInit(ctx context.Context, options SwarmInitOptions) (SwarmInitResult, error)
	SwarmJoin(ctx context.Context, options SwarmJoinOptions) (SwarmJoinResult, error)
	SwarmGetUnlockKey(ctx context.Context) (SwarmGetUnlockKeyResult, error)
	SwarmUnlock(ctx context.Context, options SwarmUnlockOptions) (SwarmUnlockResult, error)
	SwarmLeave(ctx context.Context, options SwarmLeaveOptions) (SwarmLeaveResult, error)
	SwarmInspect(ctx context.Context, options SwarmInspectOptions) (SwarmInspectResult, error)
	SwarmUpdate(ctx context.Context, options SwarmUpdateOptions) (SwarmUpdateResult, error)
}

// SystemAPIClient defines API client methods for the system
type SystemAPIClient interface {
	Events(ctx context.Context, options EventsListOptions) (<-chan events.Message, <-chan error)
	Info(ctx context.Context) (system.Info, error)
	RegistryLogin(ctx context.Context, auth registry.AuthConfig) (registry.AuthenticateOKBody, error)
	DiskUsage(ctx context.Context, options DiskUsageOptions) (system.DiskUsage, error)
	Ping(ctx context.Context, options PingOptions) (PingResult, error)
}

// VolumeAPIClient defines API client methods for the volumes
type VolumeAPIClient interface {
	VolumeCreate(ctx context.Context, options VolumeCreateOptions) (VolumeCreateResult, error)
	VolumeInspect(ctx context.Context, volumeID string, options VolumeInspectOptions) (VolumeInspectResult, error)
	VolumeList(ctx context.Context, options VolumeListOptions) (VolumeListResult, error)
	VolumeRemove(ctx context.Context, volumeID string, options VolumeRemoveOptions) error
	VolumesPrune(ctx context.Context, opts VolumePruneOptions) (VolumePruneResult, error)
	VolumeUpdate(ctx context.Context, volumeID string, version swarm.Version, options VolumeUpdateOptions) error
}

// SecretAPIClient defines API client methods for secrets
type SecretAPIClient interface {
	SecretList(ctx context.Context, options SecretListOptions) (SecretListResult, error)
	SecretCreate(ctx context.Context, options SecretCreateOptions) (SecretCreateResult, error)
	SecretRemove(ctx context.Context, id string, options SecretRemoveOptions) (SecretRemoveResult, error)
	SecretInspect(ctx context.Context, id string, options SecretInspectOptions) (SecretInspectResult, error)
	SecretUpdate(ctx context.Context, id string, options SecretUpdateOptions) (SecretUpdateResult, error)
}

// ConfigAPIClient defines API client methods for configs
type ConfigAPIClient interface {
	ConfigList(ctx context.Context, options ConfigListOptions) (ConfigListResult, error)
	ConfigCreate(ctx context.Context, options ConfigCreateOptions) (ConfigCreateResult, error)
	ConfigRemove(ctx context.Context, id string, options ConfigRemoveOptions) (ConfigRemoveResult, error)
	ConfigInspect(ctx context.Context, id string, options ConfigInspectOptions) (ConfigInspectResult, error)
	ConfigUpdate(ctx context.Context, id string, options ConfigUpdateOptions) (ConfigUpdateResult, error)
}
