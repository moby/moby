// Package nri integrates the daemon with the NRI (Node Resource Interface) framework.
//
// NRI allows external plugins to observe and adjust container resources and settings
// at creation time, and to observe container lifecycle events. These plugins run with
// the same level of trust as the daemon itself, because they can make arbitrary
// modifications to container settings.
//
// The NRI framework is implemented by [github.com/containerd/nri] - see that
// package for more details about NRI and the framework.
//
// Plugins are long-running processed (not instantiated per-request like runtime shims,
// so they can maintain state across container events). They can either be started by
// the NRI framework itself, it is configured with directories to search for plugins
// and config for those plugins. Or, plugins can independently, and connect to the
// daemon via a listening socket. By default, the listening socket is disabled in this
// implementation.
package nri

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"sync"

	"github.com/containerd/log"
	"github.com/containerd/nri/pkg/adaptation"
	nrilog "github.com/containerd/nri/pkg/log"
	"github.com/moby/moby/v2/daemon/container"
	"github.com/moby/moby/v2/daemon/internal/rootless"
	"github.com/moby/moby/v2/daemon/pkg/opts"
	"github.com/moby/moby/v2/dockerversion"
	"github.com/moby/moby/v2/pkg/homedir"
)

const (
	// defaultPluginSubdir is the default location for NRI plugins under libexec,
	// which is in a different location for rootful/rootless Docker.
	defaultPluginSubdir = "docker/nri-plugins"
	// defaultPluginConfigSubdir is the default location for NRI plugin config under etc,
	// which is in a different location for rootful/rootless Docker.
	defaultPluginConfigSubdir = "docker/nri/conf.d"
)

type NRI struct {
	cfg Config

	// mu protects nri - read lock for container operations, write lock for sync and shutdown.
	mu   sync.RWMutex
	adap *adaptation.Adaptation
}

type ContainerLister interface {
	List() []*container.Container
}

type Config struct {
	DaemonConfig    opts.NRIOpts
	ContainerLister ContainerLister
}

// NewNRI creates and starts a new NRI instance based on the provided configuration.
func NewNRI(ctx context.Context, cfg Config) (*NRI, error) {
	n := &NRI{cfg: cfg}
	if !n.cfg.DaemonConfig.Enable {
		log.G(ctx).Info("NRI is disabled")
		return n, nil
	}

	if err := setDefaultPaths(&n.cfg.DaemonConfig); err != nil {
		return nil, err
	}
	log.G(ctx).WithFields(log.Fields{
		"pluginPath":       n.cfg.DaemonConfig.PluginPath,
		"pluginConfigPath": n.cfg.DaemonConfig.PluginConfigPath,
		"socketPath":       n.cfg.DaemonConfig.SocketPath,
	}).Info("Starting NRI")
	nrilog.Set(&logShim{})

	var err error
	n.adap, err = adaptation.New("docker", dockerversion.Version, n.syncFn, n.updateFn, nriOptions(n.cfg.DaemonConfig)...)
	if err != nil {
		return nil, err
	}
	if err := n.adap.Start(); err != nil {
		return nil, err
	}
	return n, nil
}

// Shutdown stops the NRI instance and releases its resources.
func (n *NRI) Shutdown(ctx context.Context) {
	n.mu.Lock()
	defer n.mu.Unlock()
	if n.adap == nil {
		return
	}
	log.G(ctx).Info("Shutting down NRI")
	n.adap.Stop()
	n.adap = nil
}

// CreateContainer notifies plugins of a "creation" NRI-lifecycle event for a container,
// allowing the plugin to adjust settings before the container is created.
//
// No lock is acquired on ctr, CreateContainer the caller must ensure it cannot be
// accessed from other threads.
func (n *NRI) CreateContainer(ctx context.Context, ctr *container.Container) error {
	n.mu.RLock()
	defer n.mu.RUnlock()
	if n.adap == nil {
		return nil
	}
	// ctr.State can safely be locked here, but there's no need because it's expected
	// to be newly created and not yet accessible in any other thread.

	nriPod, nriCtr, err := containerToNRI(ctr)
	if err != nil {
		return err
	}

	// TODO(robmry): call RunPodSandbox?

	resp, err := n.adap.CreateContainer(ctx, &adaptation.CreateContainerRequest{
		Pod:       nriPod,
		Container: nriCtr,
	})
	if err != nil {
		return err
	}

	if resp.GetUpdate() != nil {
		return errors.New("container update is not supported")
	}
	if err := applyAdjustments(ctx, ctr, resp.GetAdjust()); err != nil {
		return err
	}
	return nil
}

// syncFn is called when a plugin registers, allowing the plugin to learn the
// current state of all containers.
func (n *NRI) syncFn(ctx context.Context, syncCB adaptation.SyncCB) error {
	// Claim a write lock so containers can't be created/removed until sync is done.
	// The plugin will get create/remove events after the sync, so won't miss anything.
	//
	// If a container's state changes during the sync, the plugin may see already-modified
	// state, then get a change notification with no changes.
	n.mu.Lock()
	defer n.mu.Unlock()

	containers := n.cfg.ContainerLister.List()
	nriPods := make([]*adaptation.PodSandbox, 0, len(containers))
	nriCtrs := make([]*adaptation.Container, 0, len(containers))
	for _, ctr := range containers {
		ctr.State.Lock()
		nriPod, nriCtr, err := containerToNRI(ctr)
		ctr.State.Unlock()
		if err != nil {
			return fmt.Errorf("converting container %s to NRI: %w", ctr.ID, err)
		}
		nriPods = append(nriPods, nriPod)
		nriCtrs = append(nriCtrs, nriCtr)
	}
	updates, err := syncCB(ctx, nriPods, nriCtrs)
	if err != nil {
		return fmt.Errorf("synchronizing NRI state: %w", err)
	}
	if len(updates) > 0 {
		return errors.New("container updates during sync are not implemented")
	}
	return nil
}

