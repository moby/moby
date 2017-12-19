package containerd

import (
	"context"
	"io"
	"path/filepath"
	"sync"

	"github.com/containerd/containerd/cio"
	"github.com/containerd/containerd/linux/runctypes"
	"github.com/docker/docker/api/errdefs"
	"github.com/docker/docker/libcontainerd"
	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

// PluginNamespace is the name used for the plugins namespace
var PluginNamespace = "plugins.moby"

// ExitHandler represents an object that is called when the exit event is received from containerd
type ExitHandler interface {
	HandleExitEvent(id string) error
}

// New creates a new containerd plugin executor
func New(rootDir string, remote libcontainerd.Remote, exitHandler ExitHandler) (*Executor, error) {
	e := &Executor{
		rootDir:     rootDir,
		exitHandler: exitHandler,
	}
	client, err := remote.NewClient(PluginNamespace, e)
	if err != nil {
		return nil, errors.Wrap(err, "error creating containerd exec client")
	}
	e.client = client
	return e, nil
}

// Executor is the containerd client implementation of a plugin executor
type Executor struct {
	rootDir     string
	client      libcontainerd.Client
	exitHandler ExitHandler
}

// Create creates a new container
func (e *Executor) Create(id string, spec specs.Spec, stdout, stderr io.WriteCloser) error {
	opts := runctypes.RuncOptions{
		RuntimeRoot: filepath.Join(e.rootDir, "runtime-root"),
	}
	ctx := context.Background()
	err := e.client.Create(ctx, id, &spec, &opts)
	if err != nil {
		return err
	}

	_, err = e.client.Start(ctx, id, "", false, attachStreamsFunc(stdout, stderr))
	return err
}

// Restore restores a container
func (e *Executor) Restore(id string, stdout, stderr io.WriteCloser) error {
	alive, _, err := e.client.Restore(context.Background(), id, attachStreamsFunc(stdout, stderr))
	if err != nil && !errdefs.IsNotFound(err) {
		return err
	}
	if !alive {
		_, _, err = e.client.DeleteTask(context.Background(), id)
		if err != nil && !errdefs.IsNotFound(err) {
			logrus.WithError(err).Errorf("failed to delete container plugin %s task from containerd", id)
			return err
		}

		err = e.client.Delete(context.Background(), id)
		if err != nil && !errdefs.IsNotFound(err) {
			logrus.WithError(err).Errorf("failed to delete container plugin %s from containerd", id)
			return err
		}
	}
	return nil
}

// IsRunning returns if the container with the given id is running
func (e *Executor) IsRunning(id string) (bool, error) {
	status, err := e.client.Status(context.Background(), id)
	return status == libcontainerd.StatusRunning, err
}

// Signal sends the specified signal to the container
func (e *Executor) Signal(id string, signal int) error {
	return e.client.SignalProcess(context.Background(), id, libcontainerd.InitProcessName, signal)
}

// ProcessEvent handles events from containerd
// All events are ignored except the exit event, which is sent of to the stored handler
func (e *Executor) ProcessEvent(id string, et libcontainerd.EventType, ei libcontainerd.EventInfo) error {
	switch et {
	case libcontainerd.EventExit:
		// delete task and container
		if _, _, err := e.client.DeleteTask(context.Background(), id); err != nil {
			logrus.WithError(err).Errorf("failed to delete container plugin %s task from containerd", id)
		}

		if err := e.client.Delete(context.Background(), id); err != nil {
			logrus.WithError(err).Errorf("failed to delete container plugin %s from containerd", id)
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

func attachStreamsFunc(stdout, stderr io.WriteCloser) libcontainerd.StdioCallback {
	return func(iop *libcontainerd.IOPipe) (cio.IO, error) {
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
