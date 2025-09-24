package client

import (
	"context"
	"io"
	"net"
	"net/http"

	"github.com/moby/moby/api/types"
	"github.com/moby/moby/api/types/build"
	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/events"
	"github.com/moby/moby/api/types/filters"
	"github.com/moby/moby/api/types/image"
	"github.com/moby/moby/api/types/network"
	"github.com/moby/moby/api/types/plugin"
	"github.com/moby/moby/api/types/registry"
	"github.com/moby/moby/api/types/swarm"
	"github.com/moby/moby/api/types/system"
	"github.com/moby/moby/api/types/volume"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
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
	HTTPClient() *http.Client
	ServerVersion(ctx context.Context) (types.Version, error)
	NegotiateAPIVersion(ctx context.Context)
	NegotiateAPIVersionPing(types.Ping)
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
	ContainerAttach(ctx context.Context, container string, options ContainerAttachOptions) (HijackedResponse, error)
	ContainerCommit(ctx context.Context, container string, options ContainerCommitOptions) (container.CommitResponse, error)
	ContainerCreate(ctx context.Context, config *container.Config, hostConfig *container.HostConfig, networkingConfig *network.NetworkingConfig, platform *ocispec.Platform, containerName string) (container.CreateResponse, error)
	ContainerDiff(ctx context.Context, container string) ([]container.FilesystemChange, error)
	ContainerExecAttach(ctx context.Context, execID string, options container.ExecAttachOptions) (HijackedResponse, error)
	ContainerExecCreate(ctx context.Context, container string, options container.ExecOptions) (container.ExecCreateResponse, error)
	ContainerExecInspect(ctx context.Context, execID string) (container.ExecInspect, error)
	ContainerExecResize(ctx context.Context, execID string, options ContainerResizeOptions) error
	ContainerExecStart(ctx context.Context, execID string, options container.ExecStartOptions) error
	ContainerExport(ctx context.Context, container string) (io.ReadCloser, error)
	ContainerInspect(ctx context.Context, container string) (container.InspectResponse, error)
	ContainerInspectWithRaw(ctx context.Context, container string, getSize bool) (container.InspectResponse, []byte, error)
	ContainerKill(ctx context.Context, container, signal string) error
	ContainerList(ctx context.Context, options ContainerListOptions) ([]container.Summary, error)
	ContainerLogs(ctx context.Context, container string, options ContainerLogsOptions) (io.ReadCloser, error)
	ContainerPause(ctx context.Context, container string) error
	ContainerRemove(ctx context.Context, container string, options ContainerRemoveOptions) error
	ContainerRename(ctx context.Context, container, newContainerName string) error
	ContainerResize(ctx context.Context, container string, options ContainerResizeOptions) error
	ContainerRestart(ctx context.Context, container string, options ContainerStopOptions) error
	ContainerStatPath(ctx context.Context, container, path string) (container.PathStat, error)
	ContainerStats(ctx context.Context, container string, stream bool) (StatsResponseReader, error)
	ContainerStatsOneShot(ctx context.Context, container string) (StatsResponseReader, error)
	ContainerStart(ctx context.Context, container string, options ContainerStartOptions) error
	ContainerStop(ctx context.Context, container string, options ContainerStopOptions) error
	ContainerTop(ctx context.Context, container string, arguments []string) (container.TopResponse, error)
	ContainerUnpause(ctx context.Context, container string) error
	ContainerUpdate(ctx context.Context, container string, updateConfig container.UpdateConfig) (container.UpdateResponse, error)
	ContainerWait(ctx context.Context, container string, condition container.WaitCondition) (<-chan container.WaitResponse, <-chan error)
	CopyFromContainer(ctx context.Context, container, srcPath string) (io.ReadCloser, container.PathStat, error)
	CopyToContainer(ctx context.Context, container, path string, content io.Reader, options CopyToContainerOptions) error
	ContainersPrune(ctx context.Context, pruneFilters filters.Args) (container.PruneReport, error)
}

// DistributionAPIClient defines API client methods for the registry
type DistributionAPIClient interface {
	DistributionInspect(ctx context.Context, image, encodedRegistryAuth string) (registry.DistributionInspect, error)
}

