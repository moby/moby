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

package tasks

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	api "github.com/containerd/containerd/api/services/tasks/v1"
	"github.com/containerd/containerd/api/types"
	"github.com/containerd/containerd/api/types/runc/options"
	"github.com/containerd/containerd/api/types/task"
	"github.com/containerd/errdefs"
	"github.com/containerd/errdefs/pkg/errgrpc"
	"github.com/containerd/log"
	"github.com/containerd/plugin"
	"github.com/containerd/plugin/registry"
	"github.com/containerd/typeurl/v2"
	"github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/containerd/containerd/v2/core/containers"
	"github.com/containerd/containerd/v2/core/content"
	"github.com/containerd/containerd/v2/core/events"
	"github.com/containerd/containerd/v2/core/images"
	"github.com/containerd/containerd/v2/core/metadata"
	"github.com/containerd/containerd/v2/core/mount"
	"github.com/containerd/containerd/v2/core/runtime"
	"github.com/containerd/containerd/v2/pkg/archive"
	"github.com/containerd/containerd/v2/pkg/blockio"
	"github.com/containerd/containerd/v2/pkg/filters"
	"github.com/containerd/containerd/v2/pkg/protobuf"
	"github.com/containerd/containerd/v2/pkg/protobuf/proto"
	ptypes "github.com/containerd/containerd/v2/pkg/protobuf/types"
	"github.com/containerd/containerd/v2/pkg/rdt"
	"github.com/containerd/containerd/v2/pkg/timeout"
	"github.com/containerd/containerd/v2/plugins"
	"github.com/containerd/containerd/v2/plugins/services"
)

var (
	_                    = (api.TasksClient)(&local{})
	empty                = &ptypes.Empty{}
	tasksServiceRequires = []plugin.Type{
		plugins.EventPlugin,
		plugins.RuntimePluginV2,
		plugins.MetadataPlugin,
		plugins.TaskMonitorPlugin,
	}
)

const (
	stateTimeout = "io.containerd.timeout.task.state"
)

// Config for the tasks service plugin
type Config struct {
	// BlockIOConfigFile specifies the path to blockio configuration file
	BlockIOConfigFile string `toml:"blockio_config_file" json:"blockioConfigFile"`
	// RdtConfigFile specifies the path to RDT configuration file
	RdtConfigFile string `toml:"rdt_config_file" json:"rdtConfigFile"`
}

func init() {
	registry.Register(&plugin.Registration{
		Type:     plugins.ServicePlugin,
		ID:       services.TasksService,
		Requires: tasksServiceRequires,
		Config:   &Config{},
		InitFn:   initFunc,
	})

	timeout.Set(stateTimeout, 2*time.Second)
}

func initFunc(ic *plugin.InitContext) (interface{}, error) {
	config := ic.Config.(*Config)

	v2r, err := ic.GetByID(plugins.RuntimePluginV2, "task")
	if err != nil {
		return nil, err
	}

	m, err := ic.GetSingle(plugins.MetadataPlugin)
	if err != nil {
		return nil, err
	}

	ep, err := ic.GetSingle(plugins.EventPlugin)
	if err != nil {
		return nil, err
	}

	monitor, err := ic.GetSingle(plugins.TaskMonitorPlugin)
	if err != nil {
		if !errors.Is(err, plugin.ErrPluginNotFound) {
			return nil, err
		}
		monitor = runtime.NewNoopMonitor()
	}

	db := m.(*metadata.DB)
	l := &local{
		containers: metadata.NewContainerStore(db),
		store:      db.ContentStore(),
		publisher:  ep.(events.Publisher),
		monitor:    monitor.(runtime.TaskMonitor),
		v2Runtime:  v2r.(runtime.PlatformRuntime),
	}

	v2Tasks, err := l.v2Runtime.Tasks(ic.Context, true)
	if err != nil {
		return nil, err
	}
	for _, t := range v2Tasks {
		l.monitor.Monitor(t, nil)
	}

	if err := blockio.SetConfig(config.BlockIOConfigFile); err != nil {
		log.G(ic.Context).WithError(err).Errorf("blockio initialization failed")
	}
	if err := rdt.SetConfig(config.RdtConfigFile); err != nil {
		log.G(ic.Context).WithError(err).Errorf("RDT initialization failed")
	}

	return l, nil
}

