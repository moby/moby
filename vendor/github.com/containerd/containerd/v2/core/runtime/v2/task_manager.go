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
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	goruntime "runtime"
	"slices"
	"strings"

	"github.com/containerd/errdefs"
	"github.com/containerd/log"
	"github.com/containerd/platforms"
	"github.com/containerd/plugin"
	"github.com/containerd/plugin/registry"
	"github.com/containerd/typeurl/v2"
	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/opencontainers/runtime-spec/specs-go/features"

	bootapi "github.com/containerd/containerd/api/runtime/bootstrap/v1"
	apitypes "github.com/containerd/containerd/api/types"

	"github.com/containerd/containerd/v2/core/mount"
	"github.com/containerd/containerd/v2/core/runtime"
	"github.com/containerd/containerd/v2/pkg/protobuf/proto"
	"github.com/containerd/containerd/v2/pkg/timeout"
	"github.com/containerd/containerd/v2/plugins"
	"github.com/containerd/containerd/v2/plugins/services/warning"
)

// TaskConfig for the runtime task manager
type TaskConfig struct {
	// Supported platforms
	Platforms []string `toml:"platforms"`
}

func init() {
	registry.Register(&plugin.Registration{
		Type: plugins.RuntimePluginV2,
		ID:   "task",
		Requires: []plugin.Type{
			plugins.ShimPlugin,
			plugins.MountManagerPlugin,
			plugins.WarningPlugin,
		},
		Config: &TaskConfig{
			Platforms: defaultPlatforms(),
		},
		InitFn: func(ic *plugin.InitContext) (any, error) {
			config := ic.Config.(*TaskConfig)

			supportedPlatforms, err := platforms.ParseAll(config.Platforms)
			if err != nil {
				return nil, err
			}
			ic.Meta.Platforms = supportedPlatforms
			if ic.Meta.Exports == nil {
				ic.Meta.Exports = make(map[string]string, 1)
			}
			ic.Meta.Exports["log-uri-schemes"] = strings.Join(supportedLogURISchemes(), ",")

			shimManagerI, err := ic.GetSingle(plugins.ShimPlugin)
			if err != nil {
				return nil, err
			}
			shimManager := shimManagerI.(*ShimManager)

			var mounts mount.Manager
			if mountsI, err := ic.GetSingle(plugins.MountManagerPlugin); err == nil {
				mounts = mountsI.(mount.Manager)
			} else if !errors.Is(err, plugin.ErrPluginNotFound) {
				return nil, err
			}
			root, state := ic.Properties[plugins.PropertyRootDir], ic.Properties[plugins.PropertyStateDir]
			for _, d := range []string{root, state} {
				// root:  the parent of this directory is created as 0o700, not 0o711.
				// state: the parent of this directory is created as 0o711 too, so as to support userns-remapped containers.
				if err := os.MkdirAll(d, 0711); err != nil {
					return nil, err
				}
			}

			if err := shimManager.LoadExistingShims(ic.Context, state, root); err != nil {
				return nil, fmt.Errorf("failed to load existing shims for task manager")
			}

			warningsI, err := ic.GetSingle(plugins.WarningPlugin)
			if err != nil {
				return nil, err
			}
			warnings := warningsI.(warning.Service)
			emitPlatformWarnings(ic.Context, warnings)

			return &TaskManager{
				root:    root,
				state:   state,
				manager: shimManager,
				taskMounts: &taskMountController{
					manager: mounts,
					legacy:  newDeprecatedMountCapabilities(shimManager),
				},
			}, nil
		},
	})
}

// TaskManager wraps task service client on top of shim manager.
type TaskManager struct {
	root       string
	state      string
	manager    *ShimManager
	taskMounts *taskMountController
}

// NewTaskManager creates a new task manager instance.
// root is the rootDir of TaskManager plugin to store persistent data
// state is the stateDir of TaskManager plugin to store transient data
// shims is  ShimManager for TaskManager to create/delete shims
func NewTaskManager(ctx context.Context, root, state string, shims *ShimManager) (*TaskManager, error) {
	if err := shims.LoadExistingShims(ctx, state, root); err != nil {
		return nil, fmt.Errorf("failed to load existing shims for task manager")
	}
	m := &TaskManager{
		root:    root,
		state:   state,
		manager: shims,
		taskMounts: &taskMountController{
			legacy: newDeprecatedMountCapabilities(shims),
		},
	}
	return m, nil
}

// ID of the task manager
func (m *TaskManager) ID() string {
	return plugins.RuntimePluginV2.String() + ".task"
}

