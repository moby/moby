// DEPRECATION NOTICE. PLEASE DO NOT ADD ANYTHING TO THIS FILE.
//
// For additional commments see server/server.go
//
package server

import (
	"github.com/docker/docker/daemon"
	"github.com/docker/docker/daemonconfig"
	"github.com/docker/docker/engine"
)

func (srv *Server) handlerWrap(h engine.Handler) engine.Handler {
	return func(job *engine.Job) engine.Status {
		srv.tasks.Add(1)
		defer srv.tasks.Done()
		return h(job)
	}
}

// jobInitApi runs the remote api server `srv` as a daemon,
// Only one api server can run at the same time - this is enforced by a pidfile.
// The signals SIGINT, SIGQUIT and SIGTERM are intercepted for cleanup.
func InitServer(job *engine.Job) engine.Status {
	job.Logf("Creating server")
	cfg := daemonconfig.ConfigFromJob(job)
	srv, err := NewServer(job.Eng, cfg)
	if err != nil {
		return job.Error(err)
	}
	job.Eng.Hack_SetGlobalVar("httpapi.server", srv)
	job.Eng.Hack_SetGlobalVar("httpapi.daemon", srv.daemon)

	for name, handler := range map[string]engine.Handler{
		"build": srv.Build,
		"pull":  srv.ImagePull,
		"push":  srv.ImagePush,
	} {
		if err := job.Eng.Register(name, srv.handlerWrap(handler)); err != nil {
			return job.Error(err)
		}
	}
	// Install image-related commands from the image subsystem.
	// See `graph/service.go`
	if err := srv.daemon.Repositories().Install(job.Eng); err != nil {
		return job.Error(err)
	}
	// Install daemon-related commands from the daemon subsystem.
	// See `daemon/`
	if err := srv.daemon.Install(job.Eng); err != nil {
		return job.Error(err)
	}
	return engine.StatusOK
}

func NewServer(eng *engine.Engine, config *daemonconfig.Config) (*Server, error) {
	daemon, err := daemon.NewDaemon(config, eng)
	if err != nil {
		return nil, err
	}
	srv := &Server{
		Eng:         eng,
		daemon:      daemon,
		pullingPool: make(map[string]chan struct{}),
		pushingPool: make(map[string]chan struct{}),
	}
	return srv, nil
}
