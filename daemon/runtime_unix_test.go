//go:build !windows

package daemon

import (
	"io/fs"
	"os"
	"strings"
	"testing"

	runtimeoptions_v1 "github.com/containerd/containerd/pkg/runtimeoptions/v1"
	"github.com/containerd/containerd/plugin"
	v2runcoptions "github.com/containerd/containerd/runtime/v2/runc/options"
	"github.com/docker/docker/api/types/system"
	"github.com/docker/docker/daemon/config"
	"github.com/docker/docker/errdefs"
	"github.com/imdario/mergo"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestSetupRuntimes(t *testing.T) {
	cases := []struct {
		name      string
		config    *config.Config
		expectErr string
	}{
		{
			name: "Empty",
			config: &config.Config{
				Runtimes: map[string]system.Runtime{
					"myruntime": {},
				},
			},
			expectErr: "either a runtimeType or a path must be configured",
		},
		{
			name: "ArgsOnly",
			config: &config.Config{
				Runtimes: map[string]system.Runtime{
					"myruntime": {Args: []string{"foo", "bar"}},
				},
			},
			expectErr: "either a runtimeType or a path must be configured",
		},
		{
			name: "OptionsOnly",
			config: &config.Config{
				Runtimes: map[string]system.Runtime{
					"myruntime": {Options: map[string]interface{}{"hello": "world"}},
				},
			},
			expectErr: "either a runtimeType or a path must be configured",
		},
		{
			name: "PathAndType",
			config: &config.Config{
				Runtimes: map[string]system.Runtime{
					"myruntime": {Path: "/bin/true", Type: "io.containerd.runsc.v1"},
				},
			},
			expectErr: "cannot configure both",
		},
		{
			name: "PathAndOptions",
			config: &config.Config{
				Runtimes: map[string]system.Runtime{
					"myruntime": {Path: "/bin/true", Options: map[string]interface{}{"a": "b"}},
				},
			},
			expectErr: "options cannot be used with a path runtime",
		},
		{
			name: "TypeAndArgs",
			config: &config.Config{
				Runtimes: map[string]system.Runtime{
					"myruntime": {Type: "io.containerd.runsc.v1", Args: []string{"--version"}},
				},
			},
			expectErr: "args cannot be used with a runtimeType runtime",
		},
		{
			name: "PathArgsOptions",
			config: &config.Config{
				Runtimes: map[string]system.Runtime{
					"myruntime": {
						Path:    "/bin/true",
						Args:    []string{"--version"},
						Options: map[string]interface{}{"hmm": 3},
					},
				},
			},
			expectErr: "options cannot be used with a path runtime",
		},
		{
			name: "TypeOptionsArgs",
			config: &config.Config{
				Runtimes: map[string]system.Runtime{
					"myruntime": {
						Type:    "io.containerd.kata.v2",
						Options: map[string]interface{}{"a": "b"},
						Args:    []string{"--help"},
					},
				},
			},
			expectErr: "args cannot be used with a runtimeType runtime",
		},
		{
			name: "PathArgsTypeOptions",
			config: &config.Config{
				Runtimes: map[string]system.Runtime{
					"myruntime": {
						Path:    "/bin/true",
						Args:    []string{"foo"},
						Type:    "io.containerd.runsc.v1",
						Options: map[string]interface{}{"a": "b"},
					},
				},
			},
			expectErr: "cannot configure both",
		},
		{
			name: "CannotOverrideStockRuntime",
			config: &config.Config{
				Runtimes: map[string]system.Runtime{
					config.StockRuntimeName: {},
				},
			},
			expectErr: `runtime name 'runc' is reserved`,
		},
		{
			name: "SetStockRuntimeAsDefault",
			config: &config.Config{
				CommonConfig: config.CommonConfig{
					DefaultRuntime: config.StockRuntimeName,
				},
			},
		},
		{
			name: "SetLinuxRuntimeAsDefault",
			config: &config.Config{
				CommonConfig: config.CommonConfig{
					DefaultRuntime: linuxV2RuntimeName,
				},
			},
		},
		{
			name: "CannotSetBogusRuntimeAsDefault",
			config: &config.Config{
				CommonConfig: config.CommonConfig{
					DefaultRuntime: "notdefined",
				},
			},
			expectErr: "specified default runtime 'notdefined' does not exist",
		},
		{
			name: "SetDefinedRuntimeAsDefault",
			config: &config.Config{
				Runtimes: map[string]system.Runtime{
					"some-runtime": {
						Path: "/usr/local/bin/file-not-found",
					},
				},
				CommonConfig: config.CommonConfig{
					DefaultRuntime: "some-runtime",
				},
			},
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			cfg, err := config.New()
			assert.NilError(t, err)
			cfg.Root = t.TempDir()
			assert.NilError(t, mergo.Merge(cfg, tc.config, mergo.WithOverride))
			assert.Assert(t, initRuntimesDir(cfg))

			_, err = setupRuntimes(cfg)
			if tc.expectErr == "" {
				assert.NilError(t, err)
			} else {
				assert.ErrorContains(t, err, tc.expectErr)
			}
		})
	}
}

