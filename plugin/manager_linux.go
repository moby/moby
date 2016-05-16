// +build linux,experimental

package plugin

import (
	"os"
	"path/filepath"
	"syscall"

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
	if err := pm.containerdClient.Create(p.p.ID, libcontainerd.Spec(*spec), libcontainerd.WithRestartManager(p.restartManager)); err != nil { // POC-only
		return err
	}

	socket := p.p.Manifest.Interface.Socket
	p.client, err = plugins.NewClient("unix://"+filepath.Join(p.runtimeSourcePath, socket), nil)
	if err != nil {
		return err
	}

	//TODO: check net.Dial

	pm.Lock() // fixme: lock single record
	p.p.Active = true
	pm.save()
	pm.Unlock()

	for _, typ := range p.p.Manifest.Interface.Types {
		if handler := pm.handlers[typ.String()]; handler != nil {
			handler(p.Name(), p.Client())
		}
	}

	return nil
}

func (pm *Manager) initSpec(p *plugin) (*specs.Spec, error) {
	s := oci.DefaultSpec()

	rootfs := filepath.Join(pm.libRoot, p.p.ID, "rootfs")
	s.Root = specs.Root{
		Path:     rootfs,
		Readonly: false, // TODO: all plugins should be readonly? settable in manifest?
	}

	mounts := append(p.p.Config.Mounts, types.PluginMount{
		Source:      &p.runtimeSourcePath,
		Destination: defaultPluginRuntimeDestination,
		Type:        "bind",
		Options:     []string{"rbind", "rshared"},
	}, types.PluginMount{
		Source:      &p.stateSourcePath,
		Destination: defaultPluginStateDestination,
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

	envs := make([]string, 1, len(p.p.Config.Env)+1)
	envs[0] = "PATH=" + system.DefaultPathEnv
	envs = append(envs, p.p.Config.Env...)

	args := append(p.p.Manifest.Entrypoint, p.p.Config.Args...)
	s.Process = specs.Process{
		Terminal: false,
		Args:     args,
		Cwd:      "/", // TODO: add in manifest?
		Env:      envs,
	}

	return &s, nil
}

func (pm *Manager) disable(p *plugin) error {
	if err := p.restartManager.Cancel(); err != nil {
		logrus.Error(err)
	}
	if err := pm.containerdClient.Signal(p.p.ID, int(syscall.SIGKILL)); err != nil {
		logrus.Error(err)
	}
	os.RemoveAll(p.runtimeSourcePath)
	pm.Lock() // fixme: lock single record
	defer pm.Unlock()
	p.p.Active = false
	pm.save()
	return nil
}
