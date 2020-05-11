package registry // import "github.com/docker/docker/registry"

import (
	reference "github.com/containerd/containerd/reference/docker"
	"github.com/docker/docker/api/types/registry"
)

// APIVersion is an integral representation of an API version (presently
// either 1 or 2)
type APIVersion int

func (av APIVersion) String() string {
	return apiVersions[av]
}

// API Version identifiers.
const (
	APIVersion1 APIVersion = 1
	APIVersion2 APIVersion = 2
)

var apiVersions = map[APIVersion]string{
	APIVersion1: "v1",
	APIVersion2: "v2",
}

// RepositoryInfo describes a repository
type RepositoryInfo struct {
	Name reference.Named
	// Index points to registry information
	Index *registry.IndexInfo
	// Official indicates whether the repository is considered official.
	// If the registry is official, and the normalized name does not
	// contain a '/' (e.g. "foo"), then it is considered an official repo.
	Official bool
	// Class represents the class of the repository, such as "plugin"
	// or "image".
	Class string
}