func TestGetRuntime(t *testing.T) {
	// Configured runtimes can have any arbitrary name, including names
	// which would not be allowed as implicit runtime names. Explicit takes
	// precedence over implicit.
	const configuredRtName = "my/custom.runtime.v1"
	configuredRuntime := system.Runtime{Path: "/bin/true"}

	const rtWithArgsName = "withargs"
	rtWithArgs := system.Runtime{
		Path: "/bin/false",
		Args: []string{"--version"},
	}

	const shimWithOptsName = "shimwithopts"
	shimWithOpts := system.Runtime{
		Type:    plugin.RuntimeRuncV2,
		Options: map[string]interface{}{"IoUid": 42},
	}

	const shimAliasName = "wasmedge"
	shimAlias := system.Runtime{Type: "io.containerd.wasmedge.v1"}

	const configuredShimByPathName = "shimwithpath"
	configuredShimByPath := system.Runtime{Type: "/path/to/my/shim"}

	// A runtime configured with the generic 'runtimeoptions/v1.Options' shim configuration options.
	// https://gvisor.dev/docs/user_guide/containerd/configuration/#:~:text=to%20the%20shim.-,Containerd%201.3%2B,-Starting%20in%201.3
	const gvisorName = "gvisor"
	gvisorRuntime := system.Runtime{
		Type: "io.containerd.runsc.v1",
		Options: map[string]interface{}{
			"TypeUrl":    "io.containerd.runsc.v1.options",
			"ConfigPath": "/path/to/runsc.toml",
		},
	}

	cfg, err := config.New()
	assert.NilError(t, err)

	cfg.Root = t.TempDir()
	cfg.Runtimes = map[string]system.Runtime{
		configuredRtName:         configuredRuntime,
		rtWithArgsName:           rtWithArgs,
		shimWithOptsName:         shimWithOpts,
		shimAliasName:            shimAlias,
		configuredShimByPathName: configuredShimByPath,
		gvisorName:               gvisorRuntime,
	}
	assert.NilError(t, initRuntimesDir(cfg))
	runtimes, err := setupRuntimes(cfg)
	assert.NilError(t, err)

	stockRuntime, ok := runtimes.configured[config.StockRuntimeName]
	assert.Assert(t, ok, "stock runtime could not be found (test needs to be updated)")
	stockRuntime.Features = nil

	configdOpts := *stockRuntime.Opts.(*v2runcoptions.Options)
	configdOpts.BinaryName = configuredRuntime.Path
	wantConfigdRuntime := &shimConfig{
		Shim: stockRuntime.Shim,
		Opts: &configdOpts,
	}

	for _, tt := range []struct {
		name, runtime string
		want          *shimConfig
	}{
		{
			name:    "StockRuntime",
			runtime: config.StockRuntimeName,
			want:    stockRuntime,
		},
		{
			name:    "ShimName",
			runtime: "io.containerd.my-shim.v42",
			want:    &shimConfig{Shim: "io.containerd.my-shim.v42"},
		},
		{
			// containerd is pretty loose about the format of runtime names. Perhaps too
			// loose. The only requirements are that the name contain a dot and (depending
			// on the containerd version) not start with a dot. It does not enforce any
			// particular format of the dot-delimited components of the name.
			name:    "VersionlessShimName",
			runtime: "io.containerd.my-shim",
			want:    &shimConfig{Shim: "io.containerd.my-shim"},
		},
		{
			name:    "IllformedShimName",
			runtime: "myshim",
		},
		{
			name:    "EmptyString",
			runtime: "",
			want:    stockRuntime,
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
			want:    wantConfigdRuntime,
		},
		{
			name:    "ShimWithOpts",
			runtime: shimWithOptsName,
			want: &shimConfig{
				Shim: shimWithOpts.Type,
				Opts: &v2runcoptions.Options{IoUid: 42},
			},
		},
		{
			name:    "ShimAlias",
			runtime: shimAliasName,
			want:    &shimConfig{Shim: shimAlias.Type},
		},
		{
			name:    "ConfiguredShimByPath",
			runtime: configuredShimByPathName,
			want:    &shimConfig{Shim: configuredShimByPath.Type},
		},
		{
			name:    "ConfiguredShimWithRuntimeoptionsShimConfig",
			runtime: gvisorName,
			want: &shimConfig{
				Shim: gvisorRuntime.Type,
				Opts: &runtimeoptions_v1.Options{
					TypeUrl:    gvisorRuntime.Options["TypeUrl"].(string),
					ConfigPath: gvisorRuntime.Options["ConfigPath"].(string),
				},
			},
		},
	} {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			shim, opts, err := runtimes.Get(tt.runtime)
			if tt.want != nil {
				assert.Check(t, err)
				got := &shimConfig{Shim: shim, Opts: opts}
				assert.Check(t, is.DeepEqual(got, tt.want))
			} else {
				assert.Check(t, is.Equal(shim, ""))
				assert.Check(t, is.Nil(opts))
				assert.Check(t, errdefs.IsInvalidParameter(err), "[%T] %[1]v", err)
			}
		})
	}
	t.Run("RuntimeWithArgs", func(t *testing.T) {
		shim, opts, err := runtimes.Get(rtWithArgsName)
		assert.Check(t, err)
		assert.Check(t, is.Equal(shim, stockRuntime.Shim))
		runcopts, ok := opts.(*v2runcoptions.Options)
		if assert.Check(t, ok, "runtimes.Get() opts = type %T, want *v2runcoptions.Options", opts) {
			wrapper, err := os.ReadFile(runcopts.BinaryName)
			if assert.Check(t, err) {
				assert.Check(t, is.Contains(string(wrapper),
					strings.Join(append([]string{rtWithArgs.Path}, rtWithArgs.Args...), " ")))
			}
		}
	})
}

