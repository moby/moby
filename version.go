package docker

import (
	"github.com/dotcloud/docker/dockerversion"
	"github.com/dotcloud/docker/engine"
	"github.com/dotcloud/docker/utils"
	"runtime"
)

// dockerVersion returns detailed version information in the form of a queriable
// environment.
func dockerVersion() *engine.Env {
	v := &engine.Env{}
	v.Set("Version", dockerversion.VERSION)
	v.Set("GitCommit", dockerversion.GITCOMMIT)
	v.Set("GoVersion", runtime.Version())
	v.Set("Os", runtime.GOOS)
	v.Set("Arch", runtime.GOARCH)
	// FIXME:utils.GetKernelVersion should only be needed here
	if kernelVersion, err := utils.GetKernelVersion(); err == nil {
		v.Set("KernelVersion", kernelVersion.String())
	}
	return v
}
