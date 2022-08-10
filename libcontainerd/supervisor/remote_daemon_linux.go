package supervisor // import "github.com/docker/docker/libcontainerd/supervisor"

import (
	"os"
	"syscall"
	"time"

	"github.com/docker/docker/pkg/process"
)

func (r *remote) stopDaemon() {
	// Ask the daemon to quit
	syscall.Kill(r.daemonPid, syscall.SIGTERM)
	// Wait up to 15secs for it to stop
	for i := time.Duration(0); i < shutdownTimeout; i += time.Second {
		if !process.Alive(r.daemonPid) {
			break
		}
		time.Sleep(time.Second)
	}

	if process.Alive(r.daemonPid) {
		r.logger.WithField("pid", r.daemonPid).Warn("daemon didn't stop within 15 secs, killing it")
		syscall.Kill(r.daemonPid, syscall.SIGKILL)
	}
}

func (r *remote) killDaemon() {
	// Try to get a stack trace
	syscall.Kill(r.daemonPid, syscall.SIGUSR1)
	<-time.After(100 * time.Millisecond)
	process.Kill(r.daemonPid)
}

func (r *remote) platformCleanup() {
	_ = os.Remove(r.Address())
}