type local struct {
	containers containers.Store
	store      content.Store
	publisher  events.Publisher

	monitor   runtime.TaskMonitor
	v2Runtime runtime.PlatformRuntime
}

func (l *local) Create(ctx context.Context, r *api.CreateTaskRequest, _ ...grpc.CallOption) (*api.CreateTaskResponse, error) {
	container, err := l.getContainer(ctx, r.ContainerID)
	if err != nil {
		return nil, errgrpc.ToGRPC(err)
	}

	var (
		checkpointPath string
		taskAPIAddress string
		taskAPIVersion uint32
	)

	if r.Options != nil {
		taskOptions, err := formatOptions(container.Runtime.Name, r.Options)
		if err != nil {
			return nil, err
		}
		checkpointPath = taskOptions.CriuImagePath
		taskAPIAddress = taskOptions.TaskApiAddress
		taskAPIVersion = taskOptions.TaskApiVersion
	}

	restoreFromPath := false
	// For a restore via CRI.
	if r.Checkpoint != nil && r.Checkpoint.Annotations != nil {
		ann, ok := r.Checkpoint.Annotations["RestoreFromPath"]
		if ok {
			checkpointPath = ann
			restoreFromPath = true
		}
	}

	// jump get checkpointPath from checkpoint image
	if checkpointPath == "" && r.Checkpoint != nil {
		checkpointPath, err = os.MkdirTemp(os.Getenv("XDG_RUNTIME_DIR"), "ctrd-checkpoint")
		if err != nil {
			return nil, err
		}
		if r.Checkpoint.MediaType != images.MediaTypeContainerd1Checkpoint {
			return nil, fmt.Errorf("unsupported checkpoint type %q", r.Checkpoint.MediaType)
		}
		reader, err := l.store.ReaderAt(ctx, ocispec.Descriptor{
			MediaType:   r.Checkpoint.MediaType,
			Digest:      digest.Digest(r.Checkpoint.Digest),
			Size:        r.Checkpoint.Size,
			Annotations: r.Checkpoint.Annotations,
		})
		if err != nil {
			return nil, err
		}
		_, err = archive.Apply(ctx, checkpointPath, content.NewReader(reader))
		reader.Close()
		if err != nil {
			return nil, err
		}
	}

	opts := runtime.CreateOpts{
		Spec: container.Spec,
		IO: runtime.IO{
			Stdin:    r.Stdin,
			Stdout:   r.Stdout,
			Stderr:   r.Stderr,
			Terminal: r.Terminal,
		},
		Checkpoint:      checkpointPath,
		RestoreFromPath: restoreFromPath,
		Runtime:         container.Runtime.Name,
		RuntimeOptions:  container.Runtime.Options,
		TaskOptions:     r.Options,
		SandboxID:       container.SandboxID,
		Address:         taskAPIAddress,
		Version:         taskAPIVersion,
	}
	if r.RuntimePath != "" {
		opts.Runtime = r.RuntimePath
	}
	for _, m := range r.Rootfs {
		opts.Rootfs = append(opts.Rootfs, mount.Mount{
			Type:    m.Type,
			Source:  m.Source,
			Target:  m.Target,
			Options: m.Options,
		})
	}

	rtime := l.v2Runtime

	_, err = rtime.Get(ctx, r.ContainerID)
	if err != nil && !errdefs.IsNotFound(err) {
		return nil, errgrpc.ToGRPC(err)
	}
	if err == nil {
		return nil, errgrpc.ToGRPC(fmt.Errorf("task %s: %w", r.ContainerID, errdefs.ErrAlreadyExists))
	}
	c, err := rtime.Create(ctx, r.ContainerID, opts)
	if err != nil {
		return nil, errgrpc.ToGRPC(err)
	}
	labels := map[string]string{"runtime": container.Runtime.Name}
	if err := l.monitor.Monitor(c, labels); err != nil {
		return nil, fmt.Errorf("monitor task: %w", err)
	}
	pid, err := c.PID(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get task pid: %w", err)
	}
	return &api.CreateTaskResponse{
		ContainerID: r.ContainerID,
		Pid:         pid,
	}, nil
}

