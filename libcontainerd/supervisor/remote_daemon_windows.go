package supervisor // import "github.com/docker/docker/libcontainerd/supervisor"

import (
	"os"

	"github.com/docker/docker/pkg/process"
)

const (
	grpcPipeName  = `\\.\pipe\containerd-containerd`
	debugPipeName = `\\.\pipe\containerd-debug`
)

func (r *remote) setDefaults() {
	if r.GRPC.Address == "" {
		r.GRPC.Address = grpcPipeName
	}
	if r.Debug.Address == "" {
		r.Debug.Address = debugPipeName
	}
}

func (r *remote) stopDaemon(pid int) {
	p, err := os.FindProcess(pid)
	if err != nil {
		r.logger.WithField("pid", pid).Warn("could not find daemon process")
		return
	}

	if err = p.Kill(); err != nil {
		r.logger.WithError(err).WithField("pid", pid).Warn("could not kill daemon process")
		return
	}

	_, err = p.Wait()
	if err != nil {
		r.logger.WithError(err).WithField("pid", pid).Warn("wait for daemon process")
		return
	}
}

func (r *remote) killDaemon(pid int) {
	_ = process.Kill(pid)
}

func (r *remote) platformCleanup() {
	// Nothing to do
}
