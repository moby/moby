// DEPRECATION NOTICE. PLEASE DO NOT ADD ANYTHING TO THIS FILE.
//
// server/server.go is deprecated. We are working on breaking it up into smaller, cleaner
// pieces which will be easier to find and test. This will help make the code less
// redundant and more readable.
//
// Contributors, please don't add anything to server/server.go, unless it has the explicit
// goal of helping the deprecation effort.
//
// Maintainers, please refuse patches which add code to server/server.go.
//
// Instead try the following files:
// * For code related to local image management, try graph/
// * For code related to image downloading, uploading, remote search etc, try registry/
// * For code related to the docker daemon, try daemon/
// * For small utilities which could potentially be useful outside of Docker, try pkg/
// * For miscalleneous "util" functions which are docker-specific, try encapsulating them
//     inside one of the subsystems above. If you really think they should be more widely
//     available, are you sure you can't remove the docker dependencies and move them to
//     pkg? In last resort, you can add them to utils/ (but please try not to).

package server

import (
	"os"
	"runtime"
	"sync"
	"time"

	"github.com/docker/docker/daemon"
	"github.com/docker/docker/dockerversion"
	"github.com/docker/docker/engine"
	"github.com/docker/docker/pkg/parsers/kernel"
	"github.com/docker/docker/pkg/parsers/operatingsystem"
	"github.com/docker/docker/registry"
	"github.com/docker/docker/utils"
)

func (srv *Server) DockerInfo(job *engine.Job) engine.Status {
	images, _ := srv.daemon.Graph().Map()
	var imgcount int
	if images == nil {
		imgcount = 0
	} else {
		imgcount = len(images)
	}
	kernelVersion := "<unknown>"
	if kv, err := kernel.GetKernelVersion(); err == nil {
		kernelVersion = kv.String()
	}

	operatingSystem := "<unknown>"
	if s, err := operatingsystem.GetOperatingSystem(); err == nil {
		operatingSystem = s
	}
	if inContainer, err := operatingsystem.IsContainerized(); err != nil {
		utils.Errorf("Could not determine if daemon is containerized: %v", err)
		operatingSystem += " (error determining if containerized)"
	} else if inContainer {
		operatingSystem += " (containerized)"
	}

	// if we still have the original dockerinit binary from before we copied it locally, let's return the path to that, since that's more intuitive (the copied path is trivial to derive by hand given VERSION)
	initPath := utils.DockerInitPath("")
	if initPath == "" {
		// if that fails, we'll just return the path from the daemon
		initPath = srv.daemon.SystemInitPath()
	}

	v := &engine.Env{}
	v.SetInt("Containers", len(srv.daemon.List()))
	v.SetInt("Images", imgcount)
	v.Set("Driver", srv.daemon.GraphDriver().String())
	v.SetJson("DriverStatus", srv.daemon.GraphDriver().Status())
	v.SetBool("MemoryLimit", srv.daemon.SystemConfig().MemoryLimit)
	v.SetBool("SwapLimit", srv.daemon.SystemConfig().SwapLimit)
	v.SetBool("IPv4Forwarding", !srv.daemon.SystemConfig().IPv4ForwardingDisabled)
	v.SetBool("Debug", os.Getenv("DEBUG") != "")
	v.SetInt("NFd", utils.GetTotalUsedFds())
	v.SetInt("NGoroutines", runtime.NumGoroutine())
	v.Set("ExecutionDriver", srv.daemon.ExecutionDriver().Name())
	v.SetInt("NEventsListener", srv.eventPublisher.SubscribersCount())
	v.Set("KernelVersion", kernelVersion)
	v.Set("OperatingSystem", operatingSystem)
	v.Set("IndexServerAddress", registry.IndexServerAddress())
	v.Set("InitSha1", dockerversion.INITSHA1)
	v.Set("InitPath", initPath)
	v.SetList("Sockets", srv.daemon.Sockets)
	if _, err := v.WriteTo(job.Stdout); err != nil {
		return job.Error(err)
	}
	return engine.StatusOK
}

func (srv *Server) SetRunning(status bool) {
	srv.Lock()
	defer srv.Unlock()

	srv.running = status
}

func (srv *Server) IsRunning() bool {
	srv.RLock()
	defer srv.RUnlock()
	return srv.running
}

func (srv *Server) Close() error {
	if srv == nil {
		return nil
	}
	srv.SetRunning(false)
	done := make(chan struct{})
	go func() {
		srv.tasks.Wait()
		close(done)
	}()
	select {
	// Waiting server jobs for 15 seconds, shutdown immediately after that time
	case <-time.After(time.Second * 15):
	case <-done:
	}
	if srv.daemon == nil {
		return nil
	}
	return srv.daemon.Close()
}

type Server struct {
	sync.RWMutex
	daemon         *daemon.Daemon
	pullingPool    map[string]chan struct{}
	pushingPool    map[string]chan struct{}
	events         []utils.JSONMessage
	eventPublisher *utils.JSONMessagePublisher
	Eng            *engine.Engine
	running        bool
	tasks          sync.WaitGroup
}
