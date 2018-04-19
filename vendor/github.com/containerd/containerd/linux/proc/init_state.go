// +build !windows

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

package proc

import (
	"context"
	"sync"
	"syscall"

	"github.com/containerd/console"
	"github.com/containerd/containerd/errdefs"
	"github.com/containerd/fifo"
	runc "github.com/containerd/go-runc"
	google_protobuf "github.com/gogo/protobuf/types"
	"github.com/pkg/errors"
)

type initState interface {
	State

	Pause(context.Context) error
	Resume(context.Context) error
	Update(context.Context, *google_protobuf.Any) error
	Checkpoint(context.Context, *CheckpointConfig) error
	Exec(context.Context, string, *ExecConfig) (Process, error)
}

type createdState struct {
	p *Init
}

func (s *createdState) transition(name string) error {
	switch name {
	case "running":
		s.p.initState = &runningState{p: s.p}
	case "stopped":
		s.p.initState = &stoppedState{p: s.p}
	case "deleted":
		s.p.initState = &deletedState{}
	default:
		return errors.Errorf("invalid state transition %q to %q", stateName(s), name)
	}
	return nil
}

func (s *createdState) Pause(ctx context.Context) error {
	s.p.mu.Lock()
	defer s.p.mu.Unlock()

	return errors.Errorf("cannot pause task in created state")
}

func (s *createdState) Resume(ctx context.Context) error {
	s.p.mu.Lock()
	defer s.p.mu.Unlock()

	return errors.Errorf("cannot resume task in created state")
}

func (s *createdState) Update(context context.Context, r *google_protobuf.Any) error {
	s.p.mu.Lock()
	defer s.p.mu.Unlock()

	return s.p.update(context, r)
}

func (s *createdState) Checkpoint(context context.Context, r *CheckpointConfig) error {
	s.p.mu.Lock()
	defer s.p.mu.Unlock()

	return errors.Errorf("cannot checkpoint a task in created state")
}

func (s *createdState) Resize(ws console.WinSize) error {
	s.p.mu.Lock()
	defer s.p.mu.Unlock()

	return s.p.resize(ws)
}

func (s *createdState) Start(ctx context.Context) error {
	s.p.mu.Lock()
	defer s.p.mu.Unlock()
	if err := s.p.start(ctx); err != nil {
		return err
	}
	return s.transition("running")
}

func (s *createdState) Delete(ctx context.Context) error {
	s.p.mu.Lock()
	defer s.p.mu.Unlock()
	if err := s.p.delete(ctx); err != nil {
		return err
	}
	return s.transition("deleted")
}

func (s *createdState) Kill(ctx context.Context, sig uint32, all bool) error {
	s.p.mu.Lock()
	defer s.p.mu.Unlock()

	return s.p.kill(ctx, sig, all)
}

func (s *createdState) SetExited(status int) {
	s.p.mu.Lock()
	defer s.p.mu.Unlock()

	s.p.setExited(status)

	if err := s.transition("stopped"); err != nil {
		panic(err)
	}
}

func (s *createdState) Exec(ctx context.Context, path string, r *ExecConfig) (Process, error) {
	s.p.mu.Lock()
	defer s.p.mu.Unlock()
	return s.p.exec(ctx, path, r)
}

type createdCheckpointState struct {
	p    *Init
	opts *runc.RestoreOpts
}

func (s *createdCheckpointState) transition(name string) error {
	switch name {
	case "running":
		s.p.initState = &runningState{p: s.p}
	case "stopped":
		s.p.initState = &stoppedState{p: s.p}
	case "deleted":
		s.p.initState = &deletedState{}
	default:
		return errors.Errorf("invalid state transition %q to %q", stateName(s), name)
	}
	return nil
}

func (s *createdCheckpointState) Pause(ctx context.Context) error {
	s.p.mu.Lock()
	defer s.p.mu.Unlock()

	return errors.Errorf("cannot pause task in created state")
}

func (s *createdCheckpointState) Resume(ctx context.Context) error {
	s.p.mu.Lock()
	defer s.p.mu.Unlock()

	return errors.Errorf("cannot resume task in created state")
}

func (s *createdCheckpointState) Update(context context.Context, r *google_protobuf.Any) error {
	s.p.mu.Lock()
	defer s.p.mu.Unlock()

	return s.p.update(context, r)
}

func (s *createdCheckpointState) Checkpoint(context context.Context, r *CheckpointConfig) error {
	s.p.mu.Lock()
	defer s.p.mu.Unlock()

	return errors.Errorf("cannot checkpoint a task in created state")
}

func (s *createdCheckpointState) Resize(ws console.WinSize) error {
	s.p.mu.Lock()
	defer s.p.mu.Unlock()

	return s.p.resize(ws)
}

