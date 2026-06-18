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

package adaptation

import (
	"context"
	"errors"

	"github.com/containerd/nri/pkg/api"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type pluginType struct {
	wasmImpl    api.Plugin        // WASM binary plugin loaded by the runtime
	ttrpcImpl   api.PluginService // external plugin connected over ttrpc
	builtinImpl api.PluginService // in-process plugin built into the runtime
	deprecated  map[Event]bool    // deprecations collected
	warned      map[Event]bool    // deprecations we have warned about
}

var (
	errUnknownImpl = errors.New("unknown plugin implementation type")
)

func (p *pluginType) isWasm() bool {
	return p.wasmImpl != nil
}

func (p *pluginType) isTtrpc() bool {
	return p.ttrpcImpl != nil
}

func (p *pluginType) isBuiltin() bool {
	return p.builtinImpl != nil
}

// Synchronize handles type-specific details of relaying a plugin Synchronize
// request.
func (p *pluginType) Synchronize(ctx context.Context, req *SynchronizeRequest) (*SynchronizeResponse, error) {
	switch {
	case p.ttrpcImpl != nil:
		return p.ttrpcImpl.Synchronize(ctx, req)
	case p.builtinImpl != nil:
		return p.builtinImpl.Synchronize(ctx, req)
	case p.wasmImpl != nil:
		return p.wasmImpl.Synchronize(ctx, req)
	}

	return nil, errUnknownImpl
}

// Configure handles type-specific details of relaying a plugin Configure request.
func (p *pluginType) Configure(ctx context.Context, req *ConfigureRequest) (*ConfigureResponse, error) {
	switch {
	case p.ttrpcImpl != nil:
		return p.ttrpcImpl.Configure(ctx, req)
	case p.builtinImpl != nil:
		return p.builtinImpl.Configure(ctx, req)
	case p.wasmImpl != nil:
		return p.wasmImpl.Configure(ctx, req)
	}

	return nil, errUnknownImpl
}

// RunPodSandbox handles type-specific deails of relaying a RunPodSandbox request.
func (p *pluginType) RunPodSandbox(ctx context.Context, req *RunPodSandboxRequest) (*RunPodSandboxResponse, error) {
	switch {
	case p.ttrpcImpl != nil:
		rpl, err := p.ttrpcImpl.RunPodSandbox(ctx, req)
		if err != nil && status.Code(err) == codes.Unimplemented {
			rpl, err = wrapOrigImpl(p).RunPodSandbox(ctx, req)
		}
		return rpl, err
	case p.builtinImpl != nil:
		return p.builtinImpl.RunPodSandbox(ctx, req)
	case p.wasmImpl != nil:
		return p.wasmImpl.RunPodSandbox(ctx, req)
	}

	return nil, errUnknownImpl
}

// UpdatePodSandbox handles type-specific details of relaying an UpdatePodSandbox
// request.
func (p *pluginType) UpdatePodSandbox(ctx context.Context, req *UpdatePodSandboxRequest) (*UpdatePodSandboxResponse, error) {
	switch {
	case p.ttrpcImpl != nil:
		return p.ttrpcImpl.UpdatePodSandbox(ctx, req)
	case p.builtinImpl != nil:
		return p.builtinImpl.UpdatePodSandbox(ctx, req)
	case p.wasmImpl != nil:
		return p.wasmImpl.UpdatePodSandbox(ctx, req)
	}

	return nil, errUnknownImpl
}

// PostUpdatePodSandbox handles type-specific details of relaying a
// PostUpdatePodSandbox request.
func (p *pluginType) PostUpdatePodSandbox(ctx context.Context, req *PostUpdatePodSandboxRequest) (*PostUpdatePodSandboxResponse, error) {
	switch {
	case p.ttrpcImpl != nil:
		rpl, err := p.ttrpcImpl.PostUpdatePodSandbox(ctx, req)
		if err != nil && status.Code(err) == codes.Unimplemented {
			rpl, err = wrapOrigImpl(p).PostUpdatePodSandbox(ctx, req)
		}
		return rpl, err
	case p.builtinImpl != nil:
		return p.builtinImpl.PostUpdatePodSandbox(ctx, req)
	case p.wasmImpl != nil:
		return p.wasmImpl.PostUpdatePodSandbox(ctx, req)
	}

	return nil, errUnknownImpl
}

// StopPodSandbox handles type-specific details of relaying a StopPodSandbox request.
func (p *pluginType) StopPodSandbox(ctx context.Context, req *StopPodSandboxRequest) (*StopPodSandboxResponse, error) {
	switch {
	case p.ttrpcImpl != nil:
		rpl, err := p.ttrpcImpl.StopPodSandbox(ctx, req)
		if err != nil && status.Code(err) == codes.Unimplemented {
			rpl, err = wrapOrigImpl(p).StopPodSandbox(ctx, req)
		}
		return rpl, err
	case p.builtinImpl != nil:
		return p.builtinImpl.StopPodSandbox(ctx, req)
	case p.wasmImpl != nil:
		return p.wasmImpl.StopPodSandbox(ctx, req)
	}

	return nil, errUnknownImpl
}

// RemovePodSandbox handles type-specific details of relaying a RemovePodSandbox
// request.
func (p *pluginType) RemovePodSandbox(ctx context.Context, req *RemovePodSandboxRequest) (*RemovePodSandboxResponse, error) {
	switch {
	case p.ttrpcImpl != nil:
		rpl, err := p.ttrpcImpl.RemovePodSandbox(ctx, req)
		if err != nil && status.Code(err) == codes.Unimplemented {
			rpl, err = wrapOrigImpl(p).RemovePodSandbox(ctx, req)
		}
		return rpl, err
	case p.builtinImpl != nil:
		return p.builtinImpl.RemovePodSandbox(ctx, req)
	case p.wasmImpl != nil:
		return p.wasmImpl.RemovePodSandbox(ctx, req)
	}

	return nil, errUnknownImpl
}

// CreateContainer handles type-specific details of relaying a CreateContainer request.
func (p *pluginType) CreateContainer(ctx context.Context, req *CreateContainerRequest) (*CreateContainerResponse, error) {
	switch {
	case p.ttrpcImpl != nil:
		return p.ttrpcImpl.CreateContainer(ctx, req)
	case p.builtinImpl != nil:
		return p.builtinImpl.CreateContainer(ctx, req)
	case p.wasmImpl != nil:
		return p.wasmImpl.CreateContainer(ctx, req)
	}

	return nil, errUnknownImpl
}

// PostCreateContainer handles type-specific details of relaying a PostCreateContainer
// request.
func (p *pluginType) PostCreateContainer(ctx context.Context, req *PostCreateContainerRequest) (*PostCreateContainerResponse, error) {
	switch {
	case p.ttrpcImpl != nil:
		rpl, err := p.ttrpcImpl.PostCreateContainer(ctx, req)
		if err != nil && status.Code(err) == codes.Unimplemented {
			rpl, err = wrapOrigImpl(p).PostCreateContainer(ctx, req)
		}
		return rpl, err
	case p.builtinImpl != nil:
		return p.builtinImpl.PostCreateContainer(ctx, req)
	case p.wasmImpl != nil:
		return p.wasmImpl.PostCreateContainer(ctx, req)
	}

	return nil, errUnknownImpl
}

// StartContainer handles type-specific details of relaying a StartContainer request.
func (p *pluginType) StartContainer(ctx context.Context, req *StartContainerRequest) (*StartContainerResponse, error) {
	switch {
	case p.ttrpcImpl != nil:
		rpl, err := p.ttrpcImpl.StartContainer(ctx, req)
		if err != nil && status.Code(err) == codes.Unimplemented {
			rpl, err = wrapOrigImpl(p).StartContainer(ctx, req)
		}
		return rpl, err
	case p.builtinImpl != nil:
		return p.builtinImpl.StartContainer(ctx, req)
	case p.wasmImpl != nil:
		return p.wasmImpl.StartContainer(ctx, req)
	}

	return nil, errUnknownImpl
}

// PostStartContainer handles type-specific details of relaying a PostStartContainer
// request.
func (p *pluginType) PostStartContainer(ctx context.Context, req *PostStartContainerRequest) (*PostStartContainerResponse, error) {
	switch {
	case p.ttrpcImpl != nil:
		rpl, err := p.ttrpcImpl.PostStartContainer(ctx, req)
		if err != nil && status.Code(err) == codes.Unimplemented {
			rpl, err = wrapOrigImpl(p).PostStartContainer(ctx, req)
		}
		return rpl, err
	case p.builtinImpl != nil:
		return p.builtinImpl.PostStartContainer(ctx, req)
	case p.wasmImpl != nil:
		return p.wasmImpl.PostStartContainer(ctx, req)
	}

	return nil, errUnknownImpl
}

// UpdateContainer handles type-specific details of relaying an UpdateContainer request.
func (p *pluginType) UpdateContainer(ctx context.Context, req *UpdateContainerRequest) (*UpdateContainerResponse, error) {
	switch {
	case p.ttrpcImpl != nil:
		return p.ttrpcImpl.UpdateContainer(ctx, req)
	case p.builtinImpl != nil:
		return p.builtinImpl.UpdateContainer(ctx, req)
	case p.wasmImpl != nil:
		return p.wasmImpl.UpdateContainer(ctx, req)
	}

	return nil, errUnknownImpl
}

// PostUpdateContainer handles type-specific details of relaying a PostUpdateContainer
// request.
func (p *pluginType) PostUpdateContainer(ctx context.Context, req *PostUpdateContainerRequest) (*PostUpdateContainerResponse, error) {
	switch {
	case p.ttrpcImpl != nil:
		rpl, err := p.ttrpcImpl.PostUpdateContainer(ctx, req)
		if err != nil && status.Code(err) == codes.Unimplemented {
			rpl, err = wrapOrigImpl(p).PostUpdateContainer(ctx, req)
		}
		return rpl, err
	case p.builtinImpl != nil:
		return p.builtinImpl.PostUpdateContainer(ctx, req)
	case p.wasmImpl != nil:
		return p.wasmImpl.PostUpdateContainer(ctx, req)
	}

	return nil, errUnknownImpl
}

// StopContainer handles type-specific details of relaying a StopContainer request.
func (p *pluginType) StopContainer(ctx context.Context, req *StopContainerRequest) (*StopContainerResponse, error) {
	switch {
	case p.ttrpcImpl != nil:
		return p.ttrpcImpl.StopContainer(ctx, req)
	case p.builtinImpl != nil:
		return p.builtinImpl.StopContainer(ctx, req)
	case p.wasmImpl != nil:
		return p.wasmImpl.StopContainer(ctx, req)
	}

	return nil, errUnknownImpl
}

// RemoveContainer handles type-specific details of relaying a RemoveContainer request.
func (p *pluginType) RemoveContainer(ctx context.Context, req *RemoveContainerRequest) (*RemoveContainerResponse, error) {
	switch {
	case p.ttrpcImpl != nil:
		rpl, err := p.ttrpcImpl.RemoveContainer(ctx, req)
		if err != nil && status.Code(err) == codes.Unimplemented {
			rpl, err = wrapOrigImpl(p).RemoveContainer(ctx, req)
		}
		return rpl, err
	case p.builtinImpl != nil:
		return p.builtinImpl.RemoveContainer(ctx, req)
	case p.wasmImpl != nil:
		return p.wasmImpl.RemoveContainer(ctx, req)
	}

	return nil, errUnknownImpl
}

// StateChange handles type-specific details of relaying a StateChange request.
func (p *pluginType) StateChange(ctx context.Context, req *StateChangeEvent) (err error) {
	switch {
	case p.ttrpcImpl != nil:
		_, err = p.ttrpcImpl.StateChange(ctx, req)
	case p.builtinImpl != nil:
		_, err = p.builtinImpl.StateChange(ctx, req)
	case p.wasmImpl != nil:
		_, err = p.wasmImpl.StateChange(ctx, req)
	default:
		err = errUnknownImpl
	}
	return err
}

// ValidateContainerAdjustment handles type-specific details of relaying a
// ValidateContainerAdjustment request.
func (p *pluginType) ValidateContainerAdjustment(ctx context.Context, req *ValidateContainerAdjustmentRequest) (*ValidateContainerAdjustmentResponse, error) {
	switch {
	case p.ttrpcImpl != nil:
		return p.ttrpcImpl.ValidateContainerAdjustment(ctx, req)
	case p.builtinImpl != nil:
		return p.builtinImpl.ValidateContainerAdjustment(ctx, req)
	case p.wasmImpl != nil:
		return p.wasmImpl.ValidateContainerAdjustment(ctx, req)
	}

	return nil, errUnknownImpl
}

type origImplWrapper struct {
	p *pluginType
	api.PluginService
}

var _ api.PluginService = (*origImplWrapper)(nil)

func wrapOrigImpl(p *pluginType) *origImplWrapper {
	if w, ok := p.ttrpcImpl.(*origImplWrapper); ok {
		return w
	}

	p.deprecated = make(map[Event]bool)
	p.warned = make(map[Event]bool)

	w := &origImplWrapper{
		PluginService: p.ttrpcImpl,
		p:             p,
	}
	p.ttrpcImpl = w

	return w
}

// RunPodSandbox funnels RunPodSandbox for old plugins over StateChange.
func (o *origImplWrapper) RunPodSandbox(ctx context.Context, req *RunPodSandboxRequest) (*RunPodSandboxResponse, error) {
	event := Event_RUN_POD_SANDBOX
	o.p.deprecated[event] = true
	_, err := o.PluginService.StateChange(ctx, &StateChangeEvent{
		Event: event,
		Pod:   req.GetPod(),
	})
	return &api.RunPodSandboxResponse{}, err
}

// PostUpdatePodSandbox funnels PostUpdatePodSandbox for old plugins over StateChange.
func (o *origImplWrapper) PostUpdatePodSandbox(ctx context.Context, req *PostUpdatePodSandboxRequest) (*PostUpdatePodSandboxResponse, error) {
	event := Event_POST_UPDATE_POD_SANDBOX
	o.p.deprecated[event] = true
	_, err := o.PluginService.StateChange(ctx, &StateChangeEvent{
		Event: event,
		Pod:   req.GetPod(),
	})
	return &api.PostUpdatePodSandboxResponse{}, err
}

// StopPodSandbox funnels StopPodSandbox for old plugins over StateChange.
func (o *origImplWrapper) StopPodSandbox(ctx context.Context, req *StopPodSandboxRequest) (*StopPodSandboxResponse, error) {
	event := Event_STOP_POD_SANDBOX
	o.p.deprecated[event] = true
	_, err := o.PluginService.StateChange(ctx, &StateChangeEvent{
		Event: event,
		Pod:   req.GetPod(),
	})
	return &api.StopPodSandboxResponse{}, err
}

// RemovePodSandbox funnels RemovePodSandbox for old plugins over StateChange.
func (o *origImplWrapper) RemovePodSandbox(ctx context.Context, req *RemovePodSandboxRequest) (*RemovePodSandboxResponse, error) {
	event := Event_REMOVE_POD_SANDBOX
	o.p.deprecated[event] = true
	_, err := o.PluginService.StateChange(ctx, &StateChangeEvent{
		Event: event,
		Pod:   req.GetPod(),
	})
	return &api.RemovePodSandboxResponse{}, err
}

// PostCreateContainer funnels PostCreateContainer for old plugins over StateChange.
func (o *origImplWrapper) PostCreateContainer(ctx context.Context, req *PostCreateContainerRequest) (*PostCreateContainerResponse, error) {
	event := Event_POST_CREATE_CONTAINER
	o.p.deprecated[event] = true
	_, err := o.PluginService.StateChange(ctx, &StateChangeEvent{
		Event:     event,
		Pod:       req.GetPod(),
		Container: req.GetContainer(),
	})
	return &api.PostCreateContainerResponse{}, err
}

// StartContainer funnels StartContainer for old plugins over StateChange.
func (o *origImplWrapper) StartContainer(ctx context.Context, req *StartContainerRequest) (*StartContainerResponse, error) {
	event := Event_START_CONTAINER
	o.p.deprecated[event] = true
	_, err := o.PluginService.StateChange(ctx, &StateChangeEvent{
		Event:     event,
		Pod:       req.GetPod(),
		Container: req.GetContainer(),
	})
	return &api.StartContainerResponse{}, err
}

// PostStartContainer funnels PostStartContainer for old plugins over StateChange.
func (o *origImplWrapper) PostStartContainer(ctx context.Context, req *PostStartContainerRequest) (*PostStartContainerResponse, error) {
	event := Event_POST_START_CONTAINER
	o.p.deprecated[event] = true
	_, err := o.PluginService.StateChange(ctx, &StateChangeEvent{
		Event:     event,
		Pod:       req.GetPod(),
		Container: req.GetContainer(),
	})
	return &api.PostStartContainerResponse{}, err
}

func (o *origImplWrapper) PostUpdateContainer(ctx context.Context, req *PostUpdateContainerRequest) (*PostUpdateContainerResponse, error) {
	event := Event_POST_UPDATE_CONTAINER
	o.p.deprecated[event] = true
	_, err := o.PluginService.StateChange(ctx, &StateChangeEvent{
		Event:     event,
		Pod:       req.GetPod(),
		Container: req.GetContainer(),
	})
	return &api.PostUpdateContainerResponse{}, err
}

// RemoveContainer funnels RemoveContainer for old plugins over StateChange.
func (o *origImplWrapper) RemoveContainer(ctx context.Context, req *RemoveContainerRequest) (*RemoveContainerResponse, error) {
	event := Event_REMOVE_CONTAINER
	o.p.deprecated[event] = true
	_, err := o.PluginService.StateChange(ctx, &StateChangeEvent{
		Event:     event,
		Pod:       req.GetPod(),
		Container: req.GetContainer(),
	})
	return &api.RemoveContainerResponse{}, err
}
