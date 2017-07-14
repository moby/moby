package containerd

import (
	"io"

	"github.com/docker/docker/libcontainerd"
	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/pkg/errors"
)

// ExitHandler represents an object that is called when the exit event is received from containerd
type ExitHandler interface {
	HandleExitEvent(id string) error
}

// New creates a new containerd plugin executor
func New(remote libcontainerd.Remote, exitHandler ExitHandler) (*Executor, error) {
	e := &Executor{exitHandler: exitHandler}
	client, err := remote.Client(e)
	if err != nil {
		return nil, errors.Wrap(err, "error creating containerd exec client")
	}
	e.client = client
	return e, nil
}

// Executor is the containerd client implementation of a plugin executor
type Executor struct {
	client      libcontainerd.Client
	exitHandler ExitHandler
}

// Create creates a new container
func (e *Executor) Create(id string, spec specs.Spec, stdout, stderr io.WriteCloser) error {
	return e.client.Create(id, "", "", spec, attachStreamsFunc(stdout, stderr))
}

// Restore restores a container
func (e *Executor) Restore(id string, stdout, stderr io.WriteCloser) error {
	return e.client.Restore(id, attachStreamsFunc(stdout, stderr))
}

// IsRunning returns if the container with the given id is running
func (e *Executor) IsRunning(id string) (bool, error) {
	pids, err := e.client.GetPidsForContainer(id)
	return len(pids) > 0, err
}

// Signal sends the specified signal to the container
func (e *Executor) Signal(id string, signal int) error {
	return e.client.Signal(id, signal)
}

// StateChanged handles state changes from containerd
// All events are ignored except the exit event, which is sent of to the stored handler
func (e *Executor) StateChanged(id string, event libcontainerd.StateInfo) error {
	switch event.State {
	case libcontainerd.StateExit:
		return e.exitHandler.HandleExitEvent(id)
	}
	return nil
}

func attachStreamsFunc(stdout, stderr io.WriteCloser) func(libcontainerd.IOPipe) error {
	return func(iop libcontainerd.IOPipe) error {
		iop.Stdin.Close()
		go func() {
			io.Copy(stdout, iop.Stdout)
			stdout.Close()
		}()
		go func() {
			io.Copy(stderr, iop.Stderr)
			stderr.Close()
		}()
		return nil
	}
}
