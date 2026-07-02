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

package v2

import (
	"context"
	"fmt"

	"github.com/containerd/ttrpc"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/emptypb"

	v2 "github.com/containerd/containerd/api/runtime/task/v2"

	api "github.com/containerd/containerd/api/runtime/task/v3" // Current version used by TaskServiceClient
)

// TaskServiceClient exposes a client interface to shims, which aims to hide
// the underlying complexity and backward compatibility (v2 task service vs v3, TTRPC vs GRPC, etc).
type TaskServiceClient interface {
	State(context.Context, *api.StateRequest) (*api.StateResponse, error)
	Create(context.Context, *api.CreateTaskRequest) (*api.CreateTaskResponse, error)
	Start(context.Context, *api.StartRequest) (*api.StartResponse, error)
	Delete(context.Context, *api.DeleteRequest) (*api.DeleteResponse, error)
	Pids(context.Context, *api.PidsRequest) (*api.PidsResponse, error)
	Pause(context.Context, *api.PauseRequest) (*emptypb.Empty, error)
	Resume(context.Context, *api.ResumeRequest) (*emptypb.Empty, error)
	Checkpoint(context.Context, *api.CheckpointTaskRequest) (*emptypb.Empty, error)
	Kill(context.Context, *api.KillRequest) (*emptypb.Empty, error)
	Exec(context.Context, *api.ExecProcessRequest) (*emptypb.Empty, error)
	ResizePty(context.Context, *api.ResizePtyRequest) (*emptypb.Empty, error)
	CloseIO(context.Context, *api.CloseIORequest) (*emptypb.Empty, error)
	Update(context.Context, *api.UpdateTaskRequest) (*emptypb.Empty, error)
	Wait(context.Context, *api.WaitRequest) (*api.WaitResponse, error)
	Stats(context.Context, *api.StatsRequest) (*api.StatsResponse, error)
	Connect(context.Context, *api.ConnectRequest) (*api.ConnectResponse, error)
	Shutdown(context.Context, *api.ShutdownRequest) (*emptypb.Empty, error)
}

// NewTaskClient returns a new task client interface which handles both GRPC and TTRPC servers depending on the
// client object type passed in.
//
// Supported client types are:
// - *ttrpc.Client
// - grpc.ClientConnInterface
//
// Currently supported servers:
// - TTRPC v2 (compatibility with shims before 2.0)
// - TTRPC v3
// - GRPC v3
func NewTaskClient(client interface{}, version int) (TaskServiceClient, error) {
	switch c := client.(type) {
	case *ttrpc.Client:
		switch version {
		case 2:
			return &ttrpcV2Bridge{client: v2.NewTaskClient(c)}, nil
		case 3:
			return api.NewTTRPCTaskClient(c), nil
		default:
			return nil, fmt.Errorf("containerd client supports only v2 and v3 TTRPC task client (got %d)", version)
		}

	case grpc.ClientConnInterface:
		if version != 3 {
			return nil, fmt.Errorf("containerd client supports only v3 GRPC task service (got %d)", version)
		}

		return &grpcV3Bridge{api.NewTaskClient(c)}, nil
	default:
		return nil, fmt.Errorf("unsupported shim client type %T", c)
	}
}

// ttrpcV2Bridge is a bridge from TTRPC v2 task service.
type ttrpcV2Bridge struct {
	client v2.TaskService
}

var _ TaskServiceClient = (*ttrpcV2Bridge)(nil)

func (b *ttrpcV2Bridge) State(ctx context.Context, request *api.StateRequest) (*api.StateResponse, error) {
	resp, err := b.client.State(ctx, &v2.StateRequest{
		ID:     request.GetID(),
		ExecID: request.GetExecID(),
	})

	return &api.StateResponse{
		ID:         resp.GetID(),
		Bundle:     resp.GetBundle(),
		Pid:        resp.GetPid(),
		Status:     resp.GetStatus(),
		Stdin:      resp.GetStdin(),
		Stdout:     resp.GetStdout(),
		Stderr:     resp.GetStderr(),
		Terminal:   resp.GetTerminal(),
		ExitStatus: resp.GetExitStatus(),
		ExitedAt:   resp.GetExitedAt(),
		ExecID:     resp.GetExecID(),
	}, err
}