func TestGetRuntime_PreflightCheck(t *testing.T) {
	cfg, err := config.New()
	assert.NilError(t, err)

	cfg.Root = t.TempDir()
	cfg.Runtimes = map[string]system.Runtime{
		"path-only": {
			Path: "/usr/local/bin/file-not-found",
		},
		"with-args": {
			Path: "/usr/local/bin/file-not-found",
			Args: []string{"--arg"},
		},
	}
	assert.NilError(t, initRuntimesDir(cfg))
	runtimes, err := setupRuntimes(cfg)
	assert.NilError(t, err, "runtime paths should not be validated during setupRuntimes()")

	t.Run("PathOnly", func(t *testing.T) {
		_, _, err := runtimes.Get("path-only")
		assert.NilError(t, err, "custom runtimes without wrapper scripts should not have pre-flight checks")
	})
	t.Run("WithArgs", func(t *testing.T) {
		_, _, err := runtimes.Get("with-args")
		assert.ErrorIs(t, err, fs.ErrNotExist)
	})
}

// TestRuntimeWrapping checks that reloading runtime config does not delete or
// modify existing wrapper scripts, which could break lifecycle management of
// existing containers.
func TestRuntimeWrapping(t *testing.T) {
	cfg, err := config.New()
	assert.NilError(t, err)
	cfg.Root = t.TempDir()
	cfg.Runtimes = map[string]system.Runtime{
		"change-args": {
			Path: "/bin/true",
			Args: []string{"foo", "bar"},
		},
		"dupe": {
			Path: "/bin/true",
			Args: []string{"foo", "bar"},
		},
		"change-path": {
			Path: "/bin/true",
			Args: []string{"baz"},
		},
		"drop-args": {
			Path: "/bin/true",
			Args: []string{"some", "arguments"},
		},
		"goes-away": {
			Path: "/bin/true",
			Args: []string{"bye"},
		},
	}
	assert.NilError(t, initRuntimesDir(cfg))
	rt, err := setupRuntimes(cfg)
	assert.Check(t, err)

	type WrapperInfo struct{ BinaryName, Content string }
	wrappers := make(map[string]WrapperInfo)
	for name := range cfg.Runtimes {
		_, opts, err := rt.Get(name)
		if assert.Check(t, err, "rt.Get(%q)", name) {
			binary := opts.(*v2runcoptions.Options).BinaryName
			content, err := os.ReadFile(binary)
			assert.Check(t, err, "could not read wrapper script contents for runtime %q", binary)
			wrappers[name] = WrapperInfo{BinaryName: binary, Content: string(content)}
		}
	}

	cfg.Runtimes["change-args"] = system.Runtime{
		Path: cfg.Runtimes["change-args"].Path,
		Args: []string{"baz", "quux"},
	}
	cfg.Runtimes["change-path"] = system.Runtime{
		Path: "/bin/false",
		Args: cfg.Runtimes["change-path"].Args,
	}
	cfg.Runtimes["drop-args"] = system.Runtime{
		Path: cfg.Runtimes["drop-args"].Path,
	}
	delete(cfg.Runtimes, "goes-away")

	_, err = setupRuntimes(cfg)
	assert.Check(t, err)

	for name, info := range wrappers {
		t.Run(name, func(t *testing.T) {
			content, err := os.ReadFile(info.BinaryName)
			assert.NilError(t, err)
			assert.DeepEqual(t, info.Content, string(content))
		})
	}
}
