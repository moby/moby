//go:build !windows

package daemon

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/containerd/containerd/plugin"
	v2runcoptions "github.com/containerd/containerd/runtime/v2/runc/options"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/daemon/config"
	"github.com/docker/docker/errdefs"
)

func TestInitRuntimes_InvalidConfigs(t *testing.T) {
	cases := []struct {
		name      string
		runtime   types.Runtime
		expectErr string
	}{
		{
			name:      "Empty",
			expectErr: "either a runtimeType or a path must be configured",
		},
		{
			name:      "ArgsOnly",
			runtime:   types.Runtime{Args: []string{"foo", "bar"}},
			expectErr: "either a runtimeType or a path must be configured",
		},
		{
			name:      "OptionsOnly",
			runtime:   types.Runtime{Options: map[string]interface{}{"hello": "world"}},
			expectErr: "either a runtimeType or a path must be configured",
		},
		{
			name:      "PathAndType",
			runtime:   types.Runtime{Path: "/bin/true", Type: "io.containerd.runsc.v1"},
			expectErr: "cannot configure both",
		},
		{
			name:      "PathAndOptions",
			runtime:   types.Runtime{Path: "/bin/true", Options: map[string]interface{}{"a": "b"}},
			expectErr: "options cannot be used with a path runtime",
		},
		{
			name:      "TypeAndArgs",
			runtime:   types.Runtime{Type: "io.containerd.runsc.v1", Args: []string{"--version"}},
			expectErr: "args cannot be used with a runtimeType runtime",
		},
		{
			name: "PathArgsOptions",
			runtime: types.Runtime{
				Path:    "/bin/true",
				Args:    []string{"--version"},
				Options: map[string]interface{}{"hmm": 3},
			},
			expectErr: "options cannot be used with a path runtime",
		},
		{
			name: "TypeOptionsArgs",
			runtime: types.Runtime{
				Type:    "io.containerd.kata.v2",
				Options: map[string]interface{}{"a": "b"},
				Args:    []string{"--help"},
			},
			expectErr: "args cannot be used with a runtimeType runtime",
		},
		{
			name: "PathArgsTypeOptions",
			runtime: types.Runtime{
				Path:    "/bin/true",
				Args:    []string{"foo"},
				Type:    "io.containerd.runsc.v1",
				Options: map[string]interface{}{"a": "b"},
			},
			expectErr: "cannot configure both",
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			cfg, err := config.New()
			assert.NilError(t, err)
			d := &Daemon{configStore: cfg}
			d.configStore.Root = t.TempDir()
			assert.Assert(t, os.Mkdir(filepath.Join(d.configStore.Root, "runtimes"), 0700))

			err = d.initRuntimes(map[string]types.Runtime{"myruntime": tt.runtime})
			assert.Check(t, is.ErrorContains(err, tt.expectErr))
		})
	}
}

func TestGetRuntime(t *testing.T) {
	// Configured runtimes can have any arbitrary name, including names
	// which would not be allowed as implicit runtime names. Explicit takes
	// precedence over implicit.
	const configuredRtName = "my/custom.runtime.v1"
	configuredRuntime := types.Runtime{Path: "/bin/true"}

	const rtWithArgsName = "withargs"
	rtWithArgs := types.Runtime{
		Path: "/bin/false",
		Args: []string{"--version"},
	}

	const shimWithOptsName = "shimwithopts"
	shimWithOpts := types.Runtime{
		Type:    plugin.RuntimeRuncV2,
		Options: map[string]interface{}{"IoUid": 42},
	}

	const shimAliasName = "wasmedge"
	shimAlias := types.Runtime{Type: "io.containerd.wasmedge.v1"}

	const configuredShimByPathName = "shimwithpath"
	configuredShimByPath := types.Runtime{Type: "/path/to/my/shim"}

	cfg, err := config.New()
	assert.NilError(t, err)

	d := &Daemon{configStore: cfg}
	d.configStore.Root = t.TempDir()
	assert.Assert(t, os.Mkdir(filepath.Join(d.configStore.Root, "runtimes"), 0700))
	d.configStore.Runtimes = map[string]types.Runtime{
		configuredRtName:         configuredRuntime,
		rtWithArgsName:           rtWithArgs,
		shimWithOptsName:         shimWithOpts,
		shimAliasName:            shimAlias,
		configuredShimByPathName: configuredShimByPath,
	}
	configureRuntimes(d.configStore)
	assert.Assert(t, d.loadRuntimes())

	stockRuntime, ok := d.configStore.Runtimes[config.StockRuntimeName]
	assert.Assert(t, ok, "stock runtime could not be found (test needs to be updated)")

	configdOpts := *stockRuntime.ShimConfig.Opts.(*v2runcoptions.Options)
	configdOpts.BinaryName = configuredRuntime.Path

	for _, tt := range []struct {
		name, runtime string
		wantShim      string
		wantOpts      interface{}
	}{
		{
			name:     "StockRuntime",
			runtime:  config.StockRuntimeName,
			wantShim: stockRuntime.ShimConfig.Binary,
			wantOpts: stockRuntime.ShimConfig.Opts,
		},
		{
			name:     "ShimName",
			runtime:  "io.containerd.my-shim.v42",
			wantShim: "io.containerd.my-shim.v42",
		},
		{
			// containerd is pretty loose about the format of runtime names. Perhaps too
			// loose. The only requirements are that the name contain a dot and (depending
			// on the containerd version) not start with a dot. It does not enforce any
			// particular format of the dot-delimited components of the name.
			name:     "VersionlessShimName",
			runtime:  "io.containerd.my-shim",
			wantShim: "io.containerd.my-shim",
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
			name:     "ConfiguredRuntime",
			runtime:  configuredRtName,
			wantShim: stockRuntime.ShimConfig.Binary,
			wantOpts: &configdOpts,
		},
		{
			name:     "RuntimeWithArgs",
			runtime:  rtWithArgsName,
			wantShim: stockRuntime.ShimConfig.Binary,
			wantOpts: defaultV2ShimConfig(
				d.configStore,
				d.rewriteRuntimePath(
					rtWithArgsName,
					rtWithArgs.Path,
					rtWithArgs.Args)).Opts,
		},
		{
			name:     "ShimWithOpts",
			runtime:  shimWithOptsName,
			wantShim: shimWithOpts.Type,
			wantOpts: &v2runcoptions.Options{IoUid: 42},
		},
		{
			name:     "ShimAlias",
			runtime:  shimAliasName,
			wantShim: shimAlias.Type,
		},
		{
			name:     "ConfiguredShimByPath",
			runtime:  configuredShimByPathName,
			wantShim: configuredShimByPath.Type,
		},
	} {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			gotShim, gotOpts, err := d.getRuntime(tt.runtime)
			assert.Check(t, is.Equal(gotShim, tt.wantShim))
			assert.Check(t, is.DeepEqual(gotOpts, tt.wantOpts))
			if tt.wantShim != "" {
				assert.Check(t, err)
			} else {
				assert.Check(t, errdefs.IsInvalidParameter(err))
			}
		})
	}
}