// updateFn may be called asynchronously by plugins.
func (n *NRI) updateFn(context.Context, []*adaptation.ContainerUpdate) ([]*adaptation.ContainerUpdate, error) {
	return nil, errors.New("not implemented")
}

func setDefaultPaths(cfg *opts.NRIOpts) error {
	if cfg.PluginPath != "" && cfg.PluginConfigPath != "" {
		return nil
	}
	libexecDir := "/usr/libexec"
	etcDir := "/etc"
	if rootless.RunningWithRootlessKit() {
		var err error
		libexecDir, err = homedir.GetLibexecHome()
		if err != nil {
			return fmt.Errorf("configuring NRI: %w", err)
		}
		etcDir, err = homedir.GetConfigHome()
		if err != nil {
			return fmt.Errorf("configuring NRI: %w", err)
		}
	}
	if cfg.PluginPath == "" {
		cfg.PluginPath = filepath.Join(libexecDir, defaultPluginSubdir)
	}
	if cfg.PluginConfigPath == "" {
		cfg.PluginConfigPath = filepath.Join(etcDir, defaultPluginConfigSubdir)
	}
	return nil
}

func nriOptions(cfg opts.NRIOpts) []adaptation.Option {
	res := []adaptation.Option{
		adaptation.WithPluginPath(cfg.PluginPath),
		adaptation.WithPluginConfigPath(cfg.PluginConfigPath),
	}
	if cfg.SocketPath == "" {
		res = append(res, adaptation.WithDisabledExternalConnections())
	} else {
		res = append(res, adaptation.WithSocketPath(cfg.SocketPath))
	}
	return res
}

func containerToNRI(ctr *container.Container) (*adaptation.PodSandbox, *adaptation.Container, error) {
	// TODO(robmry) - this implementation is incomplete, most fields are not populated.
	//
	// Many of these fields have straightforward mappings from Docker container fields,
	// but each will need consideration and tests for both outgoing settings and
	// adjutments from plugins.
	//
	// Docker doesn't have pods - but PodSandbox is how plugins will learn the container's
	// network namespace. So, the intent is to represent each container as having its own
	// PodSandbox, with the same ID and lifecycle as the container. We can probably represent
	// container-networking as containers sharing a pod.
	nriPod := &adaptation.PodSandbox{
		Id:             ctr.ID,
		Name:           ctr.Name,
		Uid:            "",
		Namespace:      "",
		Labels:         nil,
		Annotations:    nil,
		RuntimeHandler: "",
		Linux:          nil,
		Pid:            0,
		Ips:            nil,
	}

	nriCtr := &adaptation.Container{
		Id:           ctr.ID,
		PodSandboxId: ctr.ID,
		Name:         ctr.Name,
		State:        adaptation.ContainerState_CONTAINER_UNKNOWN,
		Labels:       ctr.Config.Labels,
		Annotations:  ctr.HostConfig.Annotations,
		Args:         ctr.Config.Cmd,
		Env:          ctr.Config.Env,
		Hooks:        nil,
		Linux: &adaptation.LinuxContainer{
			Namespaces:     nil,
			Devices:        nil,
			Resources:      nil,
			OomScoreAdj:    nil,
			CgroupsPath:    "",
			IoPriority:     nil,
			SeccompProfile: nil,
			SeccompPolicy:  nil,
		},
		Mounts:        nil,
		Pid:           uint32(ctr.Pid),
		Rlimits:       nil,
		CreatedAt:     0,
		StartedAt:     0,
		FinishedAt:    0,
		ExitCode:      0,
		StatusReason:  "",
		StatusMessage: "",
		CDIDevices:    nil,
	}
	return nriPod, nriCtr, nil
}

func applyAdjustments(ctx context.Context, ctr *container.Container, adj *adaptation.ContainerAdjustment) error {
	if adj == nil {
		return nil
	}
	if err := applyEnvVars(ctx, ctr, adj.Env); err != nil {
		return fmt.Errorf("applying environment variable adjustments: %w", err)
	}
	return nil
}

func applyEnvVars(ctx context.Context, ctr *container.Container, envVars []*adaptation.KeyValue) error {
	if len(envVars) == 0 {
		return nil
	}
	existing := make(map[string]int, len(ctr.Config.Env))
	for i, e := range ctr.Config.Env {
		k, _, _ := strings.Cut(e, "=")
		existing[k] = i
	}
	for _, kv := range envVars {
		if kv.Key == "" {
			return errors.New("empty environment variable key")
		}
		val := kv.Key + "=" + kv.Value
		log.G(ctx).Debugf("Applying NRI env var adjustment to %s", kv.Key)
		if i, found := existing[kv.Key]; found {
			ctr.Config.Env[i] = val
		} else {
			ctr.Config.Env = append(ctr.Config.Env, val)
		}
	}
	return nil
}
