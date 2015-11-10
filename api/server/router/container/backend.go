package container

import (
	"io"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/daemon"
	"github.com/docker/docker/pkg/archive"
	"github.com/docker/docker/runconfig"
)

// Backend has collection of container functions.
// TODO: group functions into certain functionality and create specific interfaces.
type Backend interface {
	// list containers
	Containers(config *daemon.ContainersConfig) ([]*types.Container, error)

	// container attribute update
	ContainerRename(oldName, newName string) error
	ContainerResize(name string, height, width int) error

	// container status functions
	ContainerAttachWithLogs(prefixOrName string, c *daemon.ContainerAttachWithLogsConfig) error
	ContainerLogs(containerName string, config *daemon.ContainerLogsConfig) error
	ContainerWsAttachWithLogs(prefixOrName string, c *daemon.ContainerWsAttachWithLogsConfig) error
	Exists(id string) bool
	IsPaused(id string) bool
	ContainerChanges(name string) ([]archive.Change, error)
	ContainerTop(name string, psArgs string) (*types.ContainerProcessList, error)
	ContainerStats(prefixOrName string, config *daemon.ContainerStatsConfig) error

	// container archive functions
	ContainerArchivePath(name string, path string) (content io.ReadCloser, stat *types.ContainerPathStat, err error)
	ContainerCopy(name string, res string) (io.ReadCloser, error)
	ContainerExport(name string, out io.Writer) error
	ContainerExtractToDir(name, path string, noOverwriteDirNonDir bool, content io.Reader) error
	ContainerStatPath(name string, path string) (stat *types.ContainerPathStat, err error)

	// container control functions
	ContainerCreate(params *daemon.ContainerCreateConfig) (types.ContainerCreateResponse, error)
	ContainerRestart(name string, seconds int) error
	ContainerKill(name string, sig uint64) error
	ContainerStart(name string, hostConfig *runconfig.HostConfig) error
	ContainerPause(name string) error
	ContainerUnpause(name string) error
	ContainerStop(name string, seconds int) error
	ContainerRm(name string, config *daemon.ContainerRmConfig) error
	ContainerWait(name string, timeout time.Duration) (int, error)
}
