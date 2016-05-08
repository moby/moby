package registry

import (
	"github.com/docker/docker/reference"
	registrytypes "github.com/docker/engine-api/types/registry"
)

// RepositoryData tracks the image list, list of endpoints, and list of tokens
// for a repository
type RepositoryData struct {
	// ImgList is a list of images in the repository
	ImgList map[string]*ImgData
	// Endpoints is a list of endpoints returned in X-Docker-Endpoints
	Endpoints []string
	// Tokens is currently unused (remove it?)
	Tokens []string
}

// ImgData is used to transfer image checksums to and from the registry
type ImgData struct {
	// ID is an opaque string that identifies the image
	ID              string `json:"id"`
	Checksum        string `json:"checksum,omitempty"`
	ChecksumPayload string `json:"-"`
	Tag             string `json:",omitempty"`
}

// PingResult contains the information returned when pinging a registry. It
// indicates the registry's version and whether the registry claims to be a
// standalone registry.
type PingResult struct {
	// Version is the registry version supplied by the registry in an HTTP
	// header
	Version string `json:"version"`
	// Standalone is set to true if the registry indicates it is a
	// standalone registry in the X-Docker-Registry-Standalone
	// header
	Standalone bool `json:"standalone"`
}

// APIVersion is an integral representation of an API version (presently
// either 1 or 2)
type APIVersion int

func (av APIVersion) String() string {
	return apiVersions[av]
}

// API Version identifiers.
const (
	_                      = iota
	APIVersion1 APIVersion = iota
	APIVersion2
)

var apiVersions = map[APIVersion]string{
	APIVersion1: "v1",
	APIVersion2: "v2",
}

// RepositoryInfo describes a repository
type RepositoryInfo struct {
	reference.Named
	// Index points to registry information
	Index *registrytypes.IndexInfo
	// Official indicates whether the repository is considered official.
	// If the registry is official, and the normalized name does not
	// contain a '/' (e.g. "foo"), then it is considered an official repo.
	Official bool
}