func (b *ttrpcV2Bridge) Create(ctx context.Context, request *api.CreateTaskRequest) (*api.CreateTaskResponse, error) {
	resp, err := b.client.Create(ctx, &v2.CreateTaskRequest{
		ID:               request.GetID(),
		Bundle:           request.GetBundle(),
		Rootfs:           request.GetRootfs(),
		Terminal:         request.GetTerminal(),
		Stdin:            request.GetStdin(),
		Stdout:           request.GetStdout(),
		Stderr:           request.GetStderr(),
		Checkpoint:       request.GetCheckpoint(),
		ParentCheckpoint: request.GetParentCheckpoint(),
		Options:          request.GetOptions(),
	})

	return &api.CreateTaskResponse{Pid: resp.GetPid()}, err
}

func (b *ttrpcV2Bridge) Start(ctx context.Context, request *api.StartRequest) (*api.StartResponse, error) {
	resp, err := b.client.Start(ctx, &v2.StartRequest{
		ID:     request.GetID(),
		ExecID: request.GetExecID(),
	})

	return &api.StartResponse{Pid: resp.GetPid()}, err
}

func (b *ttrpcV2Bridge) Delete(ctx context.Context, request *api.DeleteRequest) (*api.DeleteResponse, error) {
	resp, err := b.client.Delete(ctx, &v2.DeleteRequest{
		ID:     request.GetID(),
		ExecID: request.GetExecID(),
	})

	return &api.DeleteResponse{
		Pid:        resp.GetPid(),
		ExitStatus: resp.GetExitStatus(),
		ExitedAt:   resp.GetExitedAt(),
	}, err
}

func (b *ttrpcV2Bridge) Pids(ctx context.Context, request *api.PidsRequest) (*api.PidsResponse, error) {
	resp, err := b.client.Pids(ctx, &v2.PidsRequest{ID: request.GetID()})
	return &api.PidsResponse{Processes: resp.GetProcesses()}, err
}

func (b *ttrpcV2Bridge) Pause(ctx context.Context, request *api.PauseRequest) (*emptypb.Empty, error) {
	return b.client.Pause(ctx, &v2.PauseRequest{ID: request.GetID()})
}

func (b *ttrpcV2Bridge) Resume(ctx context.Context, request *api.ResumeRequest) (*emptypb.Empty, error) {
	return b.client.Resume(ctx, &v2.ResumeRequest{ID: request.GetID()})
}

func (b *ttrpcV2Bridge) Checkpoint(ctx context.Context, request *api.CheckpointTaskRequest) (*emptypb.Empty, error) {
	return b.client.Checkpoint(ctx, &v2.CheckpointTaskRequest{
		ID:      request.GetID(),
		Path:    request.GetPath(),
		Options: request.GetOptions(),
	})
}

func (b *ttrpcV2Bridge) Kill(ctx context.Context, request *api.KillRequest) (*emptypb.Empty, error) {
	return b.client.Kill(ctx, &v2.KillRequest{
		ID:     request.GetID(),
		ExecID: request.GetExecID(),
		Signal: request.GetSignal(),
		All:    request.GetAll(),
	})
}

func (b *ttrpcV2Bridge) Exec(ctx context.Context, request *api.ExecProcessRequest) (*emptypb.Empty, error) {
	return b.client.Exec(ctx, &v2.ExecProcessRequest{
		ID:       request.GetID(),
		ExecID:   request.GetExecID(),
		Terminal: request.GetTerminal(),
		Stdin:    request.GetStdin(),
		Stdout:   request.GetStdout(),
		Stderr:   request.GetStderr(),
		Spec:     request.GetSpec(),
	})
}

func (b *ttrpcV2Bridge) ResizePty(ctx context.Context, request *api.ResizePtyRequest) (*emptypb.Empty, error) {
	return b.client.ResizePty(ctx, &v2.ResizePtyRequest{
		ID:     request.GetID(),
		ExecID: request.GetExecID(),
		Width:  request.GetWidth(),
		Height: request.GetHeight(),
	})
}

func (b *ttrpcV2Bridge) CloseIO(ctx context.Context, request *api.CloseIORequest) (*emptypb.Empty, error) {
	return b.client.CloseIO(ctx, &v2.CloseIORequest{
		ID:     request.GetID(),
		ExecID: request.GetExecID(),
		Stdin:  request.GetStdin(),
	})
}

func (b *ttrpcV2Bridge) Update(ctx context.Context, request *api.UpdateTaskRequest) (*emptypb.Empty, error) {
	return b.client.Update(ctx, &v2.UpdateTaskRequest{
		ID:          request.GetID(),
		Resources:   request.GetResources(),
		Annotations: request.GetAnnotations(),
	})
}

func (b *ttrpcV2Bridge) Wait(ctx context.Context, request *api.WaitRequest) (*api.WaitResponse, error) {
	resp, err := b.client.Wait(ctx, &v2.WaitRequest{
		ID:     request.GetID(),
		ExecID: request.GetExecID(),
	})

	return &api.WaitResponse{
		ExitStatus: resp.GetExitStatus(),
		ExitedAt:   resp.GetExitedAt(),
	}, err
}

