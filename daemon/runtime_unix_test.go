//go:build !windows
// +build !windows

package daemon

import (
	"os"
	"path/filepath"
	"testing"

	v2runcoptions "github.com/containerd/containerd/runtime/v2/runc/options"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/daemon/config"
	"github.com/docker/docker/errdefs"
)

func TestGetRuntime(t *testing.T) {
	// Configured runtimes can have any arbitrary name, including names
	// which would not be allowed as implicit runtime names. Explicit takes
	// precedence over implicit.
	const configuredRtName = "my/custom.shim.v1"
	configuredRuntime := types.Runtime{Path: "/bin/true"}

	d := &Daemon{configStore: config.New()}
	d.configStore.Root = t.TempDir()
	assert.Assert(t, os.Mkdir(filepath.Join(d.configStore.Root, "runtimes"), 0700))
	d.configStore.Runtimes = map[string]types.Runtime{
		configuredRtName: configuredRuntime,
	}
	configureRuntimes(d.configStore)
	assert.Assert(t, d.loadRuntimes())

	stockRuntime, ok := d.configStore.Runtimes[config.StockRuntimeName]
	assert.Assert(t, ok, "stock runtime could not be found (test needs to be updated)")

	configdOpts := *stockRuntime.Shim.Opts.(*v2runcoptions.Options)
	configdOpts.BinaryName = configuredRuntime.Path
	wantConfigdRuntime := configuredRuntime
	wantConfigdRuntime.Shim = &types.ShimConfig{
		Binary: stockRuntime.Shim.Binary,
		Opts:   &configdOpts,
	}

	for _, tt := range []struct {
		name, runtime string
		want          *types.Runtime
	}{
		{
			name:    "StockRuntime",
			runtime: config.StockRuntimeName,
			want:    &stockRuntime,
		},
		{
			name:    "ShimName",
			runtime: "io.containerd.my-shim.v42",
			want:    &types.Runtime{Shim: &types.ShimConfig{Binary: "io.containerd.my-shim.v42"}},
		},
		{
			// containerd is pretty loose about the format of runtime names. Perhaps too
			// loose. The only requirements are that the name contain a dot and (depending
			// on the containerd version) not start with a dot. It does not enforce any
			// particular format of the dot-delimited components of the name.
			name:    "VersionlessShimName",
			runtime: "io.containerd.my-shim",
			want:    &types.Runtime{Shim: &types.ShimConfig{Binary: "io.containerd.my-shim"}},
		},
		{
			name:    "IllformedShimName",
			runtime: "myshim",
		},
		{
			name:    "EmptyString",
			runtime: "",
		},
		{
			name:    "PathToShim",
			runtime: "/path/to/runc",
		},
		{
			name:    "PathToShimName",
			runtime: "/path/to/io.containerd.runc.v2",
		},
		{
			name:    "RelPathToShim",
			runtime: "my/io.containerd.runc.v2",
		},
		{
			name:    "ConfiguredRuntime",
			runtime: configuredRtName,
			want:    &wantConfigdRuntime,
		},
	} {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			got, err := d.getRuntime(tt.runtime)
			assert.Check(t, is.DeepEqual(got, tt.want))
			if tt.want != nil {
				assert.Check(t, err)
			} else {
				assert.Check(t, errdefs.IsInvalidParameter(err))
			}
		})
	}
}
