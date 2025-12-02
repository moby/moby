package nri

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"sync"

	"github.com/containerd/log"
	"github.com/containerd/nri/pkg/adaptation"
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
	mu  sync.RWMutex
	nri *adaptation.Adaptation
}

type ContainerLister interface {
	List() []*container.Container
}

type Config struct {
	DaemonConfig    opts.NRIOpts
	ContainerLister ContainerLister
}

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

	var err error
	n.nri, err = adaptation.New("docker", dockerversion.Version, n.syncFn, n.updateFn, nriOptions(n.cfg.DaemonConfig)...)
	if err != nil {
		return nil, err
	}
	if err := n.nri.Start(); err != nil {
		return nil, err
	}
	return n, nil
}

func (n *NRI) Shutdown(ctx context.Context) {
	n.mu.Lock()
	defer n.mu.Unlock()
	if n.nri == nil {
		return
	}
	log.G(ctx).Info("Shutting down NRI")
	n.nri.Stop()
	n.nri = nil
}

func (n *NRI) syncFn(ctx context.Context, syncCB adaptation.SyncCB) error {
	return nil
}

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
