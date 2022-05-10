package containerd // import "github.com/docker/docker/plugin/executor/containerd"

import (
	"context"
	"fmt"
	"io"
	"sync"
	"syscall"

	"github.com/containerd/containerd"
	"github.com/containerd/containerd/cio"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/errdefs"
	"github.com/docker/docker/libcontainerd"
	libcontainerdtypes "github.com/docker/docker/libcontainerd/types"
	specs "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

// ExitHandler represents an object that is called when the exit event is received from containerd
type ExitHandler interface {
	HandleExitEvent(id string) error
}

// New creates a new containerd plugin executor
func New(ctx context.Context, rootDir string, cli *containerd.Client, ns string, exitHandler ExitHandler, runtime types.Runtime) (*Executor, error) {
	e := &Executor{
		rootDir:     rootDir,
		exitHandler: exitHandler,
		runtime:     runtime,
		plugins:     make(map[string]*c8dPlugin),
	}

	client, err := libcontainerd.NewClient(ctx, cli, rootDir, ns, e)
	if err != nil {
		return nil, errors.Wrap(err, "error creating containerd exec client")
	}
	e.client = client
	return e, nil
}

// Executor is the containerd client implementation of a plugin executor
type Executor struct {
	rootDir     string
	client      libcontainerdtypes.Client
	exitHandler ExitHandler
	runtime     types.Runtime

	mu      sync.Mutex // Guards plugins map
	plugins map[string]*c8dPlugin
}

type c8dPlugin struct {
	log *logrus.Entry
	ctr libcontainerdtypes.Container
	tsk libcontainerdtypes.Task
}

// deleteTaskAndContainer deletes plugin task and then plugin container from containerd
func (p c8dPlugin) deleteTaskAndContainer(ctx context.Context) {
	if p.tsk != nil {
		if _, err := p.tsk.Delete(ctx); err != nil && !errdefs.IsNotFound(err) {
			p.log.WithError(err).Error("failed to delete plugin task from containerd")
		}
	}
	if p.ctr != nil {
		if err := p.ctr.Delete(ctx); err != nil && !errdefs.IsNotFound(err) {
			p.log.WithError(err).Error("failed to delete plugin container from containerd")
		}
	}
}

// Create creates a new container
func (e *Executor) Create(id string, spec specs.Spec, stdout, stderr io.WriteCloser) error {
	ctx := context.Background()
	log := logrus.WithField("plugin", id)
	ctr, err := e.client.NewContainer(ctx, id, &spec, e.runtime.Shim.Binary, e.runtime.Shim.Opts)
	if err != nil {
		ctr2, err2 := e.client.LoadContainer(ctx, id)
		if err2 != nil {
			if !errdefs.IsNotFound(err2) {
				log.WithError(err2).Warn("Received an error while attempting to load containerd container for plugin")
			}
		} else {
			status := containerd.Unknown
			t, err2 := ctr2.Task(ctx)
			if err2 != nil {
				if !errdefs.IsNotFound(err2) {
					log.WithError(err2).Warn("Received an error while attempting to load containerd task for plugin")
				}
			} else {
				s, err2 := t.Status(ctx)
				if err2 != nil {
					log.WithError(err2).Warn("Received an error while attempting to read plugin status")
				} else {
					status = s.Status
				}
			}
			if status != containerd.Running && status != containerd.Unknown {
				if err2 := ctr2.Delete(ctx); err2 != nil && !errdefs.IsNotFound(err2) {
					log.WithError(err2).Error("Error cleaning up containerd container")
				}
				ctr, err = e.client.NewContainer(ctx, id, &spec, e.runtime.Shim.Binary, e.runtime.Shim.Opts)
			}
		}

		if err != nil {
			return errors.Wrap(err, "error creating containerd container")
		}
	}

	p := c8dPlugin{log: log, ctr: ctr}
	p.tsk, err = ctr.Start(ctx, "", false, attachStreamsFunc(stdout, stderr))
	if err != nil {
		p.deleteTaskAndContainer(ctx)
		return err
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	e.plugins[id] = &p
	return nil
}

// Restore restores a container
func (e *Executor) Restore(id string, stdout, stderr io.WriteCloser) (bool, error) {
	ctx := context.Background()
	p := c8dPlugin{log: logrus.WithField("plugin", id)}
	ctr, err := e.client.LoadContainer(ctx, id)
	if err != nil {
		if errdefs.IsNotFound(err) {
			return false, nil
		}
		return false, err
	}
	p.tsk, err = ctr.AttachTask(ctx, attachStreamsFunc(stdout, stderr))
	if err != nil {
		if errdefs.IsNotFound(err) {
			p.deleteTaskAndContainer(ctx)
			return false, nil
		}
		return false, err
	}
	s, err := p.tsk.Status(ctx)
	if err != nil {
		if errdefs.IsNotFound(err) {
			// Task vanished after attaching?
			p.tsk = nil
			p.deleteTaskAndContainer(ctx)
			return false, nil
		}
		return false, err
	}
	if s.Status == containerd.Stopped {
		p.deleteTaskAndContainer(ctx)
		return false, nil
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	e.plugins[id] = &p
	return true, nil
}

// IsRunning returns if the container with the given id is running
func (e *Executor) IsRunning(id string) (bool, error) {
	e.mu.Lock()
	p := e.plugins[id]
	e.mu.Unlock()
	if p == nil {
		return false, errdefs.NotFound(fmt.Errorf("unknown plugin %q", id))
	}
	status, err := p.tsk.Status(context.Background())
	return status.Status == containerd.Running, err
}

// Signal sends the specified signal to the container
func (e *Executor) Signal(id string, signal syscall.Signal) error {
	e.mu.Lock()
	p := e.plugins[id]
	e.mu.Unlock()
	if p == nil {
		return errdefs.NotFound(fmt.Errorf("unknown plugin %q", id))
	}
	return p.tsk.Kill(context.Background(), signal)
}

// ProcessEvent handles events from containerd
// All events are ignored except the exit event, which is sent of to the stored handler
func (e *Executor) ProcessEvent(id string, et libcontainerdtypes.EventType, ei libcontainerdtypes.EventInfo) error {
	switch et {
	case libcontainerdtypes.EventExit:
		e.mu.Lock()
		p := e.plugins[id]
		e.mu.Unlock()
		if p == nil {
			logrus.WithField("id", id).Warn("Received exit event for an unknown plugin")
		} else {
			p.deleteTaskAndContainer(context.Background())
		}
		return e.exitHandler.HandleExitEvent(ei.ContainerID)
	}
	return nil
}

type rio struct {
	cio.IO

	wg sync.WaitGroup
}

func (c *rio) Wait() {
	c.wg.Wait()
	c.IO.Wait()
}

func attachStreamsFunc(stdout, stderr io.WriteCloser) libcontainerdtypes.StdioCallback {
	return func(iop *cio.DirectIO) (cio.IO, error) {
		if iop.Stdin != nil {
			iop.Stdin.Close()
			// closing stdin shouldn't be needed here, it should never be open
			panic("plugin stdin shouldn't have been created!")
		}

		rio := &rio{IO: iop}
		rio.wg.Add(2)
		go func() {
			io.Copy(stdout, iop.Stdout)
			stdout.Close()
			rio.wg.Done()
		}()
		go func() {
			io.Copy(stderr, iop.Stderr)
			stderr.Close()
			rio.wg.Done()
		}()
		return rio, nil
	}
}
