package supervisor // import "github.com/docker/docker/libcontainerd/supervisor"

import (
	"os"
	"path/filepath"
	"syscall"
	"time"

	"github.com/containerd/containerd/defaults"
	"github.com/docker/docker/pkg/process"
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
}

func (r *remote) stopDaemon(pid int) {
	// Ask the daemon to quit
	syscall.Kill(pid, syscall.SIGTERM)
	// Wait up to 15secs for it to stop
	for i := time.Duration(0); i < shutdownTimeout; i += time.Second {
		if !process.Alive(pid) {
			break
		}
		time.Sleep(time.Second)
	}

	if process.Alive(pid) {
		r.logger.WithField("pid", pid).Warn("daemon didn't stop within 15 secs, killing it")
		syscall.Kill(pid, syscall.SIGKILL)
	}
}

func (r *remote) killDaemon(pid int) {
	// Try to get a stack trace
	_ = syscall.Kill(pid, syscall.SIGUSR1)
	<-time.After(100 * time.Millisecond)
	_ = process.Kill(pid)
}

func (r *remote) platformCleanup() {
	_ = os.Remove(r.Address())
}
