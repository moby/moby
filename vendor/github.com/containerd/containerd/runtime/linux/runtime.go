// +build linux

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

package linux

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	"github.com/boltdb/bolt"
	eventstypes "github.com/containerd/containerd/api/events"
	"github.com/containerd/containerd/api/types"
	"github.com/containerd/containerd/containers"
	"github.com/containerd/containerd/errdefs"
	"github.com/containerd/containerd/events/exchange"
	"github.com/containerd/containerd/identifiers"
	"github.com/containerd/containerd/log"
	"github.com/containerd/containerd/metadata"
	"github.com/containerd/containerd/mount"
	"github.com/containerd/containerd/namespaces"
	"github.com/containerd/containerd/platforms"
	"github.com/containerd/containerd/plugin"
	"github.com/containerd/containerd/runtime"
	"github.com/containerd/containerd/runtime/linux/proc"
	"github.com/containerd/containerd/runtime/linux/runctypes"
	shim "github.com/containerd/containerd/runtime/shim/v1"
	runc "github.com/containerd/go-runc"
	"github.com/containerd/typeurl"
	ptypes "github.com/gogo/protobuf/types"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"golang.org/x/sys/unix"
)

var (
	pluginID = fmt.Sprintf("%s.%s", plugin.RuntimePlugin, "linux")
	empty    = &ptypes.Empty{}
)

const (
	configFilename = "config.json"
	defaultRuntime = "runc"
	defaultShim    = "containerd-shim"
)

func init() {
	plugin.Register(&plugin.Registration{
		Type:   plugin.RuntimePlugin,
		ID:     "linux",
		InitFn: New,
		Requires: []plugin.Type{
			plugin.TaskMonitorPlugin,
			plugin.MetadataPlugin,
		},
		Config: &Config{
			Shim:    defaultShim,
			Runtime: defaultRuntime,
		},
	})
}

var _ = (runtime.PlatformRuntime)(&Runtime{})

// Config options for the runtime
type Config struct {
	// Shim is a path or name of binary implementing the Shim GRPC API
	Shim string `toml:"shim"`
	// Runtime is a path or name of an OCI runtime used by the shim
	Runtime string `toml:"runtime"`
	// RuntimeRoot is the path that shall be used by the OCI runtime for its data
	RuntimeRoot string `toml:"runtime_root"`
	// NoShim calls runc directly from within the pkg
	NoShim bool `toml:"no_shim"`
	// Debug enable debug on the shim
	ShimDebug bool `toml:"shim_debug"`
}

// New returns a configured runtime
func New(ic *plugin.InitContext) (interface{}, error) {
	ic.Meta.Platforms = []ocispec.Platform{platforms.DefaultSpec()}

	if err := os.MkdirAll(ic.Root, 0711); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(ic.State, 0711); err != nil {
		return nil, err
	}
	monitor, err := ic.Get(plugin.TaskMonitorPlugin)
	if err != nil {
		return nil, err
	}
	m, err := ic.Get(plugin.MetadataPlugin)
	if err != nil {
		return nil, err
	}
	cfg := ic.Config.(*Config)
	r := &Runtime{
		root:    ic.Root,
		state:   ic.State,
		monitor: monitor.(runtime.TaskMonitor),
		tasks:   runtime.NewTaskList(),
		db:      m.(*metadata.DB),
		address: ic.Address,
		events:  ic.Events,
		config:  cfg,
	}
	tasks, err := r.restoreTasks(ic.Context)
	if err != nil {
		return nil, err
	}

	// TODO: need to add the tasks to the monitor
	for _, t := range tasks {
		if err := r.tasks.AddWithNamespace(t.namespace, t); err != nil {
			return nil, err
		}
	}
	return r, nil
}

// Runtime for a linux based system
type Runtime struct {
	root    string
	state   string
	address string

	monitor runtime.TaskMonitor
	tasks   *runtime.TaskList
	db      *metadata.DB
	events  *exchange.Exchange

	config *Config
}

// ID of the runtime
func (r *Runtime) ID() string {
	return pluginID
}

