// Package builder defines interfaces for any Docker builder to implement.
//
// Historically, only server-side Dockerfile interpreters existed.
// This package allows for other implementations of Docker builders.
package builder

import (
	"io"
	"os"

	// TODO: remove dependency on daemon
	"github.com/docker/docker/daemon"
	"github.com/docker/docker/image"
	"github.com/docker/docker/runconfig"
)

// Builder abstracts a Docker builder whose only purpose is to build a Docker image referenced by an imageID.
type Builder interface {
	// Build builds a Docker image referenced by an imageID string.
	//
	// Note: Tagging an image should not be done by a Builder, it should instead be done
	// by the caller.
	//
	// TODO: make this return a reference instead of string
	Build() (imageID string)
}

// Context represents a file system tree.
type Context interface {
	// Close allows to signal that the filesystem tree won't be used anymore.
	// For Context implementations using a temporary directory, it is recommended to
	// delete the temporary directory in Close().
	Close() error
	// Stat returns an entry corresponding to path if any.
	// It is recommended to return an error if path was not found.
	Stat(path string) (FileInfo, error)
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
}

// Path returns the absolute path to the file.
func (fi PathFileInfo) Path() string {
	return fi.FilePath
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

// Docker abstracts calls to a Docker Daemon.
type Docker interface {
	// TODO: use digest reference instead of name

	// LookupImage looks up a Docker image referenced by `name`.
	LookupImage(name string) (*image.Image, error)
	// Pull tells Docker to pull image referenced by `name`.
	Pull(name string) (*image.Image, error)

	// TODO: move daemon.Container to its own package

	// Container looks up a Docker container referenced by `id`.
	Container(id string) (*daemon.Container, error)
	// Create creates a new Docker container and returns potential warnings
	// TODO: put warnings in the error
	Create(*runconfig.Config, *runconfig.HostConfig) (*daemon.Container, []string, error)
	// Remove removes a container specified by `id`.
	Remove(id string, cfg *daemon.ContainerRmConfig) error
	// Commit creates a new Docker image from an existing Docker container.
	Commit(*daemon.Container, *daemon.ContainerCommitConfig) (*image.Image, error)
	// Copy copies/extracts a source FileInfo to a destination path inside a container
	// specified by a container object.
	// TODO: make an Extract method instead of passing `decompress`
	// TODO: do not pass a FileInfo, instead refactor the archive package to export a Walk function that can be used
	// with Context.Walk
	Copy(c *daemon.Container, destPath string, src FileInfo, decompress bool) error

	// Retain retains an image avoiding it to be removed or overwritten until a corresponding Release() call.
	// TODO: remove
	Retain(sessionID, imgID string)
	// Release releases a list of images that were retained for the time of a build.
	// TODO: remove
	Release(sessionID string, activeImages []string)
}

// ImageCache abstracts an image cache store.
// (parent image, child runconfig) -> child image
type ImageCache interface {
	// GetCachedImage returns a reference to a cached image whose parent equals `parent`
	// and runconfig equals `cfg`. A cache miss is expected to return an empty ID and a nil error.
	GetCachedImage(parentID string, cfg *runconfig.Config) (imageID string, err error)
}
