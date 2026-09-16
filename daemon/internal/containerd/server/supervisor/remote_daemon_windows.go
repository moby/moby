package supervisor

import (
	"os"

	"github.com/moby/moby/v2/pkg/process"
)

const (
	binaryName    = "containerd.exe"
	grpcPipeName  = `\\.\pipe\docker-containerd`
	debugPipeName = `\\.\pipe\docker-containerd-debug`
)

func defaultGRPCAddress(_ string) string {
	return grpcPipeName
}

func defaultDebugAddress(_ string) string {
	return debugPipeName
}

func (r *Daemon) stopDaemon() {
	p, err := os.FindProcess(r.daemonPid)
	if err != nil {
		r.logger.WithField("pid", r.daemonPid).Warn("could not find containerd process")
		return
	}

	if err = p.Kill(); err != nil {
		r.logger.WithError(err).WithField("pid", r.daemonPid).Warn("could not kill containerd process")
		return
	}

	_, err = p.Wait()
	if err != nil {
		r.logger.WithError(err).WithField("pid", r.daemonPid).Warn("wait for containerd process")
		return
	}
}

func (r *Daemon) killDaemon() {
	_ = process.Kill(r.daemonPid)
}

func (r *Daemon) platformCleanup() {
	// Nothing to do
}