// Create a new task
func (r *Runtime) Create(ctx context.Context, id string, opts runtime.CreateOpts) (_ runtime.Task, err error) {
	namespace, err := namespaces.NamespaceRequired(ctx)
	if err != nil {
		return nil, err
	}

	if err := identifiers.Validate(id); err != nil {
		return nil, errors.Wrapf(err, "invalid task id")
	}

	ropts, err := r.getRuncOptions(ctx, id)
	if err != nil {
		return nil, err
	}

	bundle, err := newBundle(id,
		filepath.Join(r.state, namespace),
		filepath.Join(r.root, namespace),
		opts.Spec.Value)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err != nil {
			bundle.Delete()
		}
	}()

	shimopt := ShimLocal(r.config, r.events)
	if !r.config.NoShim {
		var cgroup string
		if opts.Options != nil {
			v, err := typeurl.UnmarshalAny(opts.Options)
			if err != nil {
				return nil, err
			}
			cgroup = v.(*runctypes.CreateOptions).ShimCgroup
		}
		exitHandler := func() {
			log.G(ctx).WithField("id", id).Info("shim reaped")
			t, err := r.tasks.Get(ctx, id)
			if err != nil {
				// Task was never started or was already successfully deleted
				return
			}
			lc := t.(*Task)

			// Stop the monitor
			if err := r.monitor.Stop(lc); err != nil {
				log.G(ctx).WithError(err).WithFields(logrus.Fields{
					"id":        id,
					"namespace": namespace,
				}).Warn("failed to stop monitor")
			}

			log.G(ctx).WithFields(logrus.Fields{
				"id":        id,
				"namespace": namespace,
			}).Warn("cleaning up after killed shim")
			if err = r.cleanupAfterDeadShim(context.Background(), bundle, namespace, id, lc.pid); err != nil {
				log.G(ctx).WithError(err).WithFields(logrus.Fields{
					"id":        id,
					"namespace": namespace,
				}).Warn("failed to clen up after killed shim")
			}
		}
		shimopt = ShimRemote(r.config, r.address, cgroup, exitHandler)
	}

	s, err := bundle.NewShimClient(ctx, namespace, shimopt, ropts)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err != nil {
			if kerr := s.KillShim(ctx); kerr != nil {
				log.G(ctx).WithError(err).Error("failed to kill shim")
			}
		}
	}()

	rt := r.config.Runtime
	if ropts != nil && ropts.Runtime != "" {
		rt = ropts.Runtime
	}
	sopts := &shim.CreateTaskRequest{
		ID:         id,
		Bundle:     bundle.path,
		Runtime:    rt,
		Stdin:      opts.IO.Stdin,
		Stdout:     opts.IO.Stdout,
		Stderr:     opts.IO.Stderr,
		Terminal:   opts.IO.Terminal,
		Checkpoint: opts.Checkpoint,
		Options:    opts.Options,
	}
	for _, m := range opts.Rootfs {
		sopts.Rootfs = append(sopts.Rootfs, &types.Mount{
			Type:    m.Type,
			Source:  m.Source,
			Options: m.Options,
		})
	}
	cr, err := s.Create(ctx, sopts)
	if err != nil {
		return nil, errdefs.FromGRPC(err)
	}
	t, err := newTask(id, namespace, int(cr.Pid), s, r.monitor, r.events,
		proc.NewRunc(ropts.RuntimeRoot, sopts.Bundle, namespace, rt, ropts.CriuPath, ropts.SystemdCgroup))
	if err != nil {
		return nil, err
	}
	if err := r.tasks.Add(ctx, t); err != nil {
		return nil, err
	}
	// after the task is created, add it to the monitor if it has a cgroup
	// this can be different on a checkpoint/restore
	if t.cg != nil {
		if err = r.monitor.Monitor(t); err != nil {
			if _, err := r.Delete(ctx, t); err != nil {
				log.G(ctx).WithError(err).Error("deleting task after failed monitor")
			}
			return nil, err
		}
	}
	r.events.Publish(ctx, runtime.TaskCreateEventTopic, &eventstypes.TaskCreate{
		ContainerID: sopts.ID,
		Bundle:      sopts.Bundle,
		Rootfs:      sopts.Rootfs,
		IO: &eventstypes.TaskIO{
			Stdin:    sopts.Stdin,
			Stdout:   sopts.Stdout,
			Stderr:   sopts.Stderr,
			Terminal: sopts.Terminal,
		},
		Checkpoint: sopts.Checkpoint,
		Pid:        uint32(t.pid),
	})

	return t, nil
}