func (l *local) Start(ctx context.Context, r *api.StartRequest, _ ...grpc.CallOption) (*api.StartResponse, error) {
	t, err := l.getTask(ctx, r.ContainerID)
	if err != nil {
		return nil, err
	}
	p := runtime.Process(t)
	if r.ExecID != "" {
		if p, err = t.Process(ctx, r.ExecID); err != nil {
			return nil, errgrpc.ToGRPC(err)
		}
	}
	if err := p.Start(ctx); err != nil {
		return nil, errgrpc.ToGRPC(err)
	}
	state, err := p.State(ctx)
	if err != nil {
		return nil, errgrpc.ToGRPC(err)
	}
	return &api.StartResponse{
		Pid: state.Pid,
	}, nil
}

func (l *local) Delete(ctx context.Context, r *api.DeleteTaskRequest, _ ...grpc.CallOption) (*api.DeleteResponse, error) {
	container, err := l.getContainer(ctx, r.ContainerID)
	if err != nil {
		return nil, err
	}

	// Get task object
	t, err := l.v2Runtime.Get(ctx, container.ID)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "task %v not found", container.ID)
	}

	if err := l.monitor.Stop(t); err != nil {
		return nil, err
	}

	exit, err := l.v2Runtime.Delete(ctx, r.ContainerID)
	if err != nil {
		return nil, errgrpc.ToGRPC(err)
	}

	return &api.DeleteResponse{
		ExitStatus: exit.Status,
		ExitedAt:   protobuf.ToTimestamp(exit.Timestamp),
		Pid:        exit.Pid,
	}, nil
}

func (l *local) DeleteProcess(ctx context.Context, r *api.DeleteProcessRequest, _ ...grpc.CallOption) (*api.DeleteResponse, error) {
	t, err := l.getTask(ctx, r.ContainerID)
	if err != nil {
		return nil, err
	}
	process, err := t.Process(ctx, r.ExecID)
	if err != nil {
		return nil, errgrpc.ToGRPC(err)
	}
	exit, err := process.Delete(ctx)
	if err != nil {
		return nil, errgrpc.ToGRPC(err)
	}
	return &api.DeleteResponse{
		ID:         r.ExecID,
		ExitStatus: exit.Status,
		ExitedAt:   protobuf.ToTimestamp(exit.Timestamp),
		Pid:        exit.Pid,
	}, nil
}

func getProcessState(ctx context.Context, p runtime.Process) (*task.Process, error) {
	ctx, cancel := timeout.WithContext(ctx, stateTimeout)
	defer cancel()

	state, err := p.State(ctx)
	if err != nil {
		if errdefs.IsNotFound(err) || errdefs.IsUnavailable(err) {
			return nil, err
		}
		log.G(ctx).WithError(err).Errorf("get state for %s", p.ID())
	}
	status := task.Status_UNKNOWN
	switch state.Status {
	case runtime.CreatedStatus:
		status = task.Status_CREATED
	case runtime.RunningStatus:
		status = task.Status_RUNNING
	case runtime.StoppedStatus:
		status = task.Status_STOPPED
	case runtime.PausedStatus:
		status = task.Status_PAUSED
	case runtime.PausingStatus:
		status = task.Status_PAUSING
	default:
		log.G(ctx).WithField("status", state.Status).Warn("unknown status")
	}
	return &task.Process{
		ID:         p.ID(),
		Pid:        state.Pid,
		Status:     status,
		Stdin:      state.Stdin,
		Stdout:     state.Stdout,
		Stderr:     state.Stderr,
		Terminal:   state.Terminal,
		ExitStatus: state.ExitStatus,
		ExitedAt:   protobuf.ToTimestamp(state.ExitedAt),
	}, nil
}

func (l *local) Get(ctx context.Context, r *api.GetRequest, _ ...grpc.CallOption) (*api.GetResponse, error) {
	task, err := l.getTask(ctx, r.ContainerID)
	if err != nil {
		return nil, err
	}
	p := runtime.Process(task)
	if r.ExecID != "" {
		if p, err = task.Process(ctx, r.ExecID); err != nil {
			return nil, errgrpc.ToGRPC(err)
		}
	}
	t, err := getProcessState(ctx, p)
	if err != nil {
		return nil, errgrpc.ToGRPC(err)
	}
	return &api.GetResponse{
		Process: t,
	}, nil
}

