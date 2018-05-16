package client // import "github.com/docker/docker/client"

import (
	"context"
	"io"
	"net"
	"net/http"
	"time"

	"github.com/docker/docker/api/types/auth"
	containertypes "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/events"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/image"
	networktypes "github.com/docker/docker/api/types/network"
	"github.com/docker/docker/api/types/plugins"
	"github.com/docker/docker/api/types/registry"
	"github.com/docker/docker/api/types/swarm"
	"github.com/docker/docker/api/types/system"
	volumetypes "github.com/docker/docker/api/types/volume"
)

// CommonAPIClient is the common methods between stable and experimental versions of APIClient.
type CommonAPIClient interface {
	ConfigAPIClient
	ContainerAPIClient
	DistributionAPIClient
	ImageAPIClient
	NodeAPIClient
	NetworkAPIClient
	PluginAPIClient
	ServiceAPIClient
	SwarmAPIClient
	SecretAPIClient
	SystemAPIClient
	VolumeAPIClient
	ClientVersion() string
	DaemonHost() string
	HTTPClient() *http.Client
	ServerVersion(ctx context.Context) (system.Version, error)
	NegotiateAPIVersion(ctx context.Context)
	NegotiateAPIVersionPing(system.Ping)
	DialSession(ctx context.Context, proto string, meta map[string][]string) (net.Conn, error)
	Close() error
}

// ContainerAPIClient defines API client methods for the containers
type ContainerAPIClient interface {
	ContainerAttach(ctx context.Context, container string, options ContainerAttachOptions) (HijackedResponse, error)
	ContainerCommit(ctx context.Context, container string, options ContainerCommitOptions) (containertypes.IDResponse, error)
	ContainerCreate(ctx context.Context, config *containertypes.Config, hostConfig *containertypes.HostConfig, networkingConfig *networktypes.NetworkingConfig, containerName string) (containertypes.ContainerCreateCreatedBody, error)
	ContainerDiff(ctx context.Context, container string) ([]containertypes.ContainerChangeResponseItem, error)
	ContainerExecAttach(ctx context.Context, execID string, config containertypes.ExecStartCheck) (HijackedResponse, error)
	ContainerExecCreate(ctx context.Context, container string, config containertypes.ExecConfig) (containertypes.IDResponse, error)
	ContainerExecInspect(ctx context.Context, execID string) (containertypes.ExecInspect, error)
	ContainerExecResize(ctx context.Context, execID string, options ResizeOptions) error
	ContainerExecStart(ctx context.Context, execID string, config containertypes.ExecStartCheck) error
	ContainerExport(ctx context.Context, container string) (io.ReadCloser, error)
	ContainerInspect(ctx context.Context, container string) (containertypes.JSON, error)
	ContainerInspectWithRaw(ctx context.Context, container string, getSize bool) (containertypes.JSON, []byte, error)
	ContainerKill(ctx context.Context, container, signal string) error
	ContainerList(ctx context.Context, options ContainerListOptions) ([]containertypes.Container, error)
	ContainerLogs(ctx context.Context, container string, options ContainerLogsOptions) (io.ReadCloser, error)
	ContainerPause(ctx context.Context, container string) error
	ContainerRemove(ctx context.Context, container string, options ContainerRemoveOptions) error
	ContainerRename(ctx context.Context, container, newContainerName string) error
	ContainerResize(ctx context.Context, container string, options ResizeOptions) error
	ContainerRestart(ctx context.Context, container string, timeout *time.Duration) error
	ContainerStatPath(ctx context.Context, container, path string) (containertypes.PathStat, error)
	ContainerStats(ctx context.Context, container string, stream bool) (containertypes.Stats, error)
	ContainerStart(ctx context.Context, container string, options ContainerStartOptions) error
	ContainerStop(ctx context.Context, container string, timeout *time.Duration) error
	ContainerTop(ctx context.Context, container string, arguments []string) (containertypes.ContainerTopOKBody, error)
	ContainerUnpause(ctx context.Context, container string) error
	ContainerUpdate(ctx context.Context, container string, updateConfig containertypes.UpdateConfig) (containertypes.ContainerUpdateOKBody, error)
	ContainerWait(ctx context.Context, container string, condition containertypes.WaitCondition) (<-chan containertypes.ContainerWaitOKBody, <-chan error)
	CopyFromContainer(ctx context.Context, container, srcPath string) (io.ReadCloser, containertypes.PathStat, error)
	CopyToContainer(ctx context.Context, container, path string, content io.Reader, options CopyToContainerOptions) error
	ContainersPrune(ctx context.Context, pruneFilters filters.Args) (containertypes.PruneReport, error)
}

// DistributionAPIClient defines API client methods for the registry
type DistributionAPIClient interface {
	DistributionInspect(ctx context.Context, image, encodedRegistryAuth string) (registry.DistributionInspect, error)
}

