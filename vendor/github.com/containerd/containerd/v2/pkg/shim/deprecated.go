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
	"fmt"
	"io"
	"os"

	bootapi "github.com/containerd/containerd/api/runtime/bootstrap/v1"
	"github.com/containerd/containerd/api/types"
	"github.com/containerd/log"
)

// StartOpts describes shim start configuration received from containerd.
//
// Deprecated: Use [bootapi.BootstrapParams] instead.
type StartOpts struct {
	Address      string
	TTRPCAddress string
	Debug        bool
}

// BootstrapParams is a JSON payload returned in stdout from shim.Start call.
//
// Deprecated: Use [bootapi.BootstrapResult] instead.
type BootstrapParams struct {
	// Version is the version of shim parameters (expected 2 for shim v2)
	Version int `json:"version"`
	// Address is a address containerd should use to connect to shim.
	Address string `json:"address"`
	// Protocol is either TTRPC or GRPC.
	Protocol string `json:"protocol"`
}

// Manager is the interface which manages the shim process.
//
// Deprecated: Use [Shim] instead.
type Manager interface {
	Name() string
	Start(ctx context.Context, id string, opts StartOpts) (BootstrapParams, error)
	Stop(ctx context.Context, id string) (StopStatus, error)
	Info(ctx context.Context, optionsR io.Reader) (*types.RuntimeInfo, error)
}

// managerShim wraps a deprecated Manager to implement the Shim interface.
type managerShim struct {
	manager Manager
}

func (m *managerShim) Name() string {
	return m.manager.Name()
}

func (m *managerShim) Start(ctx context.Context, params *bootapi.BootstrapParams) (*bootapi.BootstrapResult, error) {
	opts := StartOpts{
		Address:      params.ContainerdGrpcAddress,
		TTRPCAddress: params.ContainerdTtrpcAddress,
		Debug:        params.LogLevel <= bootapi.LogLevel_LOG_LEVEL_DEBUG,
	}

	bp, err := m.manager.Start(ctx, params.InstanceID, opts)
	if err != nil {
		return nil, err
	}

	return &bootapi.BootstrapResult{
		Version:  int32(bp.Version),
		Address:  bp.Address,
		Protocol: bp.Protocol,
	}, nil
}

func (m *managerShim) Stop(ctx context.Context, id string) (StopStatus, error) {
	return m.manager.Stop(ctx, id)
}

func (m *managerShim) Info(ctx context.Context, optionsR io.Reader) (*types.RuntimeInfo, error) {
	return m.manager.Info(ctx, optionsR)
}

// Run initializes and runs a shim server.
//
// Deprecated: Use [RunShim] instead.
func Run(ctx context.Context, manager Manager, opts ...BinaryOpts) {
	var config Config
	for _, o := range opts {
		o(&config)
	}

	shim := &managerShim{manager: manager}
	ctx = log.WithLogger(ctx, log.G(ctx).WithField("runtime", shim.Name()))

	if err := run(ctx, shim, config); err != nil {
		fmt.Fprintf(os.Stderr, "%s: %s", shim.Name(), err)
		os.Exit(1)
	}
}
