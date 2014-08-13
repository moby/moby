package builder

import (
	"github.com/docker/docker/builder/evaluator"
	"github.com/docker/docker/runconfig"
)

// Create a new builder.
func NewBuilder(opts *evaluator.BuildOpts) *evaluator.BuildFile {
	return &evaluator.BuildFile{
		Dockerfile:    nil,
		Config:        &runconfig.Config{},
		Options:       opts,
		TmpContainers: evaluator.UniqueMap{},
		TmpImages:     evaluator.UniqueMap{},
	}
}
