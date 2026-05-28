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

package builtin

import (
	"context"

	"github.com/containerd/nri/pkg/api"
)

// BuiltinPlugin implements the NRI API and runs in-process
// within the container runtime.
//
//nolint:revive // tautology builtin.Builtin*
type BuiltinPlugin struct {
	Base     string
	Index    string
	Handlers BuiltinHandlers
}

// BuiltinHandlers contains request handlers for the builtin plugin.
//
//nolint:revive // tautology builtin.Builtin*
type BuiltinHandlers struct {
	Configure            func(context.Context, *api.ConfigureRequest) (*api.ConfigureResponse, error)
	Synchronize          func(context.Context, *api.SynchronizeRequest) (*api.SynchronizeResponse, error)
	RunPodSandbox        func(context.Context, *api.RunPodSandboxRequest) error
	StopPodSandbox       func(context.Context, *api.StopPodSandboxRequest) error
	RemovePodSandbox     func(context.Context, *api.RemovePodSandboxRequest) error
	UpdatePodSandbox     func(context.Context, *api.UpdatePodSandboxRequest) (*api.UpdatePodSandboxResponse, error)
	PostUpdatePodSandbox func(context.Context, *api.PostUpdatePodSandboxRequest) error

	CreateContainer             func(context.Context, *api.CreateContainerRequest) (*api.CreateContainerResponse, error)
	PostCreateContainer         func(context.Context, *api.PostCreateContainerRequest) error
	StartContainer              func(context.Context, *api.StartContainerRequest) error
	PostStartContainer          func(context.Context, *api.PostStartContainerRequest) error
	UpdateContainer             func(context.Context, *api.UpdateContainerRequest) (*api.UpdateContainerResponse, error)
	PostUpdateContainer         func(context.Context, *api.PostUpdateContainerRequest) error
	StopContainer               func(context.Context, *api.StopContainerRequest) (*api.StopContainerResponse, error)
	RemoveContainer             func(context.Context, *api.RemoveContainerRequest) error
	ValidateContainerAdjustment func(context.Context, *api.ValidateContainerAdjustmentRequest) error
}

// Configure implements PluginService of the NRI API.
func (b *BuiltinPlugin) Configure(ctx context.Context, req *api.ConfigureRequest) (*api.ConfigureResponse, error) {
	var (
		rpl = &api.ConfigureResponse{}
		err error
	)

	if b.Handlers.Configure != nil {
		rpl, err = b.Handlers.Configure(ctx, req)
	}

	if rpl.Events == 0 {
		var events api.EventMask

		if b.Handlers.RunPodSandbox != nil {
			events.Set(api.Event_RUN_POD_SANDBOX)
		}
		if b.Handlers.StopPodSandbox != nil {
			events.Set(api.Event_STOP_POD_SANDBOX)
		}
		if b.Handlers.RemovePodSandbox != nil {
			events.Set(api.Event_REMOVE_POD_SANDBOX)
		}
		if b.Handlers.UpdatePodSandbox != nil {
			events.Set(api.Event_UPDATE_POD_SANDBOX)
		}
		if b.Handlers.PostUpdatePodSandbox != nil {
			events.Set(api.Event_POST_UPDATE_POD_SANDBOX)
		}
		if b.Handlers.CreateContainer != nil {
			events.Set(api.Event_CREATE_CONTAINER)
		}
		if b.Handlers.PostCreateContainer != nil {
			events.Set(api.Event_POST_CREATE_CONTAINER)
		}
		if b.Handlers.StartContainer != nil {
			events.Set(api.Event_START_CONTAINER)
		}
		if b.Handlers.PostStartContainer != nil {
			events.Set(api.Event_POST_START_CONTAINER)
		}
		if b.Handlers.UpdateContainer != nil {
			events.Set(api.Event_UPDATE_CONTAINER)
		}
		if b.Handlers.PostUpdateContainer != nil {
			events.Set(api.Event_POST_UPDATE_CONTAINER)
		}
		if b.Handlers.StopContainer != nil {
			events.Set(api.Event_STOP_CONTAINER)
		}
		if b.Handlers.RemoveContainer != nil {
			events.Set(api.Event_REMOVE_CONTAINER)
		}
		if b.Handlers.ValidateContainerAdjustment != nil {
			events.Set(api.Event_VALIDATE_CONTAINER_ADJUSTMENT)
		}

		rpl.Events = int32(events)
	}

	return rpl, err
}