func (b *ttrpcV2Bridge) Stats(ctx context.Context, request *api.StatsRequest) (*api.StatsResponse, error) {
	resp, err := b.client.Stats(ctx, &v2.StatsRequest{ID: request.GetID()})
	return &api.StatsResponse{Stats: resp.GetStats()}, err
}

func (b *ttrpcV2Bridge) Connect(ctx context.Context, request *api.ConnectRequest) (*api.ConnectResponse, error) {
	resp, err := b.client.Connect(ctx, &v2.ConnectRequest{ID: request.GetID()})

	return &api.ConnectResponse{
		ShimPid: resp.GetShimPid(),
		TaskPid: resp.GetTaskPid(),
		Version: resp.GetVersion(),
	}, err
}

func (b *ttrpcV2Bridge) Shutdown(ctx context.Context, request *api.ShutdownRequest) (*emptypb.Empty, error) {
	return b.client.Shutdown(ctx, &v2.ShutdownRequest{
		ID:  request.GetID(),
		Now: request.GetNow(),
	})
}

// grpcV3Bridge implements task service client for v3 GRPC server.
// GRPC uses same request/response structures as TTRPC, so it just wraps GRPC calls.
type grpcV3Bridge struct {
	client api.TaskClient
}

var _ TaskServiceClient = (*grpcV3Bridge)(nil)

func (g *grpcV3Bridge) State(ctx context.Context, request *api.StateRequest) (*api.StateResponse, error) {
	return g.client.State(ctx, request)
}

func (g *grpcV3Bridge) Create(ctx context.Context, request *api.CreateTaskRequest) (*api.CreateTaskResponse, error) {
	return g.client.Create(ctx, request)
}

func (g *grpcV3Bridge) Start(ctx context.Context, request *api.StartRequest) (*api.StartResponse, error) {
	return g.client.Start(ctx, request)
}

func (g *grpcV3Bridge) Delete(ctx context.Context, request *api.DeleteRequest) (*api.DeleteResponse, error) {
	return g.client.Delete(ctx, request)
}

func (g *grpcV3Bridge) Pids(ctx context.Context, request *api.PidsRequest) (*api.PidsResponse, error) {
	return g.client.Pids(ctx, request)
}

func (g *grpcV3Bridge) Pause(ctx context.Context, request *api.PauseRequest) (*emptypb.Empty, error) {
	return g.client.Pause(ctx, request)
}

func (g *grpcV3Bridge) Resume(ctx context.Context, request *api.ResumeRequest) (*emptypb.Empty, error) {
	return g.client.Resume(ctx, request)
}

func (g *grpcV3Bridge) Checkpoint(ctx context.Context, request *api.CheckpointTaskRequest) (*emptypb.Empty, error) {
	return g.client.Checkpoint(ctx, request)
}

func (g *grpcV3Bridge) Kill(ctx context.Context, request *api.KillRequest) (*emptypb.Empty, error) {
	return g.client.Kill(ctx, request)
}

func (g *grpcV3Bridge) Exec(ctx context.Context, request *api.ExecProcessRequest) (*emptypb.Empty, error) {
	return g.client.Exec(ctx, request)
}

func (g *grpcV3Bridge) ResizePty(ctx context.Context, request *api.ResizePtyRequest) (*emptypb.Empty, error) {
	return g.client.ResizePty(ctx, request)
}

func (g *grpcV3Bridge) CloseIO(ctx context.Context, request *api.CloseIORequest) (*emptypb.Empty, error) {
	return g.client.CloseIO(ctx, request)
}

func (g *grpcV3Bridge) Update(ctx context.Context, request *api.UpdateTaskRequest) (*emptypb.Empty, error) {
	return g.client.Update(ctx, request)
}

func (g *grpcV3Bridge) Wait(ctx context.Context, request *api.WaitRequest) (*api.WaitResponse, error) {
	return g.client.Wait(ctx, request)
}

func (g *grpcV3Bridge) Stats(ctx context.Context, request *api.StatsRequest) (*api.StatsResponse, error) {
	return g.client.Stats(ctx, request)
}

func (g *grpcV3Bridge) Connect(ctx context.Context, request *api.ConnectRequest) (*api.ConnectResponse, error) {
	return g.client.Connect(ctx, request)
}

func (g *grpcV3Bridge) Shutdown(ctx context.Context, request *api.ShutdownRequest) (*emptypb.Empty, error) {
	return g.client.Shutdown(ctx, request)
}