func (l *local) List(ctx context.Context, r *api.ListTasksRequest, _ ...grpc.CallOption) (*api.ListTasksResponse, error) {
	resp := &api.ListTasksResponse{}
	tasks, err := l.v2Runtime.Tasks(ctx, false)
	if err != nil {
		return nil, errgrpc.ToGRPC(err)
	}
	addTasks(ctx, resp, tasks)
	return resp, nil
}

func addTasks(ctx context.Context, r *api.ListTasksResponse, tasks []runtime.Task) {
	for _, t := range tasks {
		tt, err := getProcessState(ctx, t)
		if err != nil {
			if !errdefs.IsNotFound(err) { // handle race with deletion
				log.G(ctx).WithError(err).WithField("id", t.ID()).Error("converting task to protobuf")
			}
			continue
		}
		r.Tasks = append(r.Tasks, tt)
	}
}

func (l *local) Pause(ctx context.Context, r *api.PauseTaskRequest, _ ...grpc.CallOption) (*ptypes.Empty, error) {
	t, err := l.getTask(ctx, r.ContainerID)
	if err != nil {
		return nil, err
	}
	err = t.Pause(ctx)
	if err != nil {
		return nil, errgrpc.ToGRPC(err)
	}
	return empty, nil
}

func (l *local) Resume(ctx context.Context, r *api.ResumeTaskRequest, _ ...grpc.CallOption) (*ptypes.Empty, error) {
	t, err := l.getTask(ctx, r.ContainerID)
	if err != nil {
		return nil, err
	}
	err = t.Resume(ctx)
	if err != nil {
		return nil, errgrpc.ToGRPC(err)
	}
	return empty, nil
}

func (l *local) Kill(ctx context.Context, r *api.KillRequest, _ ...grpc.CallOption) (*ptypes.Empty, error) {
	t, err := l.getTask(ctx, r.ContainerID)
	if err != nil {
		return nil, err
	}
	p := runtime.Process(t)
	if r.ExecID != "" {
		if p, err = t.Process(ctx, r.ExecID); err != nil {
			return nil, errgrpc.ToGRPC(err)
		}
	}
	if err := p.Kill(ctx, r.Signal, r.All); err != nil {
		return nil, errgrpc.ToGRPC(err)
	}
	return empty, nil
}

func (l *local) ListPids(ctx context.Context, r *api.ListPidsRequest, _ ...grpc.CallOption) (*api.ListPidsResponse, error) {
	t, err := l.getTask(ctx, r.ContainerID)
	if err != nil {
		return nil, err
	}
	processList, err := t.Pids(ctx)
	if err != nil {
		return nil, errgrpc.ToGRPC(err)
	}
	var processes []*task.ProcessInfo
	for _, p := range processList {
		pInfo := task.ProcessInfo{
			Pid: p.Pid,
		}
		if p.Info != nil {
			a, err := typeurl.MarshalAnyToProto(p.Info)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal process %d info: %w", p.Pid, err)
			}
			pInfo.Info = a
		}
		processes = append(processes, &pInfo)
	}
	return &api.ListPidsResponse{
		Processes: processes,
	}, nil
}

func (l *local) Exec(ctx context.Context, r *api.ExecProcessRequest, _ ...grpc.CallOption) (*ptypes.Empty, error) {
	if r.ExecID == "" {
		return nil, status.Errorf(codes.InvalidArgument, "exec id cannot be empty")
	}
	t, err := l.getTask(ctx, r.ContainerID)
	if err != nil {
		return nil, err
	}
	if _, err := t.Exec(ctx, r.ExecID, runtime.ExecOpts{
		Spec: r.Spec,
		IO: runtime.IO{
			Stdin:    r.Stdin,
			Stdout:   r.Stdout,
			Stderr:   r.Stderr,
			Terminal: r.Terminal,
		},
	}); err != nil {
		return nil, errgrpc.ToGRPC(err)
	}
	return empty, nil
}

