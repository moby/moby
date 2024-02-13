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
	ContainerExecCreate(name string, config *types.ExecConfig) (string, error)
	ContainerExecInspect(id string) (*backend.ExecInspect, error)
	ContainerExecResize(name string, height, width int) error
	ContainerExecStart(ctx context.Context, name string, options container.ExecStartOptions) error
	ExecExists(name string) (bool, error)
}

// copyBackend includes functions to implement to provide container copy functionality.
type copyBackend interface {
	ContainerArchivePath(name string, path string) (content io.ReadCloser, stat *types.ContainerPathStat, err error)
	ContainerExport(ctx context.Context, name string, out io.Writer) error
	ContainerExtractToDir(name, path string, copyUIDGID, noOverwriteDirNonDir bool, content io.Reader) error
	ContainerStatPath(name string, path string) (stat *types.ContainerPathStat, err error)
}

// stateBackend includes functions to implement to provide container state lifecycle functionality.
type stateBackend interface {
	ContainerCreate(ctx context.Context, config backend.ContainerCreateConfig) (container.CreateResponse, error)
	ContainerKill(name string, signal string) error
	ContainerPause(name string) error
	ContainerRename(oldName, newName string) error
	ContainerResize(name string, height, width int) error
	ContainerRestart(ctx context.Context, name string, options container.StopOptions) error
	ContainerRm(name string, config *backend.ContainerRmConfig) error
	ContainerStart(ctx context.Context, name string, checkpoint string, checkpointDir string) error
	ContainerStop(ctx context.Context, name string, options container.StopOptions) error
	ContainerUnpause(name string) error
	ContainerUpdate(name string, hostConfig *container.HostConfig) (container.ContainerUpdateOKBody, error)
	ContainerWait(ctx context.Context, name string, condition containerpkg.WaitCondition) (<-chan containerpkg.StateStatus, error)
}

// monitorBackend includes functions to implement to provide containers monitoring functionality.
type monitorBackend interface {
	ContainerChanges(ctx context.Context, name string) ([]archive.Change, error)
	ContainerInspect(ctx context.Context, name string, size bool, version string) (interface{}, error)
	ContainerLogs(ctx context.Context, name string, config *container.LogsOptions) (msgs <-chan *backend.LogMessage, tty bool, err error)
	ContainerStats(ctx context.Context, name string, config *backend.ContainerStatsConfig) error
	ContainerTop(name string, psArgs string) (*container.ContainerTopOKBody, error)
	Containers(ctx context.Context, config *container.ListOptions) ([]*types.Container, error)
}

// attachBackend includes function to implement to provide container attaching functionality.
type attachBackend interface {
	ContainerAttach(name string, c *backend.ContainerAttachConfig) error
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
