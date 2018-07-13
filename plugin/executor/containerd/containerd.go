package containerd // import "github.com/docker/docker/plugin/executor/containerd"

import (
	"context"
	"io"
	"path/filepath"
	"sync"
	"time"

	"github.com/containerd/containerd/cio"
	"github.com/containerd/containerd/runtime/linux/runctypes"
	"github.com/docker/docker/errdefs"
	"github.com/docker/docker/libcontainerd"
	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

// pluginNamespace is the name used for the plugins namespace
const pluginNamespace = "plugins.moby"

// ExitHandler represents an object that is called when the exit event is received from containerd
type ExitHandler interface {
	HandleExitEvent(id string) error
}

// Client is used by the exector to perform operations.
// TODO(@cpuguy83): This should really just be based off the containerd client interface.
// However right now this whole package is tied to github.com/docker/docker/libcontainerd
type Client interface {
	Create(ctx context.Context, containerID string, spec *specs.Spec, runtimeOptions interface{}) error
	Restore(ctx context.Context, containerID string, attachStdio libcontainerd.StdioCallback) (alive bool, pid int, err error)
	Status(ctx context.Context, containerID string) (libcontainerd.Status, error)
	Delete(ctx context.Context, containerID string) error
	DeleteTask(ctx context.Context, containerID string) (uint32, time.Time, error)
	Start(ctx context.Context, containerID, checkpointDir string, withStdin bool, attachStdio libcontainerd.StdioCallback) (pid int, err error)
	SignalProcess(ctx context.Context, containerID, processID string, signal int) error
}

// New creates a new containerd plugin executor
func New(rootDir string, remote libcontainerd.Remote, exitHandler ExitHandler) (*Executor, error) {
	e := &Executor{
		rootDir:     rootDir,
		exitHandler: exitHandler,
	}
	client, err := remote.NewClient(pluginNamespace, e)
	if err != nil {
		return nil, errors.Wrap(err, "error creating containerd exec client")
	}
	e.client = client
	return e, nil
}

// Executor is the containerd client implementation of a plugin executor
type Executor struct {
	rootDir     string
	client      Client
	exitHandler ExitHandler
}

// deleteTaskAndContainer deletes plugin task and then plugin container from containerd
func deleteTaskAndContainer(ctx context.Context, cli Client, id string) {
	_, _, err := cli.DeleteTask(ctx, id)
	if err != nil && !errdefs.IsNotFound(err) {
		logrus.WithError(err).WithField("id", id).Error("failed to delete plugin task from containerd")
	}

	err = cli.Delete(ctx, id)
	if err != nil && !errdefs.IsNotFound(err) {
		logrus.WithError(err).WithField("id", id).Error("failed to delete plugin container from containerd")
	}
}

// Create creates a new container
func (e *Executor) Create(id string, spec specs.Spec, stdout, stderr io.WriteCloser) error {
	opts := runctypes.RuncOptions{
		RuntimeRoot: filepath.Join(e.rootDir, "runtime-root"),
	}
	ctx := context.Background()
	err := e.client.Create(ctx, id, &spec, &opts)
	if err != nil {
		status, err2 := e.client.Status(ctx, id)
		if err2 != nil {
			if !errdefs.IsNotFound(err2) {
				logrus.WithError(err2).WithField("id", id).Warn("Received an error while attempting to read plugin status")
			}
		} else {
			if status != libcontainerd.StatusRunning && status != libcontainerd.StatusUnknown {
				if err2 := e.client.Delete(ctx, id); err2 != nil && !errdefs.IsNotFound(err2) {
					logrus.WithError(err2).WithField("plugin", id).Error("Error cleaning up containerd container")
				}
				err = e.client.Create(ctx, id, &spec, &opts)
			}
		}

		if err != nil {
			return errors.Wrap(err, "error creating containerd container")
		}
	}

	_, err = e.client.Start(ctx, id, "", false, attachStreamsFunc(stdout, stderr))
	if err != nil {
		deleteTaskAndContainer(ctx, e.client, id)
	}
	return err
}

// Restore restores a container
func (e *Executor) Restore(id string, stdout, stderr io.WriteCloser) (bool, error) {
	alive, _, err := e.client.Restore(context.Background(), id, attachStreamsFunc(stdout, stderr))
	if err != nil && !errdefs.IsNotFound(err) {
		return false, err
	}
	if !alive {
		deleteTaskAndContainer(context.Background(), e.client, id)
	}
	return alive, nil
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
		deleteTaskAndContainer(context.Background(), e.client, id)
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
