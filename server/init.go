// DEPRECATION NOTICE. PLEASE DO NOT ADD ANYTHING TO THIS FILE.
//
// For additional commments see server/server.go
//
package server

import (
	"fmt"

	"github.com/docker/docker/daemon"
	"github.com/docker/docker/daemonconfig"
	"github.com/docker/docker/engine"
	"github.com/docker/docker/pkg/signal"
	"github.com/docker/docker/utils"
)

func (srv *Server) handlerWrap(h engine.Handler) engine.Handler {
	return func(job *engine.Job) engine.Status {
		if !srv.IsRunning() {
			return job.Errorf("Server is not running")
		}
		srv.tasks.Add(1)
		defer srv.tasks.Done()
		return h(job)
	}
}

func InitPidfile(job *engine.Job) engine.Status {
	if len(job.Args) == 0 {
		return job.Error(fmt.Errorf("no pidfile provided to initialize"))
	}
	job.Logf("Creating pidfile")
	if err := utils.CreatePidFile(job.Args[0]); err != nil {
		return job.Error(err)
	}
	return engine.StatusOK
}

// jobInitApi runs the remote api server `srv` as a daemon,
// Only one api server can run at the same time - this is enforced by a pidfile.
// The signals SIGINT, SIGQUIT and SIGTERM are intercepted for cleanup.
func InitServer(job *engine.Job) engine.Status {
	job.Logf("Creating server")
	srv, err := NewServer(job.Eng, daemonconfig.ConfigFromJob(job))
	if err != nil {
		return job.Error(err)
	}
	job.Logf("Setting up signal traps")
	signal.Trap(func() {
		utils.RemovePidFile(srv.daemon.Config().Pidfile)
		srv.Close()
	})
	job.Eng.Hack_SetGlobalVar("httpapi.server", srv)
	job.Eng.Hack_SetGlobalVar("httpapi.daemon", srv.daemon)

	for name, handler := range map[string]engine.Handler{
		"tag":          srv.ImageTag, // FIXME merge with "image_tag"
		"info":         srv.DockerInfo,
		"image_export": srv.ImageExport,
		"images":       srv.Images,
		"history":      srv.ImageHistory,
		"viz":          srv.ImagesViz,
		"log":          srv.Log,
		"load":         srv.ImageLoad,
		"build":        srv.Build,
		"pull":         srv.ImagePull,
		"import":       srv.ImageImport,
		"image_delete": srv.ImageDelete,
		"events":       srv.Events,
		"push":         srv.ImagePush,
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
	srv.SetRunning(true)
	return engine.StatusOK
}

func NewServer(eng *engine.Engine, config *daemonconfig.Config) (*Server, error) {
	daemon, err := daemon.NewDaemon(config, eng)
	if err != nil {
		return nil, err
	}
	srv := &Server{
		Eng:            eng,
		daemon:         daemon,
		pullingPool:    make(map[string]chan struct{}),
		pushingPool:    make(map[string]chan struct{}),
		events:         make([]utils.JSONMessage, 0, 64), //only keeps the 64 last events
		eventPublisher: utils.NewJSONMessagePublisher(),
	}
	daemon.SetServer(srv)
	return srv, nil
}