// Delete a task removing all on disk state
func (r *Runtime) Delete(ctx context.Context, c runtime.Task) (*runtime.Exit, error) {
	namespace, err := namespaces.NamespaceRequired(ctx)
	if err != nil {
		return nil, err
	}
	lc, ok := c.(*Task)
	if !ok {
		return nil, fmt.Errorf("task cannot be cast as *linux.Task")
	}
	if err := r.monitor.Stop(lc); err != nil {
		return nil, err
	}
	bundle := loadBundle(
		lc.id,
		filepath.Join(r.state, namespace, lc.id),
		filepath.Join(r.root, namespace, lc.id),
	)

	rsp, err := lc.shim.Delete(ctx, empty)
	if err != nil {
		if cerr := r.cleanupAfterDeadShim(ctx, bundle, namespace, c.ID(), lc.pid); cerr != nil {
			log.G(ctx).WithError(err).Error("unable to cleanup task")
		}
		return nil, errdefs.FromGRPC(err)
	}
	r.tasks.Delete(ctx, lc.id)
	if err := lc.shim.KillShim(ctx); err != nil {
		log.G(ctx).WithError(err).Error("failed to kill shim")
	}

	if err := bundle.Delete(); err != nil {
		log.G(ctx).WithError(err).Error("failed to delete bundle")
	}
	r.events.Publish(ctx, runtime.TaskDeleteEventTopic, &eventstypes.TaskDelete{
		ContainerID: lc.id,
		ExitStatus:  rsp.ExitStatus,
		ExitedAt:    rsp.ExitedAt,
		Pid:         rsp.Pid,
	})
	return &runtime.Exit{
		Status:    rsp.ExitStatus,
		Timestamp: rsp.ExitedAt,
		Pid:       rsp.Pid,
	}, nil
}

// Tasks returns all tasks known to the runtime
func (r *Runtime) Tasks(ctx context.Context) ([]runtime.Task, error) {
	return r.tasks.GetAll(ctx)
}

func (r *Runtime) restoreTasks(ctx context.Context) ([]*Task, error) {
	dir, err := ioutil.ReadDir(r.state)
	if err != nil {
		return nil, err
	}
	var o []*Task
	for _, namespace := range dir {
		if !namespace.IsDir() {
			continue
		}
		name := namespace.Name()
		log.G(ctx).WithField("namespace", name).Debug("loading tasks in namespace")
		tasks, err := r.loadTasks(ctx, name)
		if err != nil {
			return nil, err
		}
		o = append(o, tasks...)
	}
	return o, nil
}

// Get a specific task by task id
func (r *Runtime) Get(ctx context.Context, id string) (runtime.Task, error) {
	return r.tasks.Get(ctx, id)
}

func (r *Runtime) loadTasks(ctx context.Context, ns string) ([]*Task, error) {
	dir, err := ioutil.ReadDir(filepath.Join(r.state, ns))
	if err != nil {
		return nil, err
	}
	var o []*Task
	for _, path := range dir {
		if !path.IsDir() {
			continue
		}
		id := path.Name()
		bundle := loadBundle(
			id,
			filepath.Join(r.state, ns, id),
			filepath.Join(r.root, ns, id),
		)
		ctx = namespaces.WithNamespace(ctx, ns)
		pid, _ := runc.ReadPidFile(filepath.Join(bundle.path, proc.InitPidFile))
		s, err := bundle.NewShimClient(ctx, ns, ShimConnect(r.config, func() {
			err := r.cleanupAfterDeadShim(ctx, bundle, ns, id, pid)
			if err != nil {
				log.G(ctx).WithError(err).WithField("bundle", bundle.path).
					Error("cleaning up after dead shim")
			}
		}), nil)
		if err != nil {
			log.G(ctx).WithError(err).WithFields(logrus.Fields{
				"id":        id,
				"namespace": ns,
			}).Error("connecting to shim")
			err := r.cleanupAfterDeadShim(ctx, bundle, ns, id, pid)
			if err != nil {
				log.G(ctx).WithError(err).WithField("bundle", bundle.path).
					Error("cleaning up after dead shim")
			}
			continue
		}
		ropts, err := r.getRuncOptions(ctx, id)
		if err != nil {
			log.G(ctx).WithError(err).WithField("id", id).
				Error("get runtime options")
			continue
		}

		t, err := newTask(id, ns, pid, s, r.monitor, r.events,
			proc.NewRunc(ropts.RuntimeRoot, bundle.path, ns, ropts.Runtime, ropts.CriuPath, ropts.SystemdCgroup))
		if err != nil {
			log.G(ctx).WithError(err).Error("loading task type")
			continue
		}
		o = append(o, t)
	}
	return o, nil
}

