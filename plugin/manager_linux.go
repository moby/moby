// +build linux,experimental

package plugin

import (
	"os"
	"path/filepath"
	"syscall"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/docker/docker/libcontainerd"
	"github.com/docker/docker/oci"
	"github.com/docker/docker/pkg/plugins"
	"github.com/docker/docker/pkg/system"
	"github.com/docker/docker/restartmanager"
	"github.com/docker/engine-api/types"
	"github.com/docker/engine-api/types/container"
	"github.com/opencontainers/specs/specs-go"
)

func (pm *Manager) enable(p *plugin) error {
	spec, err := pm.initSpec(p)
	if err != nil {
		return err
	}

	p.restartManager = restartmanager.New(container.RestartPolicy{Name: "always"}, 0)
	if err := pm.containerdClient.Create(p.P.ID, libcontainerd.Spec(*spec), libcontainerd.WithRestartManager(p.restartManager)); err != nil { // POC-only
		return err
	}

	socket := p.P.Manifest.Interface.Socket
	p.client, err = plugins.NewClient("unix://"+filepath.Join(p.runtimeSourcePath, socket), nil)
	if err != nil {
		return err
	}

	pm.Lock() // fixme: lock single record
	p.P.Active = true
	pm.save()
	pm.Unlock()

	for _, typ := range p.P.Manifest.Interface.Types {
		if handler := pm.handlers[typ.String()]; handler != nil {
			handler(p.Name(), p.Client())
		}
	}

	return nil
}

func (pm *Manager) restore(p *plugin) error {
	p.restartManager = restartmanager.New(container.RestartPolicy{Name: "always"}, 0)
	return pm.containerdClient.Restore(p.P.ID, libcontainerd.WithRestartManager(p.restartManager))
}

func (pm *Manager) initSpec(p *plugin) (*specs.Spec, error) {
	s := oci.DefaultSpec()

	rootfs := filepath.Join(pm.libRoot, p.P.ID, "rootfs")
	s.Root = specs.Root{
		Path:     rootfs,
		Readonly: false, // TODO: all plugins should be readonly? settable in manifest?
	}

	mounts := append(p.P.Config.Mounts, types.PluginMount{
		Source:      &p.runtimeSourcePath,
		Destination: defaultPluginRuntimeDestination,
		Type:        "bind",
		Options:     []string{"rbind", "rshared"},
	})
	for _, mount := range mounts {
		m := specs.Mount{
			Destination: mount.Destination,
			Type:        mount.Type,
			Options:     mount.Options,
		}
		// TODO: if nil, then it's required and user didn't set it
		if mount.Source != nil {
			m.Source = *mount.Source
		}
		if m.Source != "" && m.Type == "bind" {
			fi, err := os.Lstat(filepath.Join(rootfs, string(os.PathSeparator), m.Destination)) // TODO: followsymlinks
			if err != nil {
				return nil, err
			}
			if fi.IsDir() {
				if err := os.MkdirAll(m.Source, 0700); err != nil {
					return nil, err
				}
			}
		}
		s.Mounts = append(s.Mounts, m)
	}

	envs := make([]string, 1, len(p.P.Config.Env)+1)
	envs[0] = "PATH=" + system.DefaultPathEnv
	envs = append(envs, p.P.Config.Env...)

	args := append(p.P.Manifest.Entrypoint, p.P.Config.Args...)
	cwd := p.P.Manifest.Workdir
	if len(cwd) == 0 {
		cwd = "/"
	}
	s.Process = specs.Process{
		Terminal: false,
		Args:     args,
		Cwd:      cwd,
		Env:      envs,
	}

	return &s, nil
}

func (pm *Manager) disable(p *plugin) error {
	if err := p.restartManager.Cancel(); err != nil {
		logrus.Error(err)
	}
	if err := pm.containerdClient.Signal(p.P.ID, int(syscall.SIGKILL)); err != nil {
		logrus.Error(err)
	}
	os.RemoveAll(p.runtimeSourcePath)
	pm.Lock() // fixme: lock single record
	defer pm.Unlock()
	p.P.Active = false
	pm.save()
	return nil
}

// Shutdown stops all plugins and called during daemon shutdown.
func (pm *Manager) Shutdown() {
	pm.RLock()
	defer pm.RUnlock()

	pm.shutdown = true
	for _, p := range pm.plugins {
		if pm.liveRestore && p.P.Active {
			logrus.Debug("Plugin active when liveRestore is set, skipping shutdown")
			continue
		}
		if p.restartManager != nil {
			if err := p.restartManager.Cancel(); err != nil {
				logrus.Error(err)
			}
		}
		if pm.containerdClient != nil && p.P.Active {
			p.exitChan = make(chan bool)
			err := pm.containerdClient.Signal(p.P.ID, int(syscall.SIGTERM))
			if err != nil {
				logrus.Errorf("Sending SIGTERM to plugin failed with error: %v", err)
			} else {
				select {
				case <-p.exitChan:
					logrus.Debug("Clean shutdown of plugin")
				case <-time.After(time.Second * 10):
					logrus.Debug("Force shutdown plugin")
					if err := pm.containerdClient.Signal(p.P.ID, int(syscall.SIGKILL)); err != nil {
						logrus.Errorf("Sending SIGKILL to plugin failed with error: %v", err)
					}
				}
			}
			close(p.exitChan)
			pm.Lock()
			p.P.Active = false
			pm.save()
			pm.Unlock()
		}
		if err := os.RemoveAll(p.runtimeSourcePath); err != nil {
			logrus.Errorf("Remove plugin runtime failed with error: %v", err)
		}
	}
}
