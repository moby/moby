//go:build !windows
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

package shim

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/containerd/console"
	eventstypes "github.com/containerd/containerd/api/events"
	"github.com/containerd/containerd/api/types/task"
	"github.com/containerd/containerd/errdefs"
	"github.com/containerd/containerd/events"
	"github.com/containerd/containerd/log"
	"github.com/containerd/containerd/mount"
	"github.com/containerd/containerd/namespaces"
	"github.com/containerd/containerd/pkg/process"
	"github.com/containerd/containerd/pkg/stdio"
	"github.com/containerd/containerd/runtime"
	"github.com/containerd/containerd/runtime/linux/runctypes"
	shimapi "github.com/containerd/containerd/runtime/v1/shim/v1"
	"github.com/containerd/containerd/sys/reaper"
	runc "github.com/containerd/go-runc"
	"github.com/containerd/typeurl"
	ptypes "github.com/gogo/protobuf/types"
	specs "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var (
	empty   = &ptypes.Empty{}
	bufPool = sync.Pool{
		New: func() interface{} {
			buffer := make([]byte, 4096)
			return &buffer
		},
	}
)

// Config contains shim specific configuration
type Config struct {
	Path          string
	Namespace     string
	WorkDir       string
	Criu          string
	RuntimeRoot   string
	SystemdCgroup bool
}

// NewService returns a new shim service that can be used via GRPC
func NewService(config Config, publisher events.Publisher) (*Service, error) {
	if config.Namespace == "" {
		return nil, fmt.Errorf("shim namespace cannot be empty")
	}
	ctx := namespaces.WithNamespace(context.Background(), config.Namespace)
	ctx = log.WithLogger(ctx, logrus.WithFields(logrus.Fields{
		"namespace": config.Namespace,
		"path":      config.Path,
		"pid":       os.Getpid(),
	}))
	s := &Service{
		config:    config,
		context:   ctx,
		processes: make(map[string]process.Process),
		events:    make(chan interface{}, 128),
		ec:        reaper.Default.Subscribe(),
	}
	go s.processExits()
	if err := s.initPlatform(); err != nil {
		return nil, errors.Wrap(err, "failed to initialized platform behavior")
	}
	go s.forward(publisher)
	return s, nil
}

// Service is the shim implementation of a remote shim over GRPC
type Service struct {
	mu sync.Mutex

	config    Config
	context   context.Context
	processes map[string]process.Process
	events    chan interface{}
	platform  stdio.Platform
	ec        chan runc.Exit

	// Filled by Create()
	id     string
	bundle string
}

