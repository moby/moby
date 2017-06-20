package client

import (
	"io"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/events"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/api/types/registry"
	"github.com/docker/docker/api/types/swarm"
	volumetypes "github.com/docker/docker/api/types/volume"
	"golang.org/x/net/context"
)

// MockDockerClient is a struct that test cases embed so that the
// test-case-struct can serve as DockerClient, effectively
// intercepting each point where the a4c command being tested does
// anything with docker.  The test-case-struct has to receive all
// DockerClient functions your code invokes (if you miss one,
// you'll get this "base class" implementation and panic.)
type MockDockerClient struct {
}

// mockDockerExplanation explains why your code is panicing.  The
// panic means that your code under test invoked a DockerClient
// function that your test case has not implemented, so you've wound
// up with the default implementation in this library, which just
// panics, to let you know you have more work to do (make your test
// case provide a suitable mock implementation of this function, or
// modify the code under test to not call this function).
var mockDockerExplanation string = "Unimplemented DockerClient function called; see go help for mockDockerExplanation"

func (m *MockDockerClient) ClientVersion() string {
	panic(mockDockerExplanation)
}
func (m *MockDockerClient) ServerVersion(ctx context.Context) (types.Version, error) {
	panic(mockDockerExplanation)
}
func (m *MockDockerClient) UpdateClientVersion(v string) {
	panic(mockDockerExplanation)
}
func (m *MockDockerClient) ContainerAttach(ctx context.Context, container string,
	options types.ContainerAttachOptions) (types.HijackedResponse, error) {
	panic(mockDockerExplanation)
}
func (m *MockDockerClient) ContainerCommit(ctx context.Context, container string,
	options types.ContainerCommitOptions) (types.IDResponse, error) {
	panic(mockDockerExplanation)
}
func (m *MockDockerClient) ContainerCreate(ctx context.Context, config *container.Config,
	hostConfig *container.HostConfig, networkingConfig *network.NetworkingConfig,
	containerName string) (container.ContainerCreateCreatedBody, error) {
	panic(mockDockerExplanation)
}
func (m *MockDockerClient) ContainerDiff(ctx context.Context,
	container string) ([]container.ContainerChangeResponseItem, error) {
	panic(mockDockerExplanation)
}
func (m *MockDockerClient) ContainerExecAttach(ctx context.Context, execID string,
	config types.ExecConfig) (types.HijackedResponse, error) {
	panic(mockDockerExplanation)
}
func (m *MockDockerClient) ContainerExecCreate(ctx context.Context, container string,
	config types.ExecConfig) (types.IDResponse, error) {
	panic(mockDockerExplanation)
}
func (m *MockDockerClient) ContainerExecInspect(ctx context.Context,
	execID string) (types.ContainerExecInspect, error) {
	panic(mockDockerExplanation)
}
func (m *MockDockerClient) ContainerExecResize(ctx context.Context, execID string, options types.ResizeOptions) error {
	panic(mockDockerExplanation)
}
func (m *MockDockerClient) ContainerExecStart(ctx context.Context, execID string, config types.ExecStartCheck) error {
	panic(mockDockerExplanation)
}
func (m *MockDockerClient) ContainerExport(ctx context.Context, container string) (io.ReadCloser, error) {
	panic(mockDockerExplanation)
}
func (m *MockDockerClient) ContainerInspect(ctx context.Context, container string) (types.ContainerJSON, error) {
	panic(mockDockerExplanation)
}
func (m *MockDockerClient) ContainerInspectWithRaw(ctx context.Context, container string,
	getSize bool) (types.ContainerJSON, []byte, error) {
	panic(mockDockerExplanation)
}
func (m *MockDockerClient) ContainerKill(ctx context.Context, container, signal string) error {
	panic(mockDockerExplanation)
}
func (m *MockDockerClient) ContainerList(ctx context.Context,
	options types.ContainerListOptions) ([]types.Container, error) {
	panic(mockDockerExplanation)
}
func (m *MockDockerClient) ContainerLogs(ctx context.Context, container string,
	options types.ContainerLogsOptions) (io.ReadCloser, error) {
	panic(mockDockerExplanation)
}
func (m *MockDockerClient) ContainerPause(ctx context.Context, container string) error {
	panic(mockDockerExplanation)
}
func (m *MockDockerClient) ContainerRemove(ctx context.Context, container string,
	options types.ContainerRemoveOptions) error {
	panic(mockDockerExplanation)
}
func (m *MockDockerClient) ContainerRename(ctx context.Context, container, newContainerName string) error {
	panic(mockDockerExplanation)
}
func (m *MockDockerClient) ContainerResize(ctx context.Context, container string, options types.ResizeOptions) error {
	panic(mockDockerExplanation)
}
func (m *MockDockerClient) ContainerRestart(ctx context.Context, container string, timeout *time.Duration) error {
	panic(mockDockerExplanation)
}
func (m *MockDockerClient) ContainerStatPath(ctx context.Context, container,
	path string) (types.ContainerPathStat, error) {
	panic(mockDockerExplanation)
}
func (m *MockDockerClient) ContainerStats(ctx context.Context, container string,
	stream bool) (types.ContainerStats, error) {
	panic(mockDockerExplanation)
}
func (m *MockDockerClient) ContainerStart(ctx context.Context, container string,
	options types.ContainerStartOptions) error {
	panic(mockDockerExplanation)
}
func (m *MockDockerClient) ContainerStop(ctx context.Context, container string, timeout *time.Duration) error {
	panic(mockDockerExplanation)
}
func (m *MockDockerClient) ContainerTop(ctx context.Context, container string,
	arguments []string) (container.ContainerTopOKBody, error) {
	panic(mockDockerExplanation)
}
func (m *MockDockerClient) ContainerUnpause(ctx context.Context, container string) error {
	panic(mockDockerExplanation)
}
func (m *MockDockerClient) ContainerUpdate(ctx context.Context, container string,
	updateConfig container.UpdateConfig) (container.ContainerUpdateOKBody, error) {
	panic(mockDockerExplanation)
}
func (m *MockDockerClient) ContainerWait(ctx context.Context, container string) (int64, error) {
	panic(mockDockerExplanation)
}
func (m *MockDockerClient) CopyFromContainer(ctx context.Context, container,
	srcPath string) (io.ReadCloser, types.ContainerPathStat, error) {
	panic(mockDockerExplanation)
}
func (m *MockDockerClient) CopyToContainer(ctx context.Context, container, path string, content io.Reader,
	options types.CopyToContainerOptions) error {
	panic(mockDockerExplanation)
}
func (m *MockDockerClient) ContainersPrune(ctx context.Context,
	pruneFilters filters.Args) (types.ContainersPruneReport, error) {
	panic(mockDockerExplanation)
}
func (m *MockDockerClient) ImageBuild(ctx context.Context, context io.Reader,
	options types.ImageBuildOptions) (types.ImageBuildResponse, error) {
	panic(mockDockerExplanation)
}
func (m *MockDockerClient) ImageCreate(ctx context.Context, parentReference string,
	options types.ImageCreateOptions) (io.ReadCloser, error) {
	panic(mockDockerExplanation)
}
func (m *MockDockerClient) ImageHistory(ctx context.Context, image string) ([]image.HistoryResponseItem, error) {
	panic(mockDockerExplanation)
}
func (m *MockDockerClient) ImageImport(ctx context.Context, source types.ImageImportSource, ref string,
	options types.ImageImportOptions) (io.ReadCloser, error) {
	panic(mockDockerExplanation)
}
func (m *MockDockerClient) ImageInspectWithRaw(ctx context.Context, image string) (types.ImageInspect, []byte, error) {
	panic(mockDockerExplanation)
}
func (m *MockDockerClient) ImageList(ctx context.Context,
	options types.ImageListOptions) ([]types.ImageSummary, error) {
	panic(mockDockerExplanation)
}
func (m *MockDockerClient) ImageLoad(ctx context.Context, input io.Reader,
	quiet bool) (types.ImageLoadResponse, error) {
	panic(mockDockerExplanation)
}
func (m *MockDockerClient) ImagePull(ctx context.Context, ref string,
	options types.ImagePullOptions) (io.ReadCloser, error) {
	panic(mockDockerExplanation)
}
func (m *MockDockerClient) ImagePush(ctx context.Context, ref string,
	options types.ImagePushOptions) (io.ReadCloser, error) {
	panic(mockDockerExplanation)
}
func (m *MockDockerClient) ImageRemove(ctx context.Context, image string,
	options types.ImageRemoveOptions) ([]types.ImageDeleteResponseItem, error) {
	panic(mockDockerExplanation)
}
func (m *MockDockerClient) ImageSearch(ctx context.Context, term string,
	options types.ImageSearchOptions) ([]registry.SearchResult, error) {
	panic(mockDockerExplanation)
}
func (m *MockDockerClient) ImageSave(ctx context.Context, images []string) (io.ReadCloser, error) {
	panic(mockDockerExplanation)
}
func (m *MockDockerClient) ImageTag(ctx context.Context, image, ref string) error {
	panic(mockDockerExplanation)
}
func (m *MockDockerClient) ImagesPrune(ctx context.Context, pruneFilter filters.Args) (types.ImagesPruneReport, error) {
	panic(mockDockerExplanation)
}
func (m *MockDockerClient) NetworkConnect(ctx context.Context, networkID, container string,
	config *network.EndpointSettings) error {
	panic(mockDockerExplanation)
}
func (m *MockDockerClient) NetworkCreate(ctx context.Context, name string,
	options types.NetworkCreate) (types.NetworkCreateResponse, error) {
	panic(mockDockerExplanation)
}
func (m *MockDockerClient) NetworkDisconnect(ctx context.Context, networkID, container string, force bool) error {
	panic(mockDockerExplanation)
}
func (m *MockDockerClient) NetworkInspect(ctx context.Context, networkID string,
	verbose bool) (types.NetworkResource, error) {
	panic(mockDockerExplanation)
}
func (m *MockDockerClient) NetworkInspectWithRaw(ctx context.Context, networkID string,
	verbose bool) (types.NetworkResource, []byte, error) {
	panic(mockDockerExplanation)
}
func (m *MockDockerClient) NetworkList(ctx context.Context,
	options types.NetworkListOptions) ([]types.NetworkResource, error) {
	panic(mockDockerExplanation)
}
func (m *MockDockerClient) NetworkRemove(ctx context.Context, networkID string) error {
	panic(mockDockerExplanation)
}
func (m *MockDockerClient) NetworksPrune(ctx context.Context,
	pruneFilter filters.Args) (types.NetworksPruneReport, error) {
	panic(mockDockerExplanation)
}
func (m *MockDockerClient) NodeInspectWithRaw(ctx context.Context, nodeID string) (swarm.Node, []byte, error) {
	panic(mockDockerExplanation)
}
func (m *MockDockerClient) NodeList(ctx context.Context, options types.NodeListOptions) ([]swarm.Node, error) {
	panic(mockDockerExplanation)
}
func (m *MockDockerClient) NodeRemove(ctx context.Context, nodeID string, options types.NodeRemoveOptions) error {
	panic(mockDockerExplanation)
}
func (m *MockDockerClient) NodeUpdate(ctx context.Context, nodeID string, version swarm.Version,
	node swarm.NodeSpec) error {
	panic(mockDockerExplanation)
}
func (m *MockDockerClient) PluginList(ctx context.Context, filter filters.Args) (types.PluginsListResponse, error) {
	panic(mockDockerExplanation)
}
func (m *MockDockerClient) PluginRemove(ctx context.Context, name string, options types.PluginRemoveOptions) error {
	panic(mockDockerExplanation)
}
func (m *MockDockerClient) PluginEnable(ctx context.Context, name string, options types.PluginEnableOptions) error {
	panic(mockDockerExplanation)
}
func (m *MockDockerClient) PluginDisable(ctx context.Context, name string, options types.PluginDisableOptions) error {
	panic(mockDockerExplanation)
}
func (m *MockDockerClient) PluginInstall(ctx context.Context, name string,
	options types.PluginInstallOptions) (io.ReadCloser, error) {
	panic(mockDockerExplanation)
}
func (m *MockDockerClient) PluginPush(ctx context.Context, name string, registryAuth string) (io.ReadCloser, error) {
	panic(mockDockerExplanation)
}
func (m *MockDockerClient) PluginSet(ctx context.Context, name string, args []string) error {
	panic(mockDockerExplanation)
}
func (m *MockDockerClient) PluginInspectWithRaw(ctx context.Context, name string) (*types.Plugin, []byte, error) {
	panic(mockDockerExplanation)
}
func (m *MockDockerClient) PluginCreate(ctx context.Context, createContext io.Reader,
	options types.PluginCreateOptions) error {
	panic(mockDockerExplanation)
}
func (m *MockDockerClient) PluginUpgrade(ctx context.Context, name string,
	options types.PluginInstallOptions) (rc io.ReadCloser, err error) {
	panic(mockDockerExplanation)
}
func (m *MockDockerClient) ServiceCreate(ctx context.Context, service swarm.ServiceSpec,
	options types.ServiceCreateOptions) (types.ServiceCreateResponse, error) {
	panic(mockDockerExplanation)
}
func (m *MockDockerClient) ServiceInspectWithRaw(ctx context.Context, serviceID string,
	opts types.ServiceInspectOptions) (swarm.Service, []byte, error) {
	panic(mockDockerExplanation)
}
func (m *MockDockerClient) ServiceList(ctx context.Context, options types.ServiceListOptions) ([]swarm.Service, error) {
	panic(mockDockerExplanation)
}
func (m *MockDockerClient) ServiceRemove(ctx context.Context, serviceID string) error {
	panic(mockDockerExplanation)
}
func (m *MockDockerClient) ServiceUpdate(ctx context.Context, serviceID string, version swarm.Version,
	service swarm.ServiceSpec, options types.ServiceUpdateOptions) (types.ServiceUpdateResponse, error) {
	panic(mockDockerExplanation)
}
func (m *MockDockerClient) ServiceLogs(ctx context.Context, serviceID string,
	options types.ContainerLogsOptions) (io.ReadCloser, error) {
	panic(mockDockerExplanation)
}
func (m *MockDockerClient) TaskInspectWithRaw(ctx context.Context, taskID string) (swarm.Task, []byte, error) {
	panic(mockDockerExplanation)
}
func (m *MockDockerClient) TaskList(ctx context.Context, options types.TaskListOptions) ([]swarm.Task, error) {
	panic(mockDockerExplanation)
}
func (m *MockDockerClient) TaskLogs(ctx context.Context, taskID string,
	options types.ContainerLogsOptions) (io.ReadCloser, error) {
	panic(mockDockerExplanation)
}
func (m *MockDockerClient) SwarmInit(ctx context.Context, req swarm.InitRequest) (string, error) {
	panic(mockDockerExplanation)
}
func (m *MockDockerClient) SwarmJoin(ctx context.Context, req swarm.JoinRequest) error {
	panic(mockDockerExplanation)
}
func (m *MockDockerClient) SwarmGetUnlockKey(ctx context.Context) (types.SwarmUnlockKeyResponse, error) {
	panic(mockDockerExplanation)
}
func (m *MockDockerClient) SwarmUnlock(ctx context.Context, req swarm.UnlockRequest) error {
	panic(mockDockerExplanation)
}
func (m *MockDockerClient) SwarmLeave(ctx context.Context, force bool) error {
	panic(mockDockerExplanation)
}
func (m *MockDockerClient) SwarmInspect(ctx context.Context) (swarm.Swarm, error) {
	panic(mockDockerExplanation)
}
func (m *MockDockerClient) SwarmUpdate(ctx context.Context, version swarm.Version, swarm swarm.Spec,
	flags swarm.UpdateFlags) error {
	panic(mockDockerExplanation)
}
func (m *MockDockerClient) Events(ctx context.Context,
	options types.EventsOptions) (<-chan events.Message, <-chan error) {
	panic(mockDockerExplanation)
}
func (m *MockDockerClient) Info(ctx context.Context) (types.Info, error) {
	panic(mockDockerExplanation)
}
func (m *MockDockerClient) RegistryLogin(ctx context.Context,
	auth types.AuthConfig) (registry.AuthenticateOKBody, error) {
	panic(mockDockerExplanation)
}
func (m *MockDockerClient) DiskUsage(ctx context.Context) (types.DiskUsage, error) {
	panic(mockDockerExplanation)
}
func (m *MockDockerClient) Ping(ctx context.Context) (types.Ping, error) {
	panic(mockDockerExplanation)
}
func (m *MockDockerClient) VolumeCreate(ctx context.Context,
	options volumetypes.VolumesCreateBody) (types.Volume, error) {
	panic(mockDockerExplanation)
}
func (m *MockDockerClient) VolumeInspect(ctx context.Context, volumeID string) (types.Volume, error) {
	panic(mockDockerExplanation)
}
func (m *MockDockerClient) VolumeInspectWithRaw(ctx context.Context, volumeID string) (types.Volume, []byte, error) {
	panic(mockDockerExplanation)
}
func (m *MockDockerClient) VolumeList(ctx context.Context, filter filters.Args) (volumetypes.VolumesListOKBody, error) {
	panic(mockDockerExplanation)
}
func (m *MockDockerClient) VolumeRemove(ctx context.Context, volumeID string, force bool) error {
	panic(mockDockerExplanation)
}
func (m *MockDockerClient) VolumesPrune(ctx context.Context,
	pruneFilter filters.Args) (types.VolumesPruneReport, error) {
	panic(mockDockerExplanation)
}
func (m *MockDockerClient) SecretList(ctx context.Context, options types.SecretListOptions) ([]swarm.Secret, error) {
	panic(mockDockerExplanation)
}
func (m *MockDockerClient) SecretCreate(ctx context.Context,
	secret swarm.SecretSpec) (types.SecretCreateResponse, error) {
	panic(mockDockerExplanation)
}
func (m *MockDockerClient) SecretRemove(ctx context.Context, id string) error {
	panic(mockDockerExplanation)
}
func (m *MockDockerClient) SecretInspectWithRaw(ctx context.Context, name string) (swarm.Secret, []byte, error) {
	panic(mockDockerExplanation)
}
func (m *MockDockerClient) SecretUpdate(ctx context.Context, id string, version swarm.Version,
	secret swarm.SecretSpec) error {
	panic(mockDockerExplanation)
}
