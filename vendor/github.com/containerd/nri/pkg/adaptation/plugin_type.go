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
)

type pluginType struct {
	wasmImpl    api.Plugin
	ttrpcImpl   api.PluginService
	builtinImpl api.PluginService
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