// Create a new initial process and container with the underlying OCI runtime
func (s *Service) Create(ctx context.Context, r *shimapi.CreateTaskRequest) (_ *shimapi.CreateTaskResponse, err error) {
	var mounts []process.Mount
	for _, m := range r.Rootfs {
		mounts = append(mounts, process.Mount{
			Type:    m.Type,
			Source:  m.Source,
			Target:  m.Target,
			Options: m.Options,
		})
	}

	rootfs := ""
	if len(mounts) > 0 {
		rootfs = filepath.Join(r.Bundle, "rootfs")
		if err := os.Mkdir(rootfs, 0711); err != nil && !os.IsExist(err) {
			return nil, err
		}
	}

	config := &process.CreateConfig{
		ID:               r.ID,
		Bundle:           r.Bundle,
		Runtime:          r.Runtime,
		Rootfs:           mounts,
		Terminal:         r.Terminal,
		Stdin:            r.Stdin,
		Stdout:           r.Stdout,
		Stderr:           r.Stderr,
		Checkpoint:       r.Checkpoint,
		ParentCheckpoint: r.ParentCheckpoint,
		Options:          r.Options,
	}
	defer func() {
		if err != nil {
			if err2 := mount.UnmountAll(rootfs, 0); err2 != nil {
				log.G(ctx).WithError(err2).Warn("Failed to cleanup rootfs mount")
			}
		}
	}()
	for _, rm := range mounts {
		m := &mount.Mount{
			Type:    rm.Type,
			Source:  rm.Source,
			Options: rm.Options,
		}
		if err := m.Mount(rootfs); err != nil {
			return nil, errors.Wrapf(err, "failed to mount rootfs component %v", m)
		}
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	process, err := newInit(
		ctx,
		s.config.Path,
		s.config.WorkDir,
		s.config.RuntimeRoot,
		s.config.Namespace,
		s.config.Criu,
		s.config.SystemdCgroup,
		s.platform,
		config,
		rootfs,
	)
	if err != nil {
		return nil, errdefs.ToGRPC(err)
	}
	if err := process.Create(ctx, config); err != nil {
		return nil, errdefs.ToGRPC(err)
	}
	// save the main task id and bundle to the shim for additional requests
	s.id = r.ID
	s.bundle = r.Bundle
	pid := process.Pid()
	s.processes[r.ID] = process
	return &shimapi.CreateTaskResponse{
		Pid: uint32(pid),
	}, nil
}

// Start a process
func (s *Service) Start(ctx context.Context, r *shimapi.StartRequest) (*shimapi.StartResponse, error) {
	p, err := s.getExecProcess(r.ID)
	if err != nil {
		return nil, err
	}
	if err := p.Start(ctx); err != nil {
		return nil, err
	}
	return &shimapi.StartResponse{
		ID:  p.ID(),
		Pid: uint32(p.Pid()),
	}, nil
}

// Delete the initial process and container
func (s *Service) Delete(ctx context.Context, r *ptypes.Empty) (*shimapi.DeleteResponse, error) {
	p, err := s.getInitProcess()
	if err != nil {
		return nil, err
	}
	if err := p.Delete(ctx); err != nil {
		return nil, errdefs.ToGRPC(err)
	}
	s.mu.Lock()
	delete(s.processes, s.id)
	s.mu.Unlock()
	s.platform.Close()
	return &shimapi.DeleteResponse{
		ExitStatus: uint32(p.ExitStatus()),
		ExitedAt:   p.ExitedAt(),
		Pid:        uint32(p.Pid()),
	}, nil
}

// DeleteProcess deletes an exec'd process
func (s *Service) DeleteProcess(ctx context.Context, r *shimapi.DeleteProcessRequest) (*shimapi.DeleteResponse, error) {
	if r.ID == s.id {
		return nil, status.Errorf(codes.InvalidArgument, "cannot delete init process with DeleteProcess")
	}
	p, err := s.getExecProcess(r.ID)
	if err != nil {
		return nil, err
	}
	if err := p.Delete(ctx); err != nil {
		return nil, errdefs.ToGRPC(err)
	}
	s.mu.Lock()
	delete(s.processes, r.ID)
	s.mu.Unlock()
	return &shimapi.DeleteResponse{
		ExitStatus: uint32(p.ExitStatus()),
		ExitedAt:   p.ExitedAt(),
		Pid:        uint32(p.Pid()),
	}, nil
}

// Exec an additional process inside the container
func (s *Service) Exec(ctx context.Context, r *shimapi.ExecProcessRequest) (*ptypes.Empty, error) {
	s.mu.Lock()

	if p := s.processes[r.ID]; p != nil {
		s.mu.Unlock()
		return nil, errdefs.ToGRPCf(errdefs.ErrAlreadyExists, "id %s", r.ID)
	}

	p := s.processes[s.id]
	s.mu.Unlock()
	if p == nil {
		return nil, errdefs.ToGRPCf(errdefs.ErrFailedPrecondition, "container must be created")
	}

	process, err := p.(*process.Init).Exec(ctx, s.config.Path, &process.ExecConfig{
		ID:       r.ID,
		Terminal: r.Terminal,
		Stdin:    r.Stdin,
		Stdout:   r.Stdout,
		Stderr:   r.Stderr,
		Spec:     r.Spec,
	})
	if err != nil {
		return nil, errdefs.ToGRPC(err)
	}
	s.mu.Lock()
	s.processes[r.ID] = process
	s.mu.Unlock()
	return empty, nil
}

// ResizePty of a process
func (s *Service) ResizePty(ctx context.Context, r *shimapi.ResizePtyRequest) (*ptypes.Empty, error) {
	if r.ID == "" {
		return nil, errdefs.ToGRPCf(errdefs.ErrInvalidArgument, "id not provided")
	}
	ws := console.WinSize{
		Width:  uint16(r.Width),
		Height: uint16(r.Height),
	}
	s.mu.Lock()
	p := s.processes[r.ID]
	s.mu.Unlock()
	if p == nil {
		return nil, errors.Errorf("process does not exist %s", r.ID)
	}
	if err := p.Resize(ws); err != nil {
		return nil, errdefs.ToGRPC(err)
	}
	return empty, nil
}

// State returns runtime state information for a process
func (s *Service) State(ctx context.Context, r *shimapi.StateRequest) (*shimapi.StateResponse, error) {
	p, err := s.getExecProcess(r.ID)
	if err != nil {
		return nil, err
	}
	st, err := p.Status(ctx)
	if err != nil {
		return nil, err
	}
	status := task.StatusUnknown
	switch st {
	case "created":
		status = task.StatusCreated
	case "running":
		status = task.StatusRunning
	case "stopped":
		status = task.StatusStopped
	case "paused":
		status = task.StatusPaused
	case "pausing":
		status = task.StatusPausing
	}
	sio := p.Stdio()
	return &shimapi.StateResponse{
		ID:         p.ID(),
		Bundle:     s.bundle,
		Pid:        uint32(p.Pid()),
		Status:     status,
		Stdin:      sio.Stdin,
		Stdout:     sio.Stdout,
		Stderr:     sio.Stderr,
		Terminal:   sio.Terminal,
		ExitStatus: uint32(p.ExitStatus()),
		ExitedAt:   p.ExitedAt(),
	}, nil
}

// Pause the container
func (s *Service) Pause(ctx context.Context, r *ptypes.Empty) (*ptypes.Empty, error) {
	p, err := s.getInitProcess()
	if err != nil {
		return nil, err
	}
	if err := p.(*process.Init).Pause(ctx); err != nil {
		return nil, err
	}
	return empty, nil
}

// Resume the container
func (s *Service) Resume(ctx context.Context, r *ptypes.Empty) (*ptypes.Empty, error) {
	p, err := s.getInitProcess()
	if err != nil {
		return nil, err
	}
	if err := p.(*process.Init).Resume(ctx); err != nil {
		return nil, err
	}
	return empty, nil
}

// Kill a process with the provided signal
func (s *Service) Kill(ctx context.Context, r *shimapi.KillRequest) (*ptypes.Empty, error) {
	if r.ID == "" {
		p, err := s.getInitProcess()
		if err != nil {
			return nil, err
		}
		if err := p.Kill(ctx, r.Signal, r.All); err != nil {
			return nil, errdefs.ToGRPC(err)
		}
		return empty, nil
	}

	p, err := s.getExecProcess(r.ID)
	if err != nil {
		return nil, err
	}
	if err := p.Kill(ctx, r.Signal, r.All); err != nil {
		return nil, errdefs.ToGRPC(err)
	}
	return empty, nil
}

// ListPids returns all pids inside the container
func (s *Service) ListPids(ctx context.Context, r *shimapi.ListPidsRequest) (*shimapi.ListPidsResponse, error) {
	pids, err := s.getContainerPids(ctx, r.ID)
	if err != nil {
		return nil, errdefs.ToGRPC(err)
	}
	var processes []*task.ProcessInfo

	s.mu.Lock()
	defer s.mu.Unlock()
	for _, pid := range pids {
		pInfo := task.ProcessInfo{
			Pid: pid,
		}
		for _, p := range s.processes {
			if p.Pid() == int(pid) {
				d := &runctypes.ProcessDetails{
					ExecID: p.ID(),
				}
				a, err := typeurl.MarshalAny(d)
				if err != nil {
					return nil, errors.Wrapf(err, "failed to marshal process %d info", pid)
				}
				pInfo.Info = a
				break
			}
		}
		processes = append(processes, &pInfo)
	}
	return &shimapi.ListPidsResponse{
		Processes: processes,
	}, nil
}

// CloseIO of a process
func (s *Service) CloseIO(ctx context.Context, r *shimapi.CloseIORequest) (*ptypes.Empty, error) {
	p, err := s.getExecProcess(r.ID)
	if err != nil {
		return nil, err
	}
	if stdin := p.Stdin(); stdin != nil {
		if err := stdin.Close(); err != nil {
			return nil, errors.Wrap(err, "close stdin")
		}
	}
	return empty, nil
}

// Checkpoint the container
func (s *Service) Checkpoint(ctx context.Context, r *shimapi.CheckpointTaskRequest) (*ptypes.Empty, error) {
	p, err := s.getInitProcess()
	if err != nil {
		return nil, err
	}
	var options runctypes.CheckpointOptions
	if r.Options != nil {
		v, err := typeurl.UnmarshalAny(r.Options)
		if err != nil {
			return nil, err
		}
		options = *v.(*runctypes.CheckpointOptions)
	}
	if err := p.(*process.Init).Checkpoint(ctx, &process.CheckpointConfig{
		Path:                     r.Path,
		Exit:                     options.Exit,
		AllowOpenTCP:             options.OpenTcp,
		AllowExternalUnixSockets: options.ExternalUnixSockets,
		AllowTerminal:            options.Terminal,
		FileLocks:                options.FileLocks,
		EmptyNamespaces:          options.EmptyNamespaces,
		WorkDir:                  options.WorkPath,
	}); err != nil {
		return nil, errdefs.ToGRPC(err)
	}
	return empty, nil
}

// ShimInfo returns shim information such as the shim's pid
func (s *Service) ShimInfo(ctx context.Context, r *ptypes.Empty) (*shimapi.ShimInfoResponse, error) {
	return &shimapi.ShimInfoResponse{
		ShimPid: uint32(os.Getpid()),
	}, nil
}

// Update a running container
func (s *Service) Update(ctx context.Context, r *shimapi.UpdateTaskRequest) (*ptypes.Empty, error) {
	p, err := s.getInitProcess()
	if err != nil {
		return nil, err
	}
	if err := p.(*process.Init).Update(ctx, r.Resources); err != nil {
		return nil, errdefs.ToGRPC(err)
	}
	return empty, nil
}

// Wait for a process to exit
func (s *Service) Wait(ctx context.Context, r *shimapi.WaitRequest) (*shimapi.WaitResponse, error) {
	p, err := s.getExecProcess(r.ID)
	if err != nil {
		return nil, err
	}
	p.Wait()

	return &shimapi.WaitResponse{
		ExitStatus: uint32(p.ExitStatus()),
		ExitedAt:   p.ExitedAt(),
	}, nil
}

func (s *Service) processExits() {
	for e := range s.ec {
		s.checkProcesses(e)
	}
}

func (s *Service) checkProcesses(e runc.Exit) {
	var p process.Process
	s.mu.Lock()
	for _, proc := range s.processes {
		if proc.Pid() == e.Pid {
			p = proc
			break
		}
	}
	s.mu.Unlock()
	if p == nil {
		log.G(s.context).Debugf("process with id:%d wasn't found", e.Pid)
		return
	}
	if ip, ok := p.(*process.Init); ok {
		// Ensure all children are killed
		if shouldKillAllOnExit(s.context, s.bundle) {
			if err := ip.KillAll(s.context); err != nil {
				log.G(s.context).WithError(err).WithField("id", ip.ID()).
					Error("failed to kill init's children")
			}
		}
	}

	p.SetExited(e.Status)
	s.events <- &eventstypes.TaskExit{
		ContainerID: s.id,
		ID:          p.ID(),
		Pid:         uint32(e.Pid),
		ExitStatus:  uint32(e.Status),
		ExitedAt:    p.ExitedAt(),
	}
}

func shouldKillAllOnExit(ctx context.Context, bundlePath string) bool {
	var bundleSpec specs.Spec
	bundleConfigContents, err := os.ReadFile(filepath.Join(bundlePath, "config.json"))
	if err != nil {
		log.G(ctx).WithError(err).Error("shouldKillAllOnExit: failed to read config.json")
		return true
	}
	if err := json.Unmarshal(bundleConfigContents, &bundleSpec); err != nil {
		log.G(ctx).WithError(err).Error("shouldKillAllOnExit: failed to unmarshal bundle json")
		return true
	}
	if bundleSpec.Linux != nil {
		for _, ns := range bundleSpec.Linux.Namespaces {
			if ns.Type == specs.PIDNamespace && ns.Path == "" {
				return false
			}
		}
	}
	return true
}

func (s *Service) getContainerPids(ctx context.Context, id string) ([]uint32, error) {
	p, err := s.getInitProcess()
	if err != nil {
		return nil, err
	}

	ps, err := p.(*process.Init).Runtime().Ps(ctx, id)
	if err != nil {
		return nil, err
	}
	pids := make([]uint32, 0, len(ps))
	for _, pid := range ps {
		pids = append(pids, uint32(pid))
	}
	return pids, nil
}

func (s *Service) forward(publisher events.Publisher) {
	for e := range s.events {
		if err := publisher.Publish(s.context, getTopic(s.context, e), e); err != nil {
			log.G(s.context).WithError(err).Error("post event")
		}
	}
}

// getInitProcess returns initial process
func (s *Service) getInitProcess() (process.Process, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	p := s.processes[s.id]
	if p == nil {
		return nil, errdefs.ToGRPCf(errdefs.ErrFailedPrecondition, "container must be created")
	}
	return p, nil
}

// getExecProcess returns exec process
func (s *Service) getExecProcess(id string) (process.Process, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	p := s.processes[id]
	if p == nil {
		return nil, errdefs.ToGRPCf(errdefs.ErrNotFound, "process %s does not exist", id)
	}
	return p, nil
}

func getTopic(ctx context.Context, e interface{}) string {
	switch e.(type) {
	case *eventstypes.TaskCreate:
		return runtime.TaskCreateEventTopic
	case *eventstypes.TaskStart:
		return runtime.TaskStartEventTopic
	case *eventstypes.TaskOOM:
		return runtime.TaskOOMEventTopic
	case *eventstypes.TaskExit:
		return runtime.TaskExitEventTopic
	case *eventstypes.TaskDelete:
		return runtime.TaskDeleteEventTopic
	case *eventstypes.TaskExecAdded:
		return runtime.TaskExecAddedEventTopic
	case *eventstypes.TaskExecStarted:
		return runtime.TaskExecStartedEventTopic
	case *eventstypes.TaskPaused:
		return runtime.TaskPausedEventTopic
	case *eventstypes.TaskResumed:
		return runtime.TaskResumedEventTopic
	case *eventstypes.TaskCheckpointed:
		return runtime.TaskCheckpointedEventTopic
	default:
		logrus.Warnf("no topic for type %#v", e)
	}
	return runtime.TaskUnknownTopic
}

func newInit(ctx context.Context, path, workDir, runtimeRoot, namespace, criu string, systemdCgroup bool, platform stdio.Platform, r *process.CreateConfig, rootfs string) (*process.Init, error) {
	var options runctypes.CreateOptions
	if r.Options != nil {
		v, err := typeurl.UnmarshalAny(r.Options)
		if err != nil {
			return nil, err
		}
		options = *v.(*runctypes.CreateOptions)
	}

	runtime := process.NewRunc(runtimeRoot, path, namespace, r.Runtime, criu, systemdCgroup)
	p := process.New(r.ID, runtime, stdio.Stdio{
		Stdin:    r.Stdin,
		Stdout:   r.Stdout,
		Stderr:   r.Stderr,
		Terminal: r.Terminal,
	})
	p.Bundle = r.Bundle
	p.Platform = platform
	p.Rootfs = rootfs
	p.WorkDir = workDir
	p.IoUID = int(options.IoUid)
	p.IoGID = int(options.IoGid)
	p.NoPivotRoot = options.NoPivotRoot
	p.NoNewKeyring = options.NoNewKeyring
	p.CriuWorkPath = options.CriuWorkPath
	if p.CriuWorkPath == "" {
		// if criu work path not set, use container WorkDir
		p.CriuWorkPath = p.WorkDir
	}

	return p, nil
}
