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
	"context"

	api "github.com/containerd/containerd/api/services/tasks/v1"
	ptypes "github.com/containerd/containerd/v2/pkg/protobuf/types"
	"github.com/containerd/containerd/v2/plugins"
	"github.com/containerd/containerd/v2/plugins/services"
	"github.com/containerd/plugin"
	"github.com/containerd/plugin/registry"
	"google.golang.org/grpc"
)

var (
	_ = (api.TasksServer)(&service{})
)

func init() {
	registry.Register(&plugin.Registration{
		Type: plugins.GRPCPlugin,
		ID:   "tasks",
		Requires: []plugin.Type{
			plugins.ServicePlugin,
		},
		InitFn: func(ic *plugin.InitContext) (interface{}, error) {
			i, err := ic.GetByID(plugins.ServicePlugin, services.TasksService)
			if err != nil {
				return nil, err
			}
			return &service{local: i.(api.TasksClient)}, nil
		},
	})
}

type service struct {
	local api.TasksClient
	api.UnimplementedTasksServer
}

func (s *service) Register(server *grpc.Server) error {
	api.RegisterTasksServer(server, s)
	return nil
}

func (s *service) Create(ctx context.Context, r *api.CreateTaskRequest) (*api.CreateTaskResponse, error) {
	return s.local.Create(ctx, r)
}

func (s *service) Start(ctx context.Context, r *api.StartRequest) (*api.StartResponse, error) {
	return s.local.Start(ctx, r)
}

func (s *service) Delete(ctx context.Context, r *api.DeleteTaskRequest) (*api.DeleteResponse, error) {
	return s.local.Delete(ctx, r)
}

func (s *service) DeleteProcess(ctx context.Context, r *api.DeleteProcessRequest) (*api.DeleteResponse, error) {
	return s.local.DeleteProcess(ctx, r)
}

func (s *service) Get(ctx context.Context, r *api.GetRequest) (*api.GetResponse, error) {
	return s.local.Get(ctx, r)
}

func (s *service) List(ctx context.Context, r *api.ListTasksRequest) (*api.ListTasksResponse, error) {
	return s.local.List(ctx, r)
}

func (s *service) Pause(ctx context.Context, r *api.PauseTaskRequest) (*ptypes.Empty, error) {
	return s.local.Pause(ctx, r)
}

func (s *service) Resume(ctx context.Context, r *api.ResumeTaskRequest) (*ptypes.Empty, error) {
	return s.local.Resume(ctx, r)
}

func (s *service) Kill(ctx context.Context, r *api.KillRequest) (*ptypes.Empty, error) {
	return s.local.Kill(ctx, r)
}

func (s *service) ListPids(ctx context.Context, r *api.ListPidsRequest) (*api.ListPidsResponse, error) {
	return s.local.ListPids(ctx, r)
}

func (s *service) Exec(ctx context.Context, r *api.ExecProcessRequest) (*ptypes.Empty, error) {
	return s.local.Exec(ctx, r)
}

func (s *service) ResizePty(ctx context.Context, r *api.ResizePtyRequest) (*ptypes.Empty, error) {
	return s.local.ResizePty(ctx, r)
}

func (s *service) CloseIO(ctx context.Context, r *api.CloseIORequest) (*ptypes.Empty, error) {
	return s.local.CloseIO(ctx, r)
}

func (s *service) Checkpoint(ctx context.Context, r *api.CheckpointTaskRequest) (*api.CheckpointTaskResponse, error) {
	return s.local.Checkpoint(ctx, r)
}

func (s *service) Update(ctx context.Context, r *api.UpdateTaskRequest) (*ptypes.Empty, error) {
	return s.local.Update(ctx, r)
}

func (s *service) Metrics(ctx context.Context, r *api.MetricsRequest) (*api.MetricsResponse, error) {
	return s.local.Metrics(ctx, r)
}

func (s *service) Wait(ctx context.Context, r *api.WaitRequest) (*api.WaitResponse, error) {
	return s.local.Wait(ctx, r)
}
