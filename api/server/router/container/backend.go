package container // import "github.com/docker/docker/api/server/router/container"

import (
	"context"
	"io"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/backend"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	containerpkg "github.com/docker/docker/container"
	"github.com/docker/docker/pkg/archive"
)

// execBackend includes functions to implement to provide exec functionality.
type execBackend interface {
	ContainerExecCreate(ctx context.Context, name string, config *types.ExecConfig) (string, error)
	ContainerExecInspect(ctx context.Context, id string) (*backend.ExecInspect, error)
	ContainerExecResize(ctx context.Context, name string, height, width int) error
	ContainerExecStart(ctx context.Context, name string, stdin io.Reader, stdout io.Writer, stderr io.Writer) error
	ExecExists(ctx context.Context, name string) (bool, error)
}

// copyBackend includes functions to implement to provide container copy functionality.
type copyBackend interface {
	ContainerArchivePath(ctx context.Context, name string, path string) (content io.ReadCloser, stat *types.ContainerPathStat, err error)
	ContainerCopy(ctx context.Context, name string, res string) (io.ReadCloser, error)
	ContainerExport(ctx context.Context, name string, out io.Writer) error
	ContainerExtractToDir(ctx context.Context, name, path string, copyUIDGID, noOverwriteDirNonDir bool, content io.Reader) error
	ContainerStatPath(ctx context.Context, name string, path string) (stat *types.ContainerPathStat, err error)
}

// stateBackend includes functions to implement to provide container state lifecycle functionality.
type stateBackend interface {
	ContainerCreate(ctx context.Context, config types.ContainerCreateConfig) (container.ContainerCreateCreatedBody, error)
	ContainerKill(ctx context.Context, name string, sig uint64) error
	ContainerPause(ctx context.Context, name string) error
	ContainerRename(ctx context.Context, oldName, newName string) error
	ContainerResize(ctx context.Context, name string, height, width int) error
	ContainerRestart(ctx context.Context, name string, seconds *int) error
	ContainerRm(ctx context.Context, name string, config *types.ContainerRmConfig) error
	ContainerStart(ctx context.Context, name string, hostConfig *container.HostConfig, checkpoint string, checkpointDir string) error
	ContainerStop(ctx context.Context, name string, seconds *int) error
	ContainerUnpause(ctx context.Context, name string) error
	ContainerUpdate(ctx context.Context, name string, hostConfig *container.HostConfig) (container.ContainerUpdateOKBody, error)
	ContainerWait(ctx context.Context, name string, condition containerpkg.WaitCondition) (<-chan containerpkg.StateStatus, error)
}

// monitorBackend includes functions to implement to provide containers monitoring functionality.
type monitorBackend interface {
	ContainerChanges(ctx context.Context, name string) ([]archive.Change, error)
	ContainerInspect(ctx context.Context, name string, size bool, version string) (interface{}, error)
	ContainerLogs(ctx context.Context, name string, config *types.ContainerLogsOptions) (msgs <-chan *backend.LogMessage, tty bool, err error)
	ContainerStats(ctx context.Context, name string, config *backend.ContainerStatsConfig) error
	ContainerTop(ctx context.Context, name string, psArgs string) (*container.ContainerTopOKBody, error)

	Containers(ctx context.Context, config *types.ContainerListOptions) ([]*types.Container, error)
}

// attachBackend includes function to implement to provide container attaching functionality.
type attachBackend interface {
	ContainerAttach(ctx context.Context, name string, c *backend.ContainerAttachConfig) error
}

// systemBackend includes functions to implement to provide system wide containers functionality
type systemBackend interface {
	ContainersPrune(ctx context.Context, pruneFilters filters.Args) (*types.ContainersPruneReport, error)
}

type commitBackend interface {
	CreateImageFromContainer(ctx context.Context, name string, config *backend.CreateImageConfig) (imageID string, err error)
}

// Backend is all the methods that need to be implemented to provide container specific functionality.
type Backend interface {
	commitBackend
	execBackend
	copyBackend
	stateBackend
	monitorBackend
	attachBackend
	systemBackend
}
