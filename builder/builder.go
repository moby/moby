package builder

import (
	"github.com/docker/docker/builder/evaluator"
	"github.com/docker/docker/nat"
	"github.com/docker/docker/runconfig"
)

// Create a new builder.
func NewBuilder(opts *evaluator.BuildOpts) *evaluator.BuildFile {
	return &evaluator.BuildFile{
		Dockerfile:    nil,
		Env:           evaluator.EnvMap{},
		Config:        initRunConfig(),
		Options:       opts,
		TmpContainers: evaluator.UniqueMap{},
		TmpImages:     evaluator.UniqueMap{},
	}
}

func initRunConfig() *runconfig.Config {
	return &runconfig.Config{
		PortSpecs: []string{},
		// FIXME(erikh) this should be a type that lives in runconfig
		ExposedPorts: map[nat.Port]struct{}{},
		Env:          []string{},
		Cmd:          []string{},

		// FIXME(erikh) this should also be a type in runconfig
		Volumes:    map[string]struct{}{},
		Entrypoint: []string{"/bin/sh", "-c"},
		OnBuild:    []string{},
	}
}
