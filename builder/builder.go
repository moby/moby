// Package builder defines interfaces for any Docker builder to implement.
//
// Historically, only server-side Dockerfile interpreters existed.
// This package allows for other implementations of Docker builders.
package builder

import (
	"io"
	"time"

	"github.com/docker/distribution/reference"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/backend"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/image"
	"golang.org/x/net/context"
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
	// TODO: use digest reference instead of name

	// GetImageOnBuild looks up a Docker image referenced by `name`.
	GetImageOnBuild(name string) (Image, error)
	// TagImageWithReference tags an image with newTag
	TagImageWithReference(image.ID, reference.Named) error
	// PullOnBuild tells Docker to pull image referenced by `name`.
	PullOnBuild(ctx context.Context, name string, authConfigs map[string]types.AuthConfig, output io.Writer) (Image, error)
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
	ContainerWait(containerID string, timeout time.Duration) (int, error)
	// ContainerUpdateCmdOnBuild updates container.Path and container.Args
	ContainerUpdateCmdOnBuild(containerID string, cmd []string) error
	// ContainerCreateWorkdir creates the workdir
	ContainerCreateWorkdir(containerID string) error

	// ContainerCopy copies/extracts a source FileInfo to a destination path inside a container
	// specified by a container object.
	// TODO: extract in the builder instead of passing `decompress`
	// TODO: use containerd/fs.changestream instead as a source
	CopyOnBuild(containerID string, destPath string, srcRoot string, srcPath string, decompress bool) error

	// HasExperimental checks if the backend supports experimental features
	HasExperimental() bool

	// SquashImage squashes the fs layers from the provided image down to the specified `to` image
	SquashImage(from string, to string) (string, error)

	// MountImage returns mounted path with rootfs of an image.
	MountImage(name string) (string, func() error, error)
}

// Image represents a Docker image used by the builder.
type Image interface {
	ImageID() string
	RunConfig() *container.Config
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