// Synchronize implements PluginService of the NRI API.
func (b *BuiltinPlugin) Synchronize(ctx context.Context, req *api.SynchronizeRequest) (*api.SynchronizeResponse, error) {
	if b.Handlers.Synchronize != nil {
		return b.Handlers.Synchronize(ctx, req)
	}
	return &api.SynchronizeResponse{}, nil
}

// Shutdown implements PluginService of the NRI API.
func (b *BuiltinPlugin) Shutdown(context.Context, *api.ShutdownRequest) (*api.ShutdownResponse, error) {
	return &api.ShutdownResponse{}, nil
}

// RunPodSandbox implements PluginService of the NRI API.
func (b *BuiltinPlugin) RunPodSandbox(ctx context.Context, req *api.RunPodSandboxRequest) (*api.RunPodSandboxResponse, error) {
	if b.Handlers.RunPodSandbox != nil {
		return &api.RunPodSandboxResponse{}, b.Handlers.RunPodSandbox(ctx, req)
	}
	return &api.RunPodSandboxResponse{}, nil
}

// UpdatePodSandbox implements PluginService of the NRI API.
func (b *BuiltinPlugin) UpdatePodSandbox(ctx context.Context, req *api.UpdatePodSandboxRequest) (*api.UpdatePodSandboxResponse, error) {
	if b.Handlers.UpdatePodSandbox != nil {
		return b.Handlers.UpdatePodSandbox(ctx, req)
	}
	return &api.UpdatePodSandboxResponse{}, nil
}

// PostUpdatePodSandbox is a handler for the PostUpdatePodSandbox event.
func (b *BuiltinPlugin) PostUpdatePodSandbox(ctx context.Context, req *api.PostUpdatePodSandboxRequest) (*api.PostUpdatePodSandboxResponse, error) {
	if b.Handlers.PostUpdatePodSandbox != nil {
		return &api.PostUpdatePodSandboxResponse{}, b.Handlers.PostUpdatePodSandbox(ctx, req)
	}
	return &api.PostUpdatePodSandboxResponse{}, nil
}

// StopPodSandbox implements PluginService of the NRI API.
func (b *BuiltinPlugin) StopPodSandbox(ctx context.Context, req *api.StopPodSandboxRequest) (*api.StopPodSandboxResponse, error) {
	if b.Handlers.StopPodSandbox != nil {
		return &api.StopPodSandboxResponse{}, b.Handlers.StopPodSandbox(ctx, req)
	}
	return &api.StopPodSandboxResponse{}, nil
}

// RemovePodSandbox implements PluginService of the NRI API.
func (b *BuiltinPlugin) RemovePodSandbox(ctx context.Context, req *api.RemovePodSandboxRequest) (*api.RemovePodSandboxResponse, error) {
	if b.Handlers.RemovePodSandbox != nil {
		return &api.RemovePodSandboxResponse{}, b.Handlers.RemovePodSandbox(ctx, req)
	}
	return &api.RemovePodSandboxResponse{}, nil
}

// CreateContainer implements PluginService of the NRI API.
func (b *BuiltinPlugin) CreateContainer(ctx context.Context, req *api.CreateContainerRequest) (*api.CreateContainerResponse, error) {
	if b.Handlers.CreateContainer != nil {
		return b.Handlers.CreateContainer(ctx, req)
	}
	return &api.CreateContainerResponse{}, nil
}

