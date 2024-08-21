package supervisor // import "github.com/docker/docker/libcontainerd/supervisor"

import (
	"os"

	"github.com/docker/docker/pkg/process"
)

const (
	binaryName    = "containerd.exe"
	grpcPipeName  = `\\.\pipe\docker-containerd`
	debugPipeName = `\\.\pipe\docker-containerd-debug`
)

func defaultGRPCAddress(stateDir string) string {
	return grpcPipeName
}

func defaultDebugAddress(stateDir string) string {
	return debugPipeName
}

func (r *remote) stopDaemon() {
	p, err := os.FindProcess(r.daemonPid)
	if err != nil {
		r.logger.WithField("pid", r.daemonPid).Warn("could not find daemon process")
		return
	}

	if err = p.Kill(); err != nil {
		r.logger.WithError(err).WithField("pid", r.daemonPid).Warn("could not kill daemon process")
		return
	}

	_, err = p.Wait()
	if err != nil {
		r.logger.WithError(err).WithField("pid", r.daemonPid).Warn("wait for daemon process")
		return
	}
}

func (r *remote) killDaemon() {
	_ = process.Kill(r.daemonPid)
}

func (r *remote) platformCleanup() {
	// Nothing to do
}