// Create launches new shim instance and creates new task
func (m *TaskManager) Create(ctx context.Context, taskID string, opts runtime.CreateOpts) (_ runtime.Task, retErr error) {
	bundle, err := NewBundle(ctx, m.root, m.state, taskID, opts.Spec)
	if err != nil {
		return nil, err
	}
	defer func() {
		if retErr != nil {
			bundle.Delete()
		}
	}()

	log.G(ctx).WithFields(log.Fields{
		"id":      taskID,
		"runtime": opts.Runtime,
	}).Debug("creating task")

	// Registered before the shim is started so that it runs after the shim
	// cleanup below: the shim may still be using these mounts.
	var activation mountActivation
	defer func() {
		if retErr != nil && activation.owned {
			dctx, cancel := timeout.WithContext(context.WithoutCancel(ctx), cleanupTimeout)
			defer cancel()
			if err := m.taskMounts.Deactivate(dctx, taskID); err != nil {
				log.G(ctx).WithError(err).WithField("task", taskID).Errorf("failed to deactivate mounts")
			}
		}
	}()

	// The shim is started before its mounts are activated so that it can report
	// which mount types and transforms it performs itself, which decides what
	// the mount manager must do on its behalf. Starting the shim does not
	// require the rootfs; only the task.Create call below consumes opts.Rootfs.
	shim, err := m.manager.Start(ctx, taskID, bundle, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to start shim: %w", err)
	}
	defer func() {
		if retErr != nil {
			m.cleanupStartedShim(ctx, taskID, shim)
		}
	}()

	var bootstrap *bootapi.BootstrapResult
	if sc, ok := shim.(shimCapabilities); ok {
		bootstrap = sc.BootstrapResult()
	}
	activation, err = m.taskMounts.Activate(ctx, taskID, opts.Runtime, bootstrap, opts.Rootfs)
	if err != nil {
		return nil, err
	}
	opts.Rootfs = activation.rootfs

	// Cast to shim task and call task service to create a new container task instance.
	// This will not be required once shim service / client implemented.
	shimTask, err := newShimTask(shim)
	if err != nil {
		return nil, err
	}

	// runc ignores silently features it doesn't know about, so for things that this is
	// problematic let's check if this runc version supports them.
	if err := m.validateRuntimeFeatures(ctx, opts); err != nil {
		return nil, fmt.Errorf("failed to validate OCI runtime features: %w", err)
	}

	t, err := func() (runtime.Task, error) {
		t, err := shimTask.Create(ctx, opts)
		if err == nil || !errdefs.IsNotImplemented(err) {
			return t, err
		}

		downgrader, ok := shim.(clientVersionDowngrader)
		if ok {
			if derr := downgrader.Downgrade(); derr == nil {
				log.G(ctx).WithError(err).WithField("id", taskID).
					Warning("failed to call task.Create, downgrading client API version to try again")

				shimTask, err = newShimTask(shim)
				if err != nil {
					return nil, fmt.Errorf("failed to create shim task after downgrading: %w", err)
				}
				return shimTask.Create(ctx, opts)
			}
		}
		return t, err
	}()
	if err != nil {
		// The shim is torn down, including removing it from m.manager.shims,
		// by the deferred cleanupStartedShim above.
		return nil, fmt.Errorf("failed to create shim task: %w", err)
	}

	return t, nil
}

// cleanupStartedShim tears down a shim that was started for a task which then
// failed to be created. It may be called before a *shimTask exists for shim,
// since it also covers the window between a successful shim start and
// taskMounts.Activate/newShimTask succeeding.
func (m *TaskManager) cleanupStartedShim(ctx context.Context, taskID string, shim ShimInstance) {
	// NOTE: ctx contains required namespace information.
	m.manager.shims.Delete(ctx, taskID)

	shimTask, err := newShimTask(shim)
	if err != nil {
		log.G(ctx).WithError(err).WithField("id", taskID).
			Error("failed to create shim task to clean up shim")
		shim.Close()
		return
	}

	if err := cleanupShimTask(ctx, shimTask); err != nil && !errdefs.IsNotFound(err) {
		log.G(ctx).WithError(err).WithField("id", taskID).Error("failed to clean up shim")
	}
}

// Get a specific task
func (m *TaskManager) Get(ctx context.Context, id string) (runtime.Task, error) {
	shim, err := m.manager.shims.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	return newShimTask(shim)
}

// Tasks lists all tasks
func (m *TaskManager) Tasks(ctx context.Context, all bool) ([]runtime.Task, error) {
	shims, err := m.manager.shims.GetAll(ctx, all)
	if err != nil {
		return nil, err
	}
	out := make([]runtime.Task, len(shims))
	for i := range shims {
		newClient, err := newShimTask(shims[i])
		if err != nil {
			return nil, err
		}
		out[i] = newClient
	}
	return out, nil
}