func (s *createdCheckpointState) Start(ctx context.Context) error {
	s.p.mu.Lock()
	defer s.p.mu.Unlock()
	p := s.p
	if _, err := s.p.runtime.Restore(ctx, p.id, p.bundle, s.opts); err != nil {
		return p.runtimeError(err, "OCI runtime restore failed")
	}
	sio := p.stdio
	if sio.Stdin != "" {
		sc, err := fifo.OpenFifo(ctx, sio.Stdin, syscall.O_WRONLY|syscall.O_NONBLOCK, 0)
		if err != nil {
			return errors.Wrapf(err, "failed to open stdin fifo %s", sio.Stdin)
		}
		p.stdin = sc
		p.closers = append(p.closers, sc)
	}
	var copyWaitGroup sync.WaitGroup
	if !sio.IsNull() {
		if err := copyPipes(ctx, p.io, sio.Stdin, sio.Stdout, sio.Stderr, &p.wg, &copyWaitGroup); err != nil {
			return errors.Wrap(err, "failed to start io pipe copy")
		}
	}

	copyWaitGroup.Wait()
	pid, err := runc.ReadPidFile(s.opts.PidFile)
	if err != nil {
		return errors.Wrap(err, "failed to retrieve OCI runtime container pid")
	}
	p.pid = pid

	return s.transition("running")
}

func (s *createdCheckpointState) Delete(ctx context.Context) error {
	s.p.mu.Lock()
	defer s.p.mu.Unlock()
	if err := s.p.delete(ctx); err != nil {
		return err
	}
	return s.transition("deleted")
}

func (s *createdCheckpointState) Kill(ctx context.Context, sig uint32, all bool) error {
	s.p.mu.Lock()
	defer s.p.mu.Unlock()

	return s.p.kill(ctx, sig, all)
}

func (s *createdCheckpointState) SetExited(status int) {
	s.p.mu.Lock()
	defer s.p.mu.Unlock()

	s.p.setExited(status)

	if err := s.transition("stopped"); err != nil {
		panic(err)
	}
}

func (s *createdCheckpointState) Exec(ctx context.Context, path string, r *ExecConfig) (Process, error) {
	s.p.mu.Lock()
	defer s.p.mu.Unlock()

	return nil, errors.Errorf("cannot exec in a created state")
}

type runningState struct {
	p *Init
}

func (s *runningState) transition(name string) error {
	switch name {
	case "stopped":
		s.p.initState = &stoppedState{p: s.p}
	case "paused":
		s.p.initState = &pausedState{p: s.p}
	default:
		return errors.Errorf("invalid state transition %q to %q", stateName(s), name)
	}
	return nil
}

func (s *runningState) Pause(ctx context.Context) error {
	s.p.mu.Lock()
	defer s.p.mu.Unlock()
	if err := s.p.pause(ctx); err != nil {
		return err
	}
	return s.transition("paused")
}

func (s *runningState) Resume(ctx context.Context) error {
	s.p.mu.Lock()
	defer s.p.mu.Unlock()

	return errors.Errorf("cannot resume a running process")
}

func (s *runningState) Update(context context.Context, r *google_protobuf.Any) error {
	s.p.mu.Lock()
	defer s.p.mu.Unlock()

	return s.p.update(context, r)
}

func (s *runningState) Checkpoint(ctx context.Context, r *CheckpointConfig) error {
	s.p.mu.Lock()
	defer s.p.mu.Unlock()

	return s.p.checkpoint(ctx, r)
}

func (s *runningState) Resize(ws console.WinSize) error {
	s.p.mu.Lock()
	defer s.p.mu.Unlock()

	return s.p.resize(ws)
}

func (s *runningState) Start(ctx context.Context) error {
	s.p.mu.Lock()
	defer s.p.mu.Unlock()

	return errors.Errorf("cannot start a running process")
}

func (s *runningState) Delete(ctx context.Context) error {
	s.p.mu.Lock()
	defer s.p.mu.Unlock()

	return errors.Errorf("cannot delete a running process")
}

func (s *runningState) Kill(ctx context.Context, sig uint32, all bool) error {
	s.p.mu.Lock()
	defer s.p.mu.Unlock()

	return s.p.kill(ctx, sig, all)
}

func (s *runningState) SetExited(status int) {
	s.p.mu.Lock()
	defer s.p.mu.Unlock()

	s.p.setExited(status)

	if err := s.transition("stopped"); err != nil {
		panic(err)
	}
}

func (s *runningState) Exec(ctx context.Context, path string, r *ExecConfig) (Process, error) {
	s.p.mu.Lock()
	defer s.p.mu.Unlock()
	return s.p.exec(ctx, path, r)
}