// ImageAPIClient defines API client methods for the images
type ImageAPIClient interface {
	ImageBuild(ctx context.Context, context io.Reader, options ImageBuildOptions) (image.BuildResponse, error)
	BuildCachePrune(ctx context.Context) (*image.BuildCachePruneReport, error)
	ImageCreate(ctx context.Context, parentReference string, options ImageCreateOptions) (io.ReadCloser, error)
	ImageHistory(ctx context.Context, image string) ([]image.HistoryResponseItem, error)
	ImageImport(ctx context.Context, source image.ImportSource, ref string, options ImageImportOptions) (io.ReadCloser, error)
	ImageInspectWithRaw(ctx context.Context, image string) (image.Inspect, []byte, error)
	ImageList(ctx context.Context, options ImageListOptions) ([]image.Summary, error)
	ImageLoad(ctx context.Context, input io.Reader, quiet bool) (image.LoadResponse, error)
	ImagePull(ctx context.Context, ref string, options ImagePullOptions) (io.ReadCloser, error)
	ImagePush(ctx context.Context, ref string, options ImagePushOptions) (io.ReadCloser, error)
	ImageRemove(ctx context.Context, image string, options ImageRemoveOptions) ([]image.DeleteResponseItem, error)
	ImageSearch(ctx context.Context, term string, options ImageSearchOptions) ([]registry.SearchResult, error)
	ImageSave(ctx context.Context, images []string) (io.ReadCloser, error)
	ImageTag(ctx context.Context, image, ref string) error
	ImagesPrune(ctx context.Context, pruneFilter filters.Args) (image.PruneReport, error)
}

// NetworkAPIClient defines API client methods for the networks
type NetworkAPIClient interface {
	NetworkConnect(ctx context.Context, network, container string, config *networktypes.EndpointSettings) error
	NetworkCreate(ctx context.Context, name string, options networktypes.NetworkCreate) (networktypes.CreateResponse, error)
	NetworkDisconnect(ctx context.Context, network, container string, force bool) error
	NetworkInspect(ctx context.Context, network string, options NetworkInspectOptions) (networktypes.Resource, error)
	NetworkInspectWithRaw(ctx context.Context, network string, options NetworkInspectOptions) (networktypes.Resource, []byte, error)
	NetworkList(ctx context.Context, options NetworkListOptions) ([]networktypes.Resource, error)
	NetworkRemove(ctx context.Context, network string) error
	NetworksPrune(ctx context.Context, pruneFilter filters.Args) (networktypes.PruneReport, error)
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
	PluginList(ctx context.Context, filter filters.Args) (plugins.PluginsListResponse, error)
	PluginRemove(ctx context.Context, name string, options PluginRemoveOptions) error
	PluginEnable(ctx context.Context, name string, options PluginEnableOptions) error
	PluginDisable(ctx context.Context, name string, options PluginDisableOptions) error
	PluginInstall(ctx context.Context, name string, options PluginInstallOptions) (io.ReadCloser, error)
	PluginUpgrade(ctx context.Context, name string, options PluginInstallOptions) (io.ReadCloser, error)
	PluginPush(ctx context.Context, name string, registryAuth string) (io.ReadCloser, error)
	PluginSet(ctx context.Context, name string, args []string) error
	PluginInspectWithRaw(ctx context.Context, name string) (*plugins.Plugin, []byte, error)
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
	SwarmUpdate(ctx context.Context, version swarm.Version, swarm swarm.Spec, flags swarm.UpdateFlags) error
}

// SystemAPIClient defines API client methods for the system
type SystemAPIClient interface {
	Events(ctx context.Context, options EventsOptions) (<-chan events.Message, <-chan error)
	Info(ctx context.Context) (system.Info, error)
	RegistryLogin(ctx context.Context, auth auth.Config) (registry.AuthenticateOKBody, error)
	DiskUsage(ctx context.Context) (system.DiskUsage, error)
	Ping(ctx context.Context) (system.Ping, error)
}

// VolumeAPIClient defines API client methods for the volumes
type VolumeAPIClient interface {
	VolumeCreate(ctx context.Context, options volumetypes.VolumeCreateBody) (volumetypes.Volume, error)
	VolumeInspect(ctx context.Context, volumeID string) (volumetypes.Volume, error)
	VolumeInspectWithRaw(ctx context.Context, volumeID string) (volumetypes.Volume, []byte, error)
	VolumeList(ctx context.Context, filter filters.Args) (volumetypes.VolumeListOKBody, error)
	VolumeRemove(ctx context.Context, volumeID string, force bool) error
	VolumesPrune(ctx context.Context, pruneFilter filters.Args) (volumetypes.PruneReport, error)
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
