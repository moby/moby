package client

import (
	"io"

	"golang.org/x/net/context"

	"github.com/docker/engine-api/types"
	"github.com/docker/engine-api/types/container"
	"github.com/docker/engine-api/types/filters"
	"github.com/docker/engine-api/types/network"
	"github.com/docker/engine-api/types/registry"
)

// APIClient is an interface that clients that talk with a docker server must implement.
type APIClient interface {
	ClientVersion() string
	ContainerAttach(options types.ContainerAttachOptions) (types.HijackedResponse, error)
	ContainerCommit(options types.ContainerCommitOptions) (types.ContainerCommitResponse, error)
	ContainerCreate(config *container.Config, hostConfig *container.HostConfig, networkingConfig *network.NetworkingConfig, containerName string) (types.ContainerCreateResponse, error)
	ContainerDiff(containerID string) ([]types.ContainerChange, error)
	ContainerExecAttach(execID string, config types.ExecConfig) (types.HijackedResponse, error)
	ContainerExecCreate(config types.ExecConfig) (types.ContainerExecCreateResponse, error)
	ContainerExecInspect(execID string) (types.ContainerExecInspect, error)
	ContainerExecResize(options types.ResizeOptions) error
	ContainerExecStart(execID string, config types.ExecStartCheck) error
	ContainerExport(ctx context.Context, containerID string) (io.ReadCloser, error)
	ContainerInspect(containerID string) (types.ContainerJSON, error)
	ContainerInspectWithRaw(containerID string, getSize bool) (types.ContainerJSON, []byte, error)
	ContainerKill(containerID, signal string) error
	ContainerList(options types.ContainerListOptions) ([]types.Container, error)
	ContainerLogs(ctx context.Context, options types.ContainerLogsOptions) (io.ReadCloser, error)
	ContainerPause(containerID string) error
	ContainerRemove(options types.ContainerRemoveOptions) error
	ContainerRename(containerID, newContainerName string) error
	ContainerResize(options types.ResizeOptions) error
	ContainerRestart(containerID string, timeout int) error
	ContainerStatPath(containerID, path string) (types.ContainerPathStat, error)
	ContainerStats(ctx context.Context, containerID string, stream bool) (io.ReadCloser, error)
	ContainerStart(containerID string) error
	ContainerStop(containerID string, timeout int) error
	ContainerTop(containerID string, arguments []string) (types.ContainerProcessList, error)
	ContainerUnpause(containerID string) error
	ContainerUpdate(containerID string, updateConfig container.UpdateConfig) error
	ContainerWait(ctx context.Context, containerID string) (int, error)
	CopyFromContainer(ctx context.Context, containerID, srcPath string) (io.ReadCloser, types.ContainerPathStat, error)
	CopyToContainer(ctx context.Context, options types.CopyToContainerOptions) error
	Events(ctx context.Context, options types.EventsOptions) (io.ReadCloser, error)
	ImageBuild(ctx context.Context, options types.ImageBuildOptions) (types.ImageBuildResponse, error)
	ImageCreate(ctx context.Context, options types.ImageCreateOptions) (io.ReadCloser, error)
	ImageHistory(imageID string) ([]types.ImageHistory, error)
	ImageImport(ctx context.Context, options types.ImageImportOptions) (io.ReadCloser, error)
	ImageInspectWithRaw(imageID string, getSize bool) (types.ImageInspect, []byte, error)
	ImageList(options types.ImageListOptions) ([]types.Image, error)
	ImageLoad(ctx context.Context, input io.Reader, quiet bool) (types.ImageLoadResponse, error)
	ImagePull(ctx context.Context, options types.ImagePullOptions, privilegeFunc RequestPrivilegeFunc) (io.ReadCloser, error)
	ImagePush(ctx context.Context, options types.ImagePushOptions, privilegeFunc RequestPrivilegeFunc) (io.ReadCloser, error)
	ImageRemove(options types.ImageRemoveOptions) ([]types.ImageDelete, error)
	ImageSearch(options types.ImageSearchOptions, privilegeFunc RequestPrivilegeFunc) ([]registry.SearchResult, error)
	ImageSave(ctx context.Context, imageIDs []string) (io.ReadCloser, error)
	ImageTag(options types.ImageTagOptions) error
	Info() (types.Info, error)
	NetworkConnect(networkID, containerID string, config *network.EndpointSettings) error
	NetworkCreate(options types.NetworkCreate) (types.NetworkCreateResponse, error)
	NetworkDisconnect(networkID, containerID string, force bool) error
	NetworkInspect(networkID string) (types.NetworkResource, error)
	NetworkList(options types.NetworkListOptions) ([]types.NetworkResource, error)
	NetworkRemove(networkID string) error
	RegistryLogin(auth types.AuthConfig) (types.AuthResponse, error)
	ServerVersion() (types.Version, error)
	VolumeCreate(options types.VolumeCreateRequest) (types.Volume, error)
	VolumeInspect(volumeID string) (types.Volume, error)
	VolumeList(filter filters.Args) (types.VolumesListResponse, error)
	VolumeRemove(volumeID string) error
}

// Ensure that Client always implements APIClient.
var _ APIClient = &Client{}