func (r *Runtime) cleanupAfterDeadShim(ctx context.Context, bundle *bundle, ns, id string, pid int) error {
	ctx = namespaces.WithNamespace(ctx, ns)
	if err := r.terminate(ctx, bundle, ns, id); err != nil {
		if r.config.ShimDebug {
			return errors.Wrap(err, "failed to terminate task, leaving bundle for debugging")
		}
		log.G(ctx).WithError(err).Warn("failed to terminate task")
	}

	// Notify Client
	exitedAt := time.Now().UTC()
	r.events.Publish(ctx, runtime.TaskExitEventTopic, &eventstypes.TaskExit{
		ContainerID: id,
		ID:          id,
		Pid:         uint32(pid),
		ExitStatus:  128 + uint32(unix.SIGKILL),
		ExitedAt:    exitedAt,
	})

	r.tasks.Delete(ctx, id)
	if err := bundle.Delete(); err != nil {
		log.G(ctx).WithError(err).Error("delete bundle")
	}

	r.events.Publish(ctx, runtime.TaskDeleteEventTopic, &eventstypes.TaskDelete{
		ContainerID: id,
		Pid:         uint32(pid),
		ExitStatus:  128 + uint32(unix.SIGKILL),
		ExitedAt:    exitedAt,
	})

	return nil
}

func (r *Runtime) terminate(ctx context.Context, bundle *bundle, ns, id string) error {
	rt, err := r.getRuntime(ctx, ns, id)
	if err != nil {
		return err
	}
	if err := rt.Delete(ctx, id, &runc.DeleteOpts{
		Force: true,
	}); err != nil {
		log.G(ctx).WithError(err).Warnf("delete runtime state %s", id)
	}
	if err := mount.Unmount(filepath.Join(bundle.path, "rootfs"), 0); err != nil {
		log.G(ctx).WithError(err).WithFields(logrus.Fields{
			"path": bundle.path,
			"id":   id,
		}).Warnf("unmount task rootfs")
	}
	return nil
}

func (r *Runtime) getRuntime(ctx context.Context, ns, id string) (*runc.Runc, error) {
	ropts, err := r.getRuncOptions(ctx, id)
	if err != nil {
		return nil, err
	}

	var (
		cmd  = r.config.Runtime
		root = proc.RuncRoot
	)
	if ropts != nil {
		if ropts.Runtime != "" {
			cmd = ropts.Runtime
		}
		if ropts.RuntimeRoot != "" {
			root = ropts.RuntimeRoot
		}
	}

	return &runc.Runc{
		Command:      cmd,
		LogFormat:    runc.JSON,
		PdeathSignal: unix.SIGKILL,
		Root:         filepath.Join(root, ns),
		Debug:        r.config.ShimDebug,
	}, nil
}

func (r *Runtime) getRuncOptions(ctx context.Context, id string) (*runctypes.RuncOptions, error) {
	var container containers.Container

	if err := r.db.View(func(tx *bolt.Tx) error {
		store := metadata.NewContainerStore(tx)
		var err error
		container, err = store.Get(ctx, id)
		return err
	}); err != nil {
		return nil, err
	}

	if container.Runtime.Options != nil {
		v, err := typeurl.UnmarshalAny(container.Runtime.Options)
		if err != nil {
			return nil, err
		}
		ropts, ok := v.(*runctypes.RuncOptions)
		if !ok {
			return nil, errors.New("invalid runtime options format")
		}

		return ropts, nil
	}
	return &runctypes.RuncOptions{}, nil
}
