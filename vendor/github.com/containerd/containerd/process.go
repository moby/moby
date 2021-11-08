/*
   Copyright The containerd Authors.

   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
*/

package containerd

import (
	"context"
	"strings"
	"syscall"
	"time"

	"github.com/containerd/containerd/api/services/tasks/v1"
	"github.com/containerd/containerd/cio"
	"github.com/containerd/containerd/errdefs"
	"github.com/pkg/errors"
)

// Process represents a system process
type Process interface {
	// ID of the process
	ID() string
	// Pid is the system specific process id
	Pid() uint32
	// Start starts the process executing the user's defined binary
	Start(context.Context) error
	// Delete removes the process and any resources allocated returning the exit status
	Delete(context.Context, ...ProcessDeleteOpts) (*ExitStatus, error)
	// Kill sends the provided signal to the process
	Kill(context.Context, syscall.Signal, ...KillOpts) error
	// Wait asynchronously waits for the process to exit, and sends the exit code to the returned channel
	Wait(context.Context) (<-chan ExitStatus, error)
	// CloseIO allows various pipes to be closed on the process
	CloseIO(context.Context, ...IOCloserOpts) error
	// Resize changes the width and height of the process's terminal
	Resize(ctx context.Context, w, h uint32) error
	// IO returns the io set for the process
	IO() cio.IO
	// Status returns the executing status of the process
	Status(context.Context) (Status, error)
}

// NewExitStatus populates an ExitStatus
func NewExitStatus(code uint32, t time.Time, err error) *ExitStatus {
	return &ExitStatus{
		code:     code,
		exitedAt: t,
		err:      err,
	}
}

// ExitStatus encapsulates a process's exit status.
// It is used by `Wait()` to return either a process exit code or an error
type ExitStatus struct {
	code     uint32
	exitedAt time.Time
	err      error
}

// Result returns the exit code and time of the exit status.
// An error may be returned here to which indicates there was an error
//   at some point while waiting for the exit status. It does not signify
//   an error with the process itself.
// If an error is returned, the process may still be running.
func (s ExitStatus) Result() (uint32, time.Time, error) {
	return s.code, s.exitedAt, s.err
}

// ExitCode returns the exit code of the process.
// This is only valid is Error() returns nil
func (s ExitStatus) ExitCode() uint32 {
	return s.code
}

// ExitTime returns the exit time of the process
// This is only valid is Error() returns nil
func (s ExitStatus) ExitTime() time.Time {
	return s.exitedAt
}

// Error returns the error, if any, that occurred while waiting for the
// process.
func (s ExitStatus) Error() error {
	return s.err
}

type process struct {
	id   string
	task *task
	pid  uint32
	io   cio.IO
}

func (p *process) ID() string {
	return p.id
}

// Pid returns the pid of the process
// The pid is not set until start is called and returns
func (p *process) Pid() uint32 {
	return p.pid
}

// Start starts the exec process
func (p *process) Start(ctx context.Context) error {
	r, err := p.task.client.TaskService().Start(ctx, &tasks.StartRequest{
		ContainerID: p.task.id,
		ExecID:      p.id,
	})
	if err != nil {
		if p.io != nil {
			p.io.Cancel()
			p.io.Wait()
			p.io.Close()
		}
		return errdefs.FromGRPC(err)
	}
	p.pid = r.Pid
	return nil
}

func (p *process) Kill(ctx context.Context, s syscall.Signal, opts ...KillOpts) error {
	var i KillInfo
	for _, o := range opts {
		if err := o(ctx, &i); err != nil {
			return err
		}
	}
	_, err := p.task.client.TaskService().Kill(ctx, &tasks.KillRequest{
		Signal:      uint32(s),
		ContainerID: p.task.id,
		ExecID:      p.id,
		All:         i.All,
	})
	return errdefs.FromGRPC(err)
}

func (p *process) Wait(ctx context.Context) (<-chan ExitStatus, error) {
	c := make(chan ExitStatus, 1)
	go func() {
		defer close(c)
		r, err := p.task.client.TaskService().Wait(ctx, &tasks.WaitRequest{
			ContainerID: p.task.id,
			ExecID:      p.id,
		})
		if err != nil {
			c <- ExitStatus{
				code: UnknownExitStatus,
				err:  err,
			}
			return
		}
		c <- ExitStatus{
			code:     r.ExitStatus,
			exitedAt: r.ExitedAt,
		}
	}()
	return c, nil
}

func (p *process) CloseIO(ctx context.Context, opts ...IOCloserOpts) error {
	r := &tasks.CloseIORequest{
		ContainerID: p.task.id,
		ExecID:      p.id,
	}
	var i IOCloseInfo
	for _, o := range opts {
		o(&i)
	}
	r.Stdin = i.Stdin
	_, err := p.task.client.TaskService().CloseIO(ctx, r)
	return errdefs.FromGRPC(err)
}

func (p *process) IO() cio.IO {
	return p.io
}

func (p *process) Resize(ctx context.Context, w, h uint32) error {
	_, err := p.task.client.TaskService().ResizePty(ctx, &tasks.ResizePtyRequest{
		ContainerID: p.task.id,
		Width:       w,
		Height:      h,
		ExecID:      p.id,
	})
	return errdefs.FromGRPC(err)
}

func (p *process) Delete(ctx context.Context, opts ...ProcessDeleteOpts) (*ExitStatus, error) {
	for _, o := range opts {
		if err := o(ctx, p); err != nil {
			return nil, err
		}
	}
	status, err := p.Status(ctx)
	if err != nil {
		return nil, err
	}
	switch status.Status {
	case Running, Paused, Pausing:
		return nil, errors.Wrapf(errdefs.ErrFailedPrecondition, "current process state: %s, process must be stopped before deletion", status.Status)
	}
	r, err := p.task.client.TaskService().DeleteProcess(ctx, &tasks.DeleteProcessRequest{
		ContainerID: p.task.id,
		ExecID:      p.id,
	})
	if err != nil {
		return nil, errdefs.FromGRPC(err)
	}
	if p.io != nil {
		p.io.Cancel()
		p.io.Wait()
		p.io.Close()
	}
	return &ExitStatus{code: r.ExitStatus, exitedAt: r.ExitedAt}, nil
}

func (p *process) Status(ctx context.Context) (Status, error) {
	r, err := p.task.client.TaskService().Get(ctx, &tasks.GetRequest{
		ContainerID: p.task.id,
		ExecID:      p.id,
	})
	if err != nil {
		return Status{}, errdefs.FromGRPC(err)
	}
	return Status{
		Status:     ProcessStatus(strings.ToLower(r.Process.Status.String())),
		ExitStatus: r.Process.ExitStatus,
	}, nil
}