// ImageAPIClient defines API client methods for the images
type ImageAPIClient interface {
	ImageBuild(ctx context.Context, context io.Reader, options ImageBuildOptions) (ImageBuildResponse, error)
	BuildCachePrune(ctx context.Context, opts BuildCachePruneOptions) (*build.CachePruneReport, error)
	BuildCancel(ctx context.Context, id string) error
	ImageCreate(ctx context.Context, parentReference string, options ImageCreateOptions) (io.ReadCloser, error)
	ImageImport(ctx context.Context, source ImageImportSource, ref string, options ImageImportOptions) (io.ReadCloser, error)

	ImageList(ctx context.Context, options ImageListOptions) ([]image.Summary, error)
	ImagePull(ctx context.Context, ref string, options ImagePullOptions) (io.ReadCloser, error)
	ImagePush(ctx context.Context, ref string, options ImagePushOptions) (io.ReadCloser, error)
	ImageRemove(ctx context.Context, image string, options ImageRemoveOptions) ([]image.DeleteResponse, error)
	ImageSearch(ctx context.Context, term string, options ImageSearchOptions) ([]registry.SearchResult, error)
	ImageTag(ctx context.Context, image, ref string) error
	ImagesPrune(ctx context.Context, pruneFilter filters.Args) (image.PruneReport, error)

	ImageInspect(ctx context.Context, image string, _ ...ImageInspectOption) (image.InspectResponse, error)
	ImageHistory(ctx context.Context, image string, _ ...ImageHistoryOption) ([]image.HistoryResponseItem, error)
	ImageLoad(ctx context.Context, input io.Reader, _ ...ImageLoadOption) (LoadResponse, error)
	ImageSave(ctx context.Context, images []string, _ ...ImageSaveOption) (io.ReadCloser, error)
}

// NetworkAPIClient defines API client methods for the networks
type NetworkAPIClient interface {
	NetworkConnect(ctx context.Context, network, container string, config *network.EndpointSettings) error
	NetworkCreate(ctx context.Context, name string, options NetworkCreateOptions) (network.CreateResponse, error)
	NetworkDisconnect(ctx context.Context, network, container string, force bool) error
	NetworkInspect(ctx context.Context, network string, options NetworkInspectOptions) (network.Inspect, error)
	NetworkInspectWithRaw(ctx context.Context, network string, options NetworkInspectOptions) (network.Inspect, []byte, error)
	NetworkList(ctx context.Context, options NetworkListOptions) ([]network.Summary, error)
	NetworkRemove(ctx context.Context, network string) error
	NetworksPrune(ctx context.Context, pruneFilter filters.Args) (network.PruneReport, error)
}

// NodeAPIClient defines API client methods for the nodes
type NodeAPIClient interface {
	NodeInspectWithRaw(ctx context.Context, nodeID string) (swarm.Node, []byte, error)
	NodeList(ctx context.Context, options NodeListOptions) ([]swarm.Node, error)
	NodeRemove(ctx context.Context, nodeID string, options NodeRemoveOptions) error
	NodeUpdate(ctx context.Context, nodeID string, version swarm.Version, node swarm.NodeSpec) error
}

// PluginAPIClient defines API client methods for the plugins
type PluginAPIClient interface {
	PluginList(ctx context.Context, filter filters.Args) (plugin.ListResponse, error)
	PluginRemove(ctx context.Context, name string, options PluginRemoveOptions) error
	PluginEnable(ctx context.Context, name string, options PluginEnableOptions) error
	PluginDisable(ctx context.Context, name string, options PluginDisableOptions) error
	PluginInstall(ctx context.Context, name string, options PluginInstallOptions) (io.ReadCloser, error)
	PluginUpgrade(ctx context.Context, name string, options PluginInstallOptions) (io.ReadCloser, error)
	PluginPush(ctx context.Context, name string, registryAuth string) (io.ReadCloser, error)
	PluginSet(ctx context.Context, name string, args []string) error
	PluginInspectWithRaw(ctx context.Context, name string) (*plugin.Plugin, []byte, error)
	PluginCreate(ctx context.Context, createContext io.Reader, options PluginCreateOptions) error
}

