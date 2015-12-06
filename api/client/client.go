// Package client provides a command-line interface for Docker.
//
// Run "docker help SUBCOMMAND" or "docker SUBCOMMAND --help" to see more information on any Docker subcommand, including the full list of options supported for the subcommand.
// See https://docs.docker.com/installation/ for instructions on installing Docker.
package client

import (
	"io"

	"github.com/docker/docker/api/client/lib"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/cliconfig"
	"github.com/docker/docker/pkg/parsers/filters"
	"github.com/docker/docker/runconfig"
)

// apiClient is an interface that clients that talk with a docker server must implement.
type apiClient interface {
	ContainerAttach(options types.ContainerAttachOptions) (types.HijackedResponse, error)
	ContainerCommit(options types.ContainerCommitOptions) (types.ContainerCommitResponse, error)
	ContainerCreate(config *runconfig.ContainerConfigWrapper, containerName string) (types.ContainerCreateResponse, error)
	ContainerDiff(containerID string) ([]types.ContainerChange, error)
	ContainerExecAttach(execID string, config runconfig.ExecConfig) (types.HijackedResponse, error)
	ContainerExecCreate(config runconfig.ExecConfig) (types.ContainerExecCreateResponse, error)
	ContainerExecInspect(execID string) (types.ContainerExecInspect, error)
	ContainerExecStart(execID string, config types.ExecStartCheck) error
	ContainerExport(containerID string) (io.ReadCloser, error)
	ContainerInspect(containerID string) (types.ContainerJSON, error)
	ContainerKill(containerID, signal string) error
	ContainerList(options types.ContainerListOptions) ([]types.Container, error)
	ContainerLogs(options types.ContainerLogsOptions) (io.ReadCloser, error)
	ContainerPause(containerID string) error
	ContainerRemove(options types.ContainerRemoveOptions) error
	ContainerRename(containerID, newContainerName string) error
	ContainerRestart(containerID string, timeout int) error
	ContainerStatPath(containerID, path string) (types.ContainerPathStat, error)
	ContainerStats(containerID string, stream bool) (io.ReadCloser, error)
	ContainerStart(containerID string) error
	ContainerStop(containerID string, timeout int) error
	ContainerTop(containerID string, arguments []string) (types.ContainerProcessList, error)
	ContainerUnpause(containerID string) error
	ContainerWait(containerID string) (int, error)
	CopyFromContainer(containerID, srcPath string) (io.ReadCloser, types.ContainerPathStat, error)
	CopyToContainer(options types.CopyToContainerOptions) error
	Events(options types.EventsOptions) (io.ReadCloser, error)
	ImageBuild(options types.ImageBuildOptions) (types.ImageBuildResponse, error)
	ImageCreate(options types.ImageCreateOptions) (io.ReadCloser, error)
	ImageHistory(imageID string) ([]types.ImageHistory, error)
	ImageImport(options types.ImageImportOptions) (io.ReadCloser, error)
	ImageList(options types.ImageListOptions) ([]types.Image, error)
	ImageLoad(input io.Reader) (io.ReadCloser, error)
	ImagePull(options types.ImagePullOptions, privilegeFunc lib.RequestPrivilegeFunc) (io.ReadCloser, error)
	ImageRemove(options types.ImageRemoveOptions) ([]types.ImageDelete, error)
	ImageSave(imageIDs []string) (io.ReadCloser, error)
	ImageTag(options types.ImageTagOptions) error
	Info() (types.Info, error)
	NetworkConnect(networkID, containerID string) error
	NetworkCreate(options types.NetworkCreate) (types.NetworkCreateResponse, error)
	NetworkDisconnect(networkID, containerID string) error
	NetworkInspect(networkID string) (types.NetworkResource, error)
	NetworkList() ([]types.NetworkResource, error)
	NetworkRemove(networkID string) error
	RegistryLogin(auth cliconfig.AuthConfig) (types.AuthResponse, error)
	SystemVersion() (types.VersionResponse, error)
	VolumeCreate(options types.VolumeCreateRequest) (types.Volume, error)
	VolumeInspect(volumeID string) (types.Volume, error)
	VolumeList(filter filters.Args) (types.VolumesListResponse, error)
	VolumeRemove(volumeID string) error
}
