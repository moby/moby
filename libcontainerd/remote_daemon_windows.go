// +build remote_daemon

package libcontainerd

import (
	"os"
)

const (
	grpcPipeName  = `\\.\pipe\docker-containerd-containerd`
	debugPipeName = `\\.\pipe\docker-containerd-debug`
)

func (r *remote) setDefaults() {
	if r.GRPC.Address == "" {
		r.GRPC.Address = grpcPipeName
	}
	if r.Debug.Address == "" {
		r.Debug.Address = debugPipeName
	}
	if r.Debug.Level == "" {
		r.Debug.Level = "info"
	}
	if r.snapshotter == "" {
		r.snapshotter = "naive" // TODO(mlaventure): switch to "windows" once implemented
	}
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

func (r *remote) platformCleanup() {
	// Nothing to do
}
