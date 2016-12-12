// Package builder defines interfaces for any Docker builder to implement.
//
// Historically, only server-side Dockerfile interpreters existed.
// This package allows for other implementations of Docker builders.
package builder

import (
	"io"
	"os"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/backend"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/image"
	"github.com/docker/docker/reference"
	"golang.org/x/net/context"
)

const (
	// DefaultDockerfileName is the Default filename with Docker commands, read by docker build
	DefaultDockerfileName string = "Dockerfile"
)

// Context represents a file system tree.
type Context interface {
	// Close allows to signal that the filesystem tree won't be used anymore.
	// For Context implementations using a temporary directory, it is recommended to
	// delete the temporary directory in Close().
	Close() error
	// Stat returns an entry corresponding to path if any.
	// It is recommended to return an error if path was not found.
	// If path is a symlink it also returns the path to the target file.
	Stat(path string) (string, FileInfo, error)
	// Open opens path from the context and returns a readable stream of it.
	Open(path string) (io.ReadCloser, error)
	// Walk walks the tree of the context with the function passed to it.
	Walk(root string, walkFn WalkFunc) error
}

// WalkFunc is the type of the function called for each file or directory visited by Context.Walk().
type WalkFunc func(path string, fi FileInfo, err error) error

// ModifiableContext represents a modifiable Context.
// TODO: remove this interface once we can get rid of Remove()
type ModifiableContext interface {
	Context
	// Remove deletes the entry specified by `path`.
	// It is usual for directory entries to delete all its subentries.
	Remove(path string) error
}

// FileInfo extends os.FileInfo to allow retrieving an absolute path to the file.
// TODO: remove this interface once pkg/archive exposes a walk function that Context can use.
type FileInfo interface {
	os.FileInfo
	Path() string
}

// PathFileInfo is a convenience struct that implements the FileInfo interface.
type PathFileInfo struct {
	os.FileInfo
	// FilePath holds the absolute path to the file.
	FilePath string
	// FileName holds the basename for the file.
	FileName string
}

// Path returns the absolute path to the file.
func (fi PathFileInfo) Path() string {
	return fi.FilePath
}

// Name returns the basename of the file.
func (fi PathFileInfo) Name() string {
	if fi.FileName != "" {
		return fi.FileName
	}
	return fi.FileInfo.Name()
}

// Hashed defines an extra method intended for implementations of os.FileInfo.
type Hashed interface {
	// Hash returns the hash of a file.
	Hash() string
	SetHash(string)
}

// HashedFileInfo is a convenient struct that augments FileInfo with a field.
type HashedFileInfo struct {
	FileInfo
	// FileHash represents the hash of a file.
	FileHash string
}

// Hash returns the hash of a file.
func (fi HashedFileInfo) Hash() string {
	return fi.FileHash
}

// SetHash sets the hash of a file.
func (fi *HashedFileInfo) SetHash(h string) {
	fi.FileHash = h
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
	ContainerAttachRaw(cID string, stdin io.ReadCloser, stdout, stderr io.Writer, stream bool) error
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
	// TODO: make an Extract method instead of passing `decompress`
	// TODO: do not pass a FileInfo, instead refactor the archive package to export a Walk function that can be used
	// with Context.Walk
	// ContainerCopy(name string, res string) (io.ReadCloser, error)
	// TODO: use copyBackend api
	CopyOnBuild(containerID string, destPath string, src FileInfo, decompress bool) error

	// HasExperimental checks if the backend supports experimental features
	HasExperimental() bool

	// SquashImage squashes the fs layers from the provided image down to the specified `to` image
	SquashImage(from string, to string) (string, error)
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