// PostCreateContainer implements PluginsService of the NRI API.
func (b *BuiltinPlugin) PostCreateContainer(ctx context.Context, req *api.PostCreateContainerRequest) (*api.PostCreateContainerResponse, error) {
	if b.Handlers.PostCreateContainer != nil {
		return &api.PostCreateContainerResponse{}, b.Handlers.PostCreateContainer(ctx, req)
	}
	return &api.PostCreateContainerResponse{}, nil
}

// StartContainer implements PluginService of the NRI API.
func (b *BuiltinPlugin) StartContainer(ctx context.Context, req *api.StartContainerRequest) (*api.StartContainerResponse, error) {
	if b.Handlers.StartContainer != nil {
		return &api.StartContainerResponse{}, b.Handlers.StartContainer(ctx, req)
	}
	return &api.StartContainerResponse{}, nil
}

// PostStartContainer implements PluginService of the NRI API.
func (b *BuiltinPlugin) PostStartContainer(ctx context.Context, req *api.PostStartContainerRequest) (*api.PostStartContainerResponse, error) {
	if b.Handlers.PostStartContainer != nil {
		return &api.PostStartContainerResponse{}, b.Handlers.PostStartContainer(ctx, req)
	}
	return &api.PostStartContainerResponse{}, nil
}

// UpdateContainer implements PluginService of the NRI API.
func (b *BuiltinPlugin) UpdateContainer(ctx context.Context, req *api.UpdateContainerRequest) (*api.UpdateContainerResponse, error) {
	if b.Handlers.UpdateContainer != nil {
		return b.Handlers.UpdateContainer(ctx, req)
	}
	return &api.UpdateContainerResponse{}, nil
}

// PostUpdateContainer implements PluginService of the NRI API.
func (b *BuiltinPlugin) PostUpdateContainer(ctx context.Context, req *api.PostUpdateContainerRequest) (*api.PostUpdateContainerResponse, error) {
	if b.Handlers.PostUpdateContainer != nil {
		return &api.PostUpdateContainerResponse{}, b.Handlers.PostUpdateContainer(ctx, req)
	}
	return &api.PostUpdateContainerResponse{}, nil
}

// StopContainer implements PluginService of the NRI API.
func (b *BuiltinPlugin) StopContainer(ctx context.Context, req *api.StopContainerRequest) (*api.StopContainerResponse, error) {
	if b.Handlers.StopContainer != nil {
		return b.Handlers.StopContainer(ctx, req)
	}
	return &api.StopContainerResponse{}, nil
}

// RemoveContainer implements PluginService of the NRI API.
func (b *BuiltinPlugin) RemoveContainer(ctx context.Context, req *api.RemoveContainerRequest) (*api.RemoveContainerResponse, error) {
	if b.Handlers.RemoveContainer != nil {
		return &api.RemoveContainerResponse{}, b.Handlers.RemoveContainer(ctx, req)
	}
	return &api.RemoveContainerResponse{}, nil
}

// StateChange implements PluginService of the NRI API.
func (b *BuiltinPlugin) StateChange(_ context.Context, _ *api.StateChangeEvent) (*api.StateChangeResponse, error) {
	// TODO: remove eventually once StateChange is removed from the wire protocol.
	return &api.StateChangeResponse{}, nil
}

// ValidateContainerAdjustment implements PluginService of the NRI API.
func (b *BuiltinPlugin) ValidateContainerAdjustment(ctx context.Context, req *api.ValidateContainerAdjustmentRequest) (*api.ValidateContainerAdjustmentResponse, error) {
	if b.Handlers.ValidateContainerAdjustment != nil {
		if err := b.Handlers.ValidateContainerAdjustment(ctx, req); err != nil {
			return &api.ValidateContainerAdjustmentResponse{
				Reject: true,
				Reason: err.Error(),
			}, nil
		}
	}

	return &api.ValidateContainerAdjustmentResponse{}, nil
}
