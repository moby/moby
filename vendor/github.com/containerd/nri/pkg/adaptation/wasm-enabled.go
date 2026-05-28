//go:build !nri_no_wasm

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
	"fmt"

	"github.com/containerd/nri/pkg/api"
	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/imports/wasi_snapshot_preview1"
)

func getWasmService() (*api.PluginPlugin, error) {
	wasmWithCloseOnContextDone := func(ctx context.Context) (wazero.Runtime, error) {
		var (
			cfg = wazero.NewRuntimeConfig().WithCloseOnContextDone(true)
			r   = wazero.NewRuntimeWithConfig(ctx, cfg)
		)
		if _, err := wasi_snapshot_preview1.Instantiate(ctx, r); err != nil {
			return nil, err
		}
		return r, nil
	}

	wasmPlugins, err := api.NewPluginPlugin(
		context.Background(),
		api.WazeroRuntime(wasmWithCloseOnContextDone),
	)
	if err != nil {
		return nil, fmt.Errorf("unable to initialize WASM service: %w", err)
	}

	return wasmPlugins, nil
}
