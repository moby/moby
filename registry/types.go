package registry // import "github.com/docker/docker/registry"

import (
	"github.com/distribution/reference"
	"github.com/docker/docker/api/types/registry"
)

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
	//
	// Deprecated: this field is no longer used, and will be removed in the next release.
	Class string
}
