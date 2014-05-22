package runconfig

import (
	"github.com/dotcloud/docker/engine"
)

// A run config contains configuration options that only affect a single
// run instance. The data in this is never serialized to disk outside
// the container and will not be reused the next time the container
// starts
type RunConfig struct {
	PrivateFiles map[string][]byte
}

// This can be used when you want to pass a config and optionally a RunConfig
type HostAndRunConfig struct {
	*HostConfig
	RunConfig *RunConfig
}

func ContainerRunConfigFromJob(job *engine.Job) *RunConfig {
	// If no environment was set, then no hostconfig was passed.
	if job.EnvExists("RunConfig") {
		runConfig := RunConfig{}
		job.GetenvJson("RunConfig", &runConfig)
		return &runConfig
	}
	return nil
}
