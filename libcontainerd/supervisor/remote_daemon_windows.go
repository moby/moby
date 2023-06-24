package supervisor // import "github.com/docker/docker/libcontainerd/supervisor"

import (
	"os"

	"github.com/docker/docker/pkg/process"
)

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
	process.Kill(r.daemonPid)
}

func (r *remote) platformCleanup() {
	// Nothing to do
}
