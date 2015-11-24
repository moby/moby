package local

import (
	"io"
	"time"

	"github.com/docker/docker/pkg/archive"
	"github.com/docker/docker/pkg/idtools"
	"github.com/docker/docker/pkg/jsonmessage"
	"github.com/docker/docker/pkg/parsers/filters"

	// TODO return types need to be refactored into pkg
	// consists mainly of XYZConfig structs to pass information
	"github.com/docker/docker/api/types"                // bunch of return types
	"github.com/docker/docker/api/types/versions/v1p20" // container json
	"github.com/docker/docker/cliconfig"                // auth config
	"github.com/docker/docker/daemon"                   // many container configs
	"github.com/docker/docker/daemon/events"            // event format
	"github.com/docker/docker/graph"                    // image pull config
	"github.com/docker/docker/registry"                 // registry.searchresults
	"github.com/docker/docker/runconfig"                // container run config
)

// ImageBackend is the set of methods that provide image specific
// functionality
type ImageBackend interface {
	ImageDelete(imageRef string, force, prune bool) ([]types.ImageDelete, error)
	TagImage(repoName, tag, imageName string, force bool) error
	PullImage(image string, tag string,
		imagePullConfig *graph.ImagePullConfig) error
	LoadImage(inTar io.ReadCloser, outStream io.Writer) error
	LookupImage(name string) (*types.ImageInspect, error)

	ImportImage(src, repo, tag, msg string, inConfig io.ReadCloser,
		outStream io.Writer, containerConfig *runconfig.Config) error
	ExportImage(names []string, outStream io.Writer) error
	PushImage(localName string, imagePushConfig *graph.ImagePushConfig) error
	ListImages(filterArgs, filter string, all bool) ([]*types.Image, error)
	ImageHistory(name string) ([]*types.ImageHistory, error)
	AuthenticateToRegistry(authConfig *cliconfig.AuthConfig) (string, error)
	SearchRegistryForImages(term string,
		authConfig *cliconfig.AuthConfig,
		headers map[string][]string) (*registry.SearchResults, error)
}

// ContainerBackend is the set of methods that provide contianer
// specific functionality
type ContainerBackend interface {
	Exists(id string) bool

	ContainerCopy(name string, res string) (io.ReadCloser, error)
	ContainerStatPath(name string,
		path string) (stat *types.ContainerPathStat, err error)
	ContainerArchivePath(name string,
		path string) (content io.ReadCloser,
		stat *types.ContainerPathStat, err error)
	ContainerExtractToDir(name, path string, noOverwriteDirNonDir bool,
		content io.Reader) error

	ContainerInspect(name string, size bool) (*types.ContainerJSON, error)
	ContainerInspect120(name string) (*v1p20.ContainerJSON, error)
	// unix and windows have differing return types
	InspectPre120Backend

	Containers(config *daemon.ContainersConfig) ([]*types.Container, error)
	ContainerStats(prefixOrName string,
		config *daemon.ContainerStatsConfig) error
	ContainerLogs(containerName string,
		logsConfig *daemon.ContainerLogsConfig) error
	ContainerExport(name string, out io.Writer) error

	ContainerStart(name string, hostConfig *runconfig.HostConfig) error
	ContainerStop(name string, seconds int) error
	ContainerKill(name string, sig uint64) error
	ContainerRestart(name string, seconds int) error
	ContainerPause(name string) error
	IsPaused(id string) bool
	ContainerUnpause(name string) error
	ContainerWait(name string, timeout time.Duration) (int, error)

	ContainerChanges(name string) ([]archive.Change, error)

	ContainerTop(name string, psArgs string) (*types.ContainerProcessList, error)
	ContainerRename(oldName, newName string) error

	ContainerCreate(containerCreateConfig *daemon.ContainerCreateConfig) (types.ContainerCreateResponse, error)

	ContainerRm(name string, config *daemon.ContainerRmConfig) error
	ContainerResize(name string, height, width int) error
	ContainerExecResize(name string, height, width int) error

	ContainerAttachWithLogs(prefixOrName string,
		c *daemon.ContainerAttachWithLogsConfig) error
	ContainerWsAttachWithLogs(prefixOrName string,
		c *daemon.ContainerWsAttachWithLogsConfig) error

	ContainerExecStart(execName string, stdin io.ReadCloser,
		stdout io.Writer, stderr io.Writer) error
	// two different versions of ExecConfig, oi vey!
	// TODO: investigate making these the same struct
	ContainerExecCreate(config *runconfig.ExecConfig) (string, error)
	ContainerExecInspect(id string) (*daemon.ExecConfig, error)

	ExecExists(name string) (bool, error)

	GetUIDGIDMaps() ([]idtools.IDMap, []idtools.IDMap)
	GetRemappedUIDGID() (int, int)
}

// EngineBackend is the set of methods that provide engine specific
// functionality.
type EngineBackend interface {
	SystemInfo() (*types.Info, error)

	GetEventFilter(filter filters.Args) *events.Filter
	SubscribeToEvents() ([]*jsonmessage.JSONMessage,
		chan interface{}, func())
}

// Backend is all the methods that need to be implemented
type Backend interface {
	ImageBackend
	ContainerBackend
	EngineBackend
}