type pausedState struct {
	p *Init
}

func (s *pausedState) transition(name string) error {
	switch name {
	case "running":
		s.p.initState = &runningState{p: s.p}
	case "stopped":
		s.p.initState = &stoppedState{p: s.p}
	default:
		return errors.Errorf("invalid state transition %q to %q", stateName(s), name)
	}
	return nil
}

func (s *pausedState) Pause(ctx context.Context) error {
	s.p.mu.Lock()
	defer s.p.mu.Unlock()

	return errors.Errorf("cannot pause a paused container")
}

func (s *pausedState) Resume(ctx context.Context) error {
	s.p.mu.Lock()
	defer s.p.mu.Unlock()

	if err := s.p.resume(ctx); err != nil {
		return err
	}
	return s.transition("running")
}

func (s *pausedState) Update(context context.Context, r *google_protobuf.Any) error {
	s.p.mu.Lock()
	defer s.p.mu.Unlock()

	return s.p.update(context, r)
}

func (s *pausedState) Checkpoint(ctx context.Context, r *CheckpointConfig) error {
	s.p.mu.Lock()
	defer s.p.mu.Unlock()

	return s.p.checkpoint(ctx, r)
}

func (s *pausedState) Resize(ws console.WinSize) error {
	s.p.mu.Lock()
	defer s.p.mu.Unlock()

	return s.p.resize(ws)
}

func (s *pausedState) Start(ctx context.Context) error {
	s.p.mu.Lock()
	defer s.p.mu.Unlock()

	return errors.Errorf("cannot start a paused process")
}

func (s *pausedState) Delete(ctx context.Context) error {
	s.p.mu.Lock()
	defer s.p.mu.Unlock()

	return errors.Errorf("cannot delete a paused process")
}

func (s *pausedState) Kill(ctx context.Context, sig uint32, all bool) error {
	s.p.mu.Lock()
	defer s.p.mu.Unlock()

	return s.p.kill(ctx, sig, all)
}

func (s *pausedState) SetExited(status int) {
	s.p.mu.Lock()
	defer s.p.mu.Unlock()

	s.p.setExited(status)

	if err := s.transition("stopped"); err != nil {
		panic(err)
	}
}

func (s *pausedState) Exec(ctx context.Context, path string, r *ExecConfig) (Process, error) {
	s.p.mu.Lock()
	defer s.p.mu.Unlock()

	return nil, errors.Errorf("cannot exec in a paused state")
}

type stoppedState struct {
	p *Init
}

func (s *stoppedState) transition(name string) error {
	switch name {
	case "deleted":
		s.p.initState = &deletedState{}
	default:
		return errors.Errorf("invalid state transition %q to %q", stateName(s), name)
	}
	return nil
}

func (s *stoppedState) Pause(ctx context.Context) error {
	s.p.mu.Lock()
	defer s.p.mu.Unlock()

	return errors.Errorf("cannot pause a stopped container")
}

func (s *stoppedState) Resume(ctx context.Context) error {
	s.p.mu.Lock()
	defer s.p.mu.Unlock()

	return errors.Errorf("cannot resume a stopped container")
}

func (s *stoppedState) Update(context context.Context, r *google_protobuf.Any) error {
	s.p.mu.Lock()
	defer s.p.mu.Unlock()

	return errors.Errorf("cannot update a stopped container")
}

func (s *stoppedState) Checkpoint(ctx context.Context, r *CheckpointConfig) error {
	s.p.mu.Lock()
	defer s.p.mu.Unlock()

	return errors.Errorf("cannot checkpoint a stopped container")
}

func (s *stoppedState) Resize(ws console.WinSize) error {
	s.p.mu.Lock()
	defer s.p.mu.Unlock()

	return errors.Errorf("cannot resize a stopped container")
}

func (s *stoppedState) Start(ctx context.Context) error {
	s.p.mu.Lock()
	defer s.p.mu.Unlock()

	return errors.Errorf("cannot start a stopped process")
}

func (s *stoppedState) Delete(ctx context.Context) error {
	s.p.mu.Lock()
	defer s.p.mu.Unlock()
	if err := s.p.delete(ctx); err != nil {
		return err
	}
	return s.transition("deleted")
}

func (s *stoppedState) Kill(ctx context.Context, sig uint32, all bool) error {
	return errdefs.ToGRPCf(errdefs.ErrNotFound, "process %s not found", s.p.id)
}

func (s *stoppedState) SetExited(status int) {
	// no op
}

func (s *stoppedState) Exec(ctx context.Context, path string, r *ExecConfig) (Process, error) {
	s.p.mu.Lock()
	defer s.p.mu.Unlock()

	return nil, errors.Errorf("cannot exec in a stopped state")
}
