package libcontainerd

import (
	"os"
	"path/filepath"
	"syscall"
	"time"

	"github.com/docker/docker/pkg/system"
)

const (
	sockFile      = "docker-containerd.sock"
	debugSockFile = "docker-containerd-debug.sock"
)

func (r *remote) setDefaults() {
	if r.GRPC.Address == "" {
		r.GRPC.Address = filepath.Join(r.stateDir, sockFile)
	}
	if r.Debug.Address == "" {
		r.Debug.Address = filepath.Join(r.stateDir, debugSockFile)
	}
	if r.Debug.Level == "" {
		r.Debug.Level = "info"
	}
	if r.OOMScore == 0 {
		r.OOMScore = -999
	}
	if r.snapshotter == "" {
		r.snapshotter = "overlay"
	}
}

func (r *remote) stopDaemon() {
	// Ask the daemon to quit
	syscall.Kill(r.daemonPid, syscall.SIGTERM)
	// Wait up to 15secs for it to stop
	for i := time.Duration(0); i < shutdownTimeout; i += time.Second {
		if !system.IsProcessAlive(r.daemonPid) {
			break
		}
		time.Sleep(time.Second)
	}

	if system.IsProcessAlive(r.daemonPid) {
		r.logger.WithField("pid", r.daemonPid).Warn("daemon didn't stop within 15 secs, killing it")
		syscall.Kill(r.daemonPid, syscall.SIGKILL)
	}
}

func (r *remote) platformCleanup() {
	os.Remove(filepath.Join(r.stateDir, sockFile))
}
