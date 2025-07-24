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
}