func (l *local) ResizePty(ctx context.Context, r *api.ResizePtyRequest, _ ...grpc.CallOption) (*ptypes.Empty, error) {
	t, err := l.getTask(ctx, r.ContainerID)
	if err != nil {
		return nil, err
	}
	p := runtime.Process(t)
	if r.ExecID != "" {
		if p, err = t.Process(ctx, r.ExecID); err != nil {
			return nil, errgrpc.ToGRPC(err)
		}
	}
	if err := p.ResizePty(ctx, runtime.ConsoleSize{
		Width:  r.Width,
		Height: r.Height,
	}); err != nil {
		return nil, errgrpc.ToGRPC(err)
	}
	return empty, nil
}

func (l *local) CloseIO(ctx context.Context, r *api.CloseIORequest, _ ...grpc.CallOption) (*ptypes.Empty, error) {
	t, err := l.getTask(ctx, r.ContainerID)
	if err != nil {
		return nil, err
	}
	p := runtime.Process(t)
	if r.ExecID != "" {
		if p, err = t.Process(ctx, r.ExecID); err != nil {
			return nil, errgrpc.ToGRPC(err)
		}
	}
	if r.Stdin {
		if err := p.CloseIO(ctx); err != nil {
			return nil, errgrpc.ToGRPC(err)
		}
	}
	return empty, nil
}

func (l *local) Checkpoint(ctx context.Context, r *api.CheckpointTaskRequest, _ ...grpc.CallOption) (*api.CheckpointTaskResponse, error) {
	container, err := l.getContainer(ctx, r.ContainerID)
	if err != nil {
		return nil, err
	}
	t, err := l.getTaskFromContainer(ctx, container)
	if err != nil {
		return nil, err
	}
	image, err := getCheckpointPath(container.Runtime.Name, r.Options)
	if err != nil {
		return nil, err
	}
	checkpointImageExists := false
	if image == "" {
		checkpointImageExists = true
		image, err = os.MkdirTemp(os.Getenv("XDG_RUNTIME_DIR"), "ctrd-checkpoint")
		if err != nil {
			return nil, errgrpc.ToGRPC(err)
		}
		defer os.RemoveAll(image)
	}
	if err := t.Checkpoint(ctx, image, r.Options); err != nil {
		return nil, errgrpc.ToGRPC(err)
	}
	// do not commit checkpoint image if checkpoint ImagePath is passed,
	// return if checkpointImageExists is false
	if !checkpointImageExists {
		return &api.CheckpointTaskResponse{}, nil
	}
	// write checkpoint to the content store
	tar := archive.Diff(ctx, "", image)
	cp, err := l.writeContent(ctx, images.MediaTypeContainerd1Checkpoint, image, tar)
	// close tar first after write
	if err := tar.Close(); err != nil {
		return nil, err
	}
	if err != nil {
		return nil, err
	}
	// write the config to the content store
	pbany := typeurl.MarshalProto(container.Spec)
	data, err := proto.Marshal(pbany)
	if err != nil {
		return nil, err
	}
	spec := bytes.NewReader(data)
	specD, err := l.writeContent(ctx, images.MediaTypeContainerd1CheckpointConfig, filepath.Join(image, "spec"), spec)
	if err != nil {
		return nil, errgrpc.ToGRPC(err)
	}
	return &api.CheckpointTaskResponse{
		Descriptors: []*types.Descriptor{
			cp,
			specD,
		},
	}, nil
}

func (l *local) Update(ctx context.Context, r *api.UpdateTaskRequest, _ ...grpc.CallOption) (*ptypes.Empty, error) {
	t, err := l.getTask(ctx, r.ContainerID)
	if err != nil {
		return nil, err
	}
	if err := t.Update(ctx, r.Resources, r.Annotations); err != nil {
		return nil, errgrpc.ToGRPC(err)
	}
	return empty, nil
}

func (l *local) Metrics(ctx context.Context, r *api.MetricsRequest, _ ...grpc.CallOption) (*api.MetricsResponse, error) {
	filter, err := filters.ParseAll(r.Filters...)
	if err != nil {
		return nil, err
	}
	var resp api.MetricsResponse
	tasks, err := l.v2Runtime.Tasks(ctx, false)
	if err != nil {
		return nil, err
	}
	getTasksMetrics(ctx, filter, tasks, &resp)
	return &resp, nil
}

