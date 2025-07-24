package registry

import (
	"github.com/distribution/reference"
	"github.com/moby/moby/api/types/registry"
)

// RepositoryInfo describes a repository
type RepositoryInfo struct {
	Name reference.Named
	// Index points to registry information
	Index *registry.IndexInfo
	// Class represents the class of the repository, such as "plugin"
	// or "image".
	//
	// Deprecated: this field is no longer used, and will be removed in the next release.
	Class string
}