// ServiceAPIClient defines API client methods for the services
type ServiceAPIClient interface {
	ServiceCreate(ctx context.Context, service swarm.ServiceSpec, options ServiceCreateOptions) (swarm.ServiceCreateResponse, error)
	ServiceInspectWithRaw(ctx context.Context, serviceID string, options ServiceInspectOptions) (swarm.Service, []byte, error)
	ServiceList(ctx context.Context, options ServiceListOptions) ([]swarm.Service, error)
	ServiceRemove(ctx context.Context, serviceID string) error
	ServiceUpdate(ctx context.Context, serviceID string, version swarm.Version, service swarm.ServiceSpec, options ServiceUpdateOptions) (swarm.ServiceUpdateResponse, error)
	ServiceLogs(ctx context.Context, serviceID string, options ContainerLogsOptions) (io.ReadCloser, error)
	TaskLogs(ctx context.Context, taskID string, options ContainerLogsOptions) (io.ReadCloser, error)
	TaskInspectWithRaw(ctx context.Context, taskID string) (swarm.Task, []byte, error)
	TaskList(ctx context.Context, options TaskListOptions) ([]swarm.Task, error)
}

// SwarmAPIClient defines API client methods for the swarm
type SwarmAPIClient interface {
	SwarmInit(ctx context.Context, req swarm.InitRequest) (string, error)
	SwarmJoin(ctx context.Context, req swarm.JoinRequest) error
	SwarmGetUnlockKey(ctx context.Context) (swarm.UnlockKeyResponse, error)
	SwarmUnlock(ctx context.Context, req swarm.UnlockRequest) error
	SwarmLeave(ctx context.Context, force bool) error
	SwarmInspect(ctx context.Context) (swarm.Swarm, error)
	SwarmUpdate(ctx context.Context, version swarm.Version, swarm swarm.Spec, flags SwarmUpdateFlags) error
}

// SystemAPIClient defines API client methods for the system
type SystemAPIClient interface {
	Events(ctx context.Context, options EventsListOptions) (<-chan events.Message, <-chan error)
	Info(ctx context.Context) (system.Info, error)
	RegistryLogin(ctx context.Context, auth registry.AuthConfig) (registry.AuthenticateOKBody, error)
	DiskUsage(ctx context.Context, options DiskUsageOptions) (system.DiskUsage, error)
	Ping(ctx context.Context) (types.Ping, error)
}

// VolumeAPIClient defines API client methods for the volumes
type VolumeAPIClient interface {
	VolumeCreate(ctx context.Context, options volume.CreateOptions) (volume.Volume, error)
	VolumeInspect(ctx context.Context, volumeID string) (volume.Volume, error)
	VolumeInspectWithRaw(ctx context.Context, volumeID string) (volume.Volume, []byte, error)
	VolumeList(ctx context.Context, options VolumeListOptions) (volume.ListResponse, error)
	VolumeRemove(ctx context.Context, volumeID string, force bool) error
	VolumesPrune(ctx context.Context, pruneFilter filters.Args) (volume.PruneReport, error)
	VolumeUpdate(ctx context.Context, volumeID string, version swarm.Version, options volume.UpdateOptions) error
}

// SecretAPIClient defines API client methods for secrets
type SecretAPIClient interface {
	SecretList(ctx context.Context, options SecretListOptions) ([]swarm.Secret, error)
	SecretCreate(ctx context.Context, secret swarm.SecretSpec) (swarm.SecretCreateResponse, error)
	SecretRemove(ctx context.Context, id string) error
	SecretInspectWithRaw(ctx context.Context, name string) (swarm.Secret, []byte, error)
	SecretUpdate(ctx context.Context, id string, version swarm.Version, secret swarm.SecretSpec) error
}

// ConfigAPIClient defines API client methods for configs
type ConfigAPIClient interface {
	ConfigList(ctx context.Context, options ConfigListOptions) ([]swarm.Config, error)
	ConfigCreate(ctx context.Context, config swarm.ConfigSpec) (swarm.ConfigCreateResponse, error)
	ConfigRemove(ctx context.Context, id string) error
	ConfigInspectWithRaw(ctx context.Context, name string) (swarm.Config, []byte, error)
	ConfigUpdate(ctx context.Context, id string, version swarm.Version, config swarm.ConfigSpec) error
}