func (l *local) Wait(ctx context.Context, r *api.WaitRequest, _ ...grpc.CallOption) (*api.WaitResponse, error) {
	t, err := l.getTask(ctx, r.ContainerID)
	if err != nil {
		return nil, err
	}
	p := runtime.Process(t)
	if r.ExecID != "" {
		if p, err = t.Process(ctx, r.ExecID); err != nil {
			return nil, errgrpc.ToGRPC(err)
		}
	}
	exit, err := p.Wait(ctx)
	if err != nil {
		return nil, errgrpc.ToGRPC(err)
	}
	return &api.WaitResponse{
		ExitStatus: exit.Status,
		ExitedAt:   protobuf.ToTimestamp(exit.Timestamp),
	}, nil
}

func getTasksMetrics(ctx context.Context, filter filters.Filter, tasks []runtime.Task, r *api.MetricsResponse) {
	for _, tk := range tasks {
		if !filter.Match(filters.AdapterFunc(func(fieldpath []string) (string, bool) {
			t := tk
			switch fieldpath[0] {
			case "id":
				return t.ID(), true
			case "namespace":
				return t.Namespace(), true
			case "runtime":
				// return t.Info().Runtime, true
			}
			return "", false
		})) {
			continue
		}
		collected := time.Now()
		stats, err := tk.Stats(ctx)
		if err != nil {
			if !errdefs.IsNotFound(err) {
				log.G(ctx).WithError(err).Errorf("collecting metrics for %s", tk.ID())
			}
			continue
		}
		r.Metrics = append(r.Metrics, &types.Metric{
			Timestamp: protobuf.ToTimestamp(collected),
			ID:        tk.ID(),
			Data:      stats,
		})
	}
}

func (l *local) writeContent(ctx context.Context, mediaType, ref string, r io.Reader) (*types.Descriptor, error) {
	writer, err := l.store.Writer(ctx, content.WithRef(ref), content.WithDescriptor(ocispec.Descriptor{MediaType: mediaType}))
	if err != nil {
		return nil, err
	}
	defer writer.Close()
	size, err := io.Copy(writer, r)
	if err != nil {
		return nil, err
	}
	if err := writer.Commit(ctx, 0, ""); err != nil && !errdefs.IsAlreadyExists(err) {
		return nil, err
	}
	return &types.Descriptor{
		MediaType:   mediaType,
		Digest:      writer.Digest().String(),
		Size:        size,
		Annotations: make(map[string]string),
	}, nil
}

func (l *local) getContainer(ctx context.Context, id string) (*containers.Container, error) {
	var container containers.Container
	container, err := l.containers.Get(ctx, id)
	if err != nil {
		return nil, errgrpc.ToGRPC(err)
	}
	return &container, nil
}

func (l *local) getTask(ctx context.Context, id string) (runtime.Task, error) {
	container, err := l.getContainer(ctx, id)
	if err != nil {
		return nil, err
	}
	return l.getTaskFromContainer(ctx, container)
}

func (l *local) getTaskFromContainer(ctx context.Context, container *containers.Container) (runtime.Task, error) {
	t, err := l.v2Runtime.Get(ctx, container.ID)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "task %v not found", container.ID)
	}
	return t, nil
}

// getCheckpointPath only suitable for runc runtime now
func getCheckpointPath(runtime string, option *ptypes.Any) (string, error) {
	if option == nil {
		return "", nil
	}

	var checkpointPath string
	v, err := typeurl.UnmarshalAny(option)
	if err != nil {
		return "", err
	}
	opts, ok := v.(*options.CheckpointOptions)
	if !ok {
		return "", fmt.Errorf("invalid task checkpoint option for %s", runtime)
	}
	checkpointPath = opts.ImagePath

	return checkpointPath, nil
}

func formatOptions(runtime string, option *ptypes.Any) (*options.Options, error) {
	v, err := typeurl.UnmarshalAny(option)
	if err != nil {
		return nil, err
	}
	opts, ok := v.(*options.Options)
	if !ok {
		return nil, fmt.Errorf("invalid task create option for %s", runtime)
	}
	return opts, nil
}
