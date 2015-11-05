// +build !windows

package container

import (
	"io"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/versions/v1p19"
	"github.com/docker/docker/api/types/versions/v1p20"
	"github.com/docker/docker/daemon"
	"github.com/docker/docker/pkg/archive"
	"github.com/docker/docker/runconfig"
)

// Backend is all the methods that need to be implemented to provide
// container specific functionality
type Backend interface {
	ContainerArchivePath(name string, path string) (content io.ReadCloser, stat *types.ContainerPathStat, err error)
	ContainerAttachWithLogs(prefixOrName string, c *daemon.ContainerAttachWithLogsConfig) error
	ContainerChanges(name string) ([]archive.Change, error)
	ContainerCopy(name string, res string) (io.ReadCloser, error)
	ContainerCreate(params *daemon.ContainerCreateConfig) (types.ContainerCreateResponse, error)
	ContainerExecCreate(config *runconfig.ExecConfig) (string, error)
	ContainerExecInspect(id string) (*daemon.ExecConfig, error)
	ContainerExecResize(name string, height, width int) error
	ContainerExecStart(name string, stdin io.ReadCloser, stdout io.Writer, stderr io.Writer) error
	ContainerExport(name string, out io.Writer) error
	ContainerExtractToDir(name, path string, noOverwriteDirNonDir bool, content io.Reader) error
	ContainerInspect(name string, size bool) (*types.ContainerJSON, error)
	ContainerInspect120(name string) (*v1p20.ContainerJSON, error)
	// unix version
	ContainerInspectPre120(name string) (*v1p19.ContainerJSON, error)
	// windows version
	//ContainerInspectPre120(name string) (*types.ContainerJSON, error)
	ContainerKill(name string, sig uint64) error
	ContainerLogs(containerName string, config *daemon.ContainerLogsConfig) error
	ContainerPause(name string) error
	ContainerRename(oldName, newName string) error
	ContainerResize(name string, height, width int) error
	ContainerRestart(name string, seconds int) error
	ContainerRm(name string, config *daemon.ContainerRmConfig) error
	Containers(config *daemon.ContainersConfig) ([]*types.Container, error)
	ContainerStart(name string, hostConfig *runconfig.HostConfig) error
	ContainerStatPath(name string, path string) (stat *types.ContainerPathStat, err error)
	ContainerStats(prefixOrName string, config *daemon.ContainerStatsConfig) error
	ContainerStop(name string, seconds int) error
	ContainerTop(name string, psArgs string) (*types.ContainerProcessList, error)
	ContainerUnpause(name string) error
	ContainerWait(name string, timeout time.Duration) (int, error)
	ContainerWsAttachWithLogs(prefixOrName string, c *daemon.ContainerWsAttachWithLogsConfig) error
	ExecExists(name string) (bool, error)
	Exists(id string) bool
	IsPaused(id string) bool
}
