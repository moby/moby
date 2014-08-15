package builder

import (
	"github.com/docker/docker/runconfig"
)

// Create a new builder. See
func NewBuilder(opts *BuildOpts) *BuildFile {
	return &BuildFile{
		Dockerfile:    nil,
		Config:        &runconfig.Config{},
		Options:       opts,
		TmpContainers: UniqueMap{},
		TmpImages:     UniqueMap{},
	}
}
