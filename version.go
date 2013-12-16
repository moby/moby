package docker

import (
	"github.com/dotcloud/docker/engine"
	"github.com/dotcloud/docker/utils"
	"runtime"
)

func init() {
	engine.Register("version", jobVersion)
}

func jobVersion(job *engine.Job) engine.Status {
	if _, err := dockerVersion().WriteTo(job.Stdout); err != nil {
		job.Errorf("%s", err)
		return engine.StatusErr
	}
	return engine.StatusOK
}

// dockerVersion returns detailed version information in the form of a queriable
// environment.
func dockerVersion() *engine.Env {
	v := &engine.Env{}
	v.Set("Version", VERSION)
	v.Set("GitCommit", GITCOMMIT)
	v.Set("GoVersion", runtime.Version())
	// FIXME:utils.GetKernelVersion should only be needed here
	if kernelVersion, err := utils.GetKernelVersion(); err == nil {
		v.Set("KernelVersion", kernelVersion.String())
	}
	return v
}