// Delete deletes the task and shim instance
func (m *TaskManager) Delete(ctx context.Context, taskID string) (*runtime.Exit, error) {
	shim, err := m.manager.shims.Get(ctx, taskID)
	if err != nil {
		return nil, err
	}

	_, err = m.manager.containers.Get(ctx, taskID)
	if err != nil {
		return nil, err
	}

	shimTask, err := newShimTask(shim)
	if err != nil {
		return nil, err
	}

	exit, err := shimTask.delete(ctx, func(ctx context.Context, id string) {
		m.manager.shims.Delete(ctx, id)
	})

	// An ErrNotFound here means the shim has no record of the task and there
	// was no cached delete result to fall back to. For example, the task was
	// never created successfully in the shim, or a previous containerd process
	// deleted it. The runtime side has still been cleaned up, so we should
	// deactivate the mounts before returning the error.
	if err != nil && !errdefs.IsNotFound(err) {
		return nil, fmt.Errorf("failed to delete task: %w", err)
	}

	// FIXME(fuweid): It seems that cleaning this up is best-effort because
	// GC can guarantee that the mount is deleted when the container is deleted.
	// What if we reuse the container and restart the task?
	if merr := m.taskMounts.Deactivate(ctx, taskID); merr != nil && !errdefs.IsNotFound(merr) {
		log.G(ctx).WithError(merr).WithField("task", taskID).Errorf("failed to deactivate mounts")
	}

	if err != nil {
		return nil, fmt.Errorf("failed to delete task: %w", err)
	}
	return exit, nil
}

func supportedLogURISchemes() []string {
	switch goruntime.GOOS {
	case "windows":
		return []string{"binary", "binary-v2", "file", "npipe"}
	default:
		return []string{"fifo", "binary", "binary-v2", "file"}
	}
}

func getRuntimeInfo(ctx context.Context, shims *ShimManager, req *apitypes.RuntimeRequest) (*apitypes.RuntimeInfo, error) {
	runtimePath, err := shims.resolveRuntimePath(req.RuntimePath)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve runtime path: %w", err)
	}
	var optsB []byte
	if req.Options != nil {
		optsB, err = proto.Marshal(req.Options)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal %s: %w", req.Options.TypeUrl, err)
		}
	}
	var stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, runtimePath, "-info")
	cmd.Stdin = bytes.NewReader(optsB)
	cmd.Stderr = &stderr
	stdout, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to run %v: %w (stderr: %q)", cmd.Args, err, stderr.String())
	}
	var info apitypes.RuntimeInfo
	if err = proto.Unmarshal(stdout, &info); err != nil {
		return nil, fmt.Errorf("failed to unmarshal stdout from %v into %T: %w", cmd.Args, &info, err)
	}
	return &info, nil
}

func (m *TaskManager) PluginInfo(ctx context.Context, request any) (any, error) {
	req, ok := request.(*apitypes.RuntimeRequest)
	if !ok {
		return nil, fmt.Errorf("unknown request type %T: %w", request, errdefs.ErrNotImplemented)
	}

	return getRuntimeInfo(ctx, m.manager, req)
}

func (m *TaskManager) validateRuntimeFeatures(ctx context.Context, opts runtime.CreateOpts) error {
	var spec specs.Spec
	if err := typeurl.UnmarshalTo(opts.Spec, &spec); err != nil {
		return fmt.Errorf("unmarshal spec: %w", err)
	}

	// Only ask for the PluginInfo if idmap mounts are used.
	if !usesIDMapMounts(spec) {
		return nil
	}

	topts := opts.TaskOptions
	if topts == nil || topts.GetValue() == nil {
		topts = opts.RuntimeOptions
	}

	pInfo, err := m.PluginInfo(ctx, &apitypes.RuntimeRequest{RuntimePath: opts.Runtime, Options: typeurl.MarshalProto(topts)})
	if err != nil {
		return fmt.Errorf("runtime info: %w", err)
	}

	pluginInfo, ok := pInfo.(*apitypes.RuntimeInfo)
	if !ok {
		return fmt.Errorf("invalid runtime info type: %T", pInfo)
	}

	feat, err := typeurl.UnmarshalAny(pluginInfo.Features)
	if err != nil {
		return fmt.Errorf("unmarshal runtime features: %w", err)
	}

	// runc-compatible runtimes silently ignores features it doesn't know about. But ignoring
	// our request to use idmap mounts can break permissions in the volume, so let's make sure
	// it supports it. For more info, see:
	//	https://github.com/opencontainers/runtime-spec/pull/1219
	//
	features, ok := feat.(*features.Features)
	if !ok {
		// Leave alone non runc-compatible runtimes that don't provide the features info,
		// they might not be affected by this.
		return nil
	}

	if err := supportsIDMapMounts(features); err != nil {
		return fmt.Errorf("idmap mounts not supported: %w", err)
	}

	return nil
}

func usesIDMapMounts(spec specs.Spec) bool {
	for _, m := range spec.Mounts {
		if m.UIDMappings != nil || m.GIDMappings != nil {
			return true
		}
		if slices.Contains(m.Options, "idmap") || slices.Contains(m.Options, "ridmap") {
			return true
		}

	}
	return false
}

func supportsIDMapMounts(features *features.Features) error {
	if features.Linux.MountExtensions == nil || features.Linux.MountExtensions.IDMap == nil {
		return errors.New("missing `mountExtensions.idmap` entry in `features` command")
	}
	if enabled := features.Linux.MountExtensions.IDMap.Enabled; enabled == nil || !*enabled {
		return errors.New("entry `mountExtensions.idmap.Enabled` in `features` command not present or disabled")
	}
	return nil
}
