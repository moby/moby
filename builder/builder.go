// Package builder defines interfaces for any Docker builder to implement.
//
// Historically, only server-side Dockerfile interpreters existed.
// This package allows for other implementations of Docker builders.
package builder

import (
	"io"

	"golang.org/x/net/context"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/backend"
	"github.com/docker/docker/api/types/container"
	containerpkg "github.com/docker/docker/container"
)

const (
	// DefaultDockerfileName is the Default filename with Docker commands, read by docker build
	DefaultDockerfileName string = "Dockerfile"
)

// Source defines a location that can be used as a source for the ADD/COPY
// instructions in the builder.
type Source interface {
	// Root returns root path for accessing source
	Root() string
	// Close allows to signal that the filesystem tree won't be used anymore.
	// For Context implementations using a temporary directory, it is recommended to
	// delete the temporary directory in Close().
	Close() error
	// Hash returns a checksum for a file
	Hash(path string) (string, error)
}

// Backend abstracts calls to a Docker Daemon.
type Backend interface {
	ImageBackend

	// ContainerAttachRaw attaches to container.
	ContainerAttachRaw(cID string, stdin io.ReadCloser, stdout, stderr io.Writer, stream bool, attached chan struct{}) error
	// ContainerCreate creates a new Docker container and returns potential warnings
	ContainerCreate(config types.ContainerCreateConfig) (container.ContainerCreateCreatedBody, error)
	// ContainerRm removes a container specified by `id`.
	ContainerRm(name string, config *types.ContainerRmConfig) error
	// Commit creates a new Docker image from an existing Docker container.
	Commit(string, *backend.ContainerCommitConfig) (string, error)
	// ContainerKill stops the container execution abruptly.
	ContainerKill(containerID string, sig uint64) error
	// ContainerStart starts a new container
	ContainerStart(containerID string, hostConfig *container.HostConfig, checkpoint string, checkpointDir string) error
	// ContainerWait stops processing until the given container is stopped.
	ContainerWait(ctx context.Context, name string, condition containerpkg.WaitCondition) (<-chan containerpkg.StateStatus, error)
	// ContainerCreateWorkdir creates the workdir
	ContainerCreateWorkdir(containerID string) error

	// ContainerCopy copies/extracts a source FileInfo to a destination path inside a container
	// specified by a container object.
	// TODO: extract in the builder instead of passing `decompress`
	// TODO: use containerd/fs.changestream instead as a source
	CopyOnBuild(containerID string, destPath string, srcRoot string, srcPath string, decompress bool) error
}

// ImageBackend are the interface methods required from an image component
type ImageBackend interface {
	GetImageAndReleasableLayer(ctx context.Context, refOrID string, opts backend.GetImageAndLayerOptions) (Image, ReleaseableLayer, error)
}

// Result is the output produced by a Builder
type Result struct {
	ImageID   string
	FromImage Image
}

// ImageCacheBuilder represents a generator for stateful image cache.
type ImageCacheBuilder interface {
	// MakeImageCache creates a stateful image cache.
	MakeImageCache(cacheFrom []string) ImageCache
}

// ImageCache abstracts an image cache.
// (parent image, child runconfig) -> child image
type ImageCache interface {
	// GetCache returns a reference to a cached image whose parent equals `parent`
	// and runconfig equals `cfg`. A cache miss is expected to return an empty ID and a nil error.
	GetCache(parentID string, cfg *container.Config) (imageID string, err error)
}

// Image represents a Docker image used by the builder.
type Image interface {
	ImageID() string
	RunConfig() *container.Config
}

// ReleaseableLayer is an image layer that can be mounted and released
type ReleaseableLayer interface {
	Release() error
	Mount() (string, error)
}
