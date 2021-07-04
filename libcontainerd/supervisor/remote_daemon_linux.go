package supervisor // import "github.com/docker/docker/libcontainerd/supervisor"

import (
	"os"
	"path/filepath"
	"syscall"
	"time"

	"github.com/containerd/containerd/defaults"
	"github.com/docker/docker/pkg/system"
)

const (
	sockFile      = "containerd.sock"
	debugSockFile = "containerd-debug.sock"
)

func (r *remote) setDefaults() {
	if r.GRPC.Address == "" {
		r.GRPC.Address = filepath.Join(r.stateDir, sockFile)
	}
	if r.GRPC.MaxRecvMsgSize == 0 {
		r.GRPC.MaxRecvMsgSize = defaults.DefaultMaxRecvMsgSize
	}
	if r.GRPC.MaxSendMsgSize == 0 {
		r.GRPC.MaxSendMsgSize = defaults.DefaultMaxSendMsgSize
	}
	if r.Debug.Address == "" {
		r.Debug.Address = filepath.Join(r.stateDir, debugSockFile)
	}

	for key, conf := range r.Plugins {
		if conf == nil {
			r.DisabledPlugins = append(r.DisabledPlugins, key)
			delete(r.Plugins, key)
		}
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

func (r *remote) killDaemon() {
	// Try to get a stack trace
	syscall.Kill(r.daemonPid, syscall.SIGUSR1)
	<-time.After(100 * time.Millisecond)
	system.KillProcess(r.daemonPid)
}

func (r *remote) platformCleanup() {
	os.Remove(filepath.Join(r.stateDir, sockFile))
}
