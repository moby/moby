// +build linux

package v2

import (
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/oci"
	"github.com/docker/docker/pkg/system"
	specs "github.com/opencontainers/runtime-spec/specs-go"
)

// InitSpec creates an OCI spec from the plugin's config.
func (p *Plugin) InitSpec(execRoot string) (*specs.Spec, error) {
	s := oci.DefaultSpec()
	s.Root = specs.Root{
		Path:     p.Rootfs,
		Readonly: false, // TODO: all plugins should be readonly? settable in config?
	}

	userMounts := make(map[string]struct{}, len(p.PluginObj.Settings.Mounts))
	for _, m := range p.PluginObj.Settings.Mounts {
		userMounts[m.Destination] = struct{}{}
	}

	execRoot = filepath.Join(execRoot, p.PluginObj.ID)
	if err := os.MkdirAll(execRoot, 0700); err != nil {
		return nil, err
	}

	mounts := append(p.PluginObj.Config.Mounts, types.PluginMount{
		Source:      &execRoot,
		Destination: defaultPluginRuntimeDestination,
		Type:        "bind",
		Options:     []string{"rbind", "rshared"},
	})

	if p.PluginObj.Config.Network.Type != "" {
		// TODO: if net == bridge, use libnetwork controller to create a new plugin-specific bridge, bind mount /etc/hosts and /etc/resolv.conf look at the docker code (allocateNetwork, initialize)
		if p.PluginObj.Config.Network.Type == "host" {
			oci.RemoveNamespace(&s, specs.NamespaceType("network"))
		}
		etcHosts := "/etc/hosts"
		resolvConf := "/etc/resolv.conf"
		mounts = append(mounts,
			types.PluginMount{
				Source:      &etcHosts,
				Destination: etcHosts,
				Type:        "bind",
				Options:     []string{"rbind", "ro"},
			},
			types.PluginMount{
				Source:      &resolvConf,
				Destination: resolvConf,
				Type:        "bind",
				Options:     []string{"rbind", "ro"},
			})
	}

	for _, mnt := range mounts {
		m := specs.Mount{
			Destination: mnt.Destination,
			Type:        mnt.Type,
			Options:     mnt.Options,
		}
		if mnt.Source == nil {
			return nil, errors.New("mount source is not specified")
		}
		m.Source = *mnt.Source
		s.Mounts = append(s.Mounts, m)
	}

	for i, m := range s.Mounts {
		if strings.HasPrefix(m.Destination, "/dev/") {
			if _, ok := userMounts[m.Destination]; ok {
				s.Mounts = append(s.Mounts[:i], s.Mounts[i+1:]...)
			}
		}
	}

	if p.PluginObj.Config.PropagatedMount != "" {
		p.PropagatedMount = filepath.Join(p.Rootfs, p.PluginObj.Config.PropagatedMount)
		s.Linux.RootfsPropagation = "rshared"
	}

	if p.PluginObj.Config.Linux.DeviceCreation {
		rwm := "rwm"
		s.Linux.Resources.Devices = []specs.DeviceCgroup{{Allow: true, Access: &rwm}}
	}
	for _, dev := range p.PluginObj.Settings.Devices {
		path := *dev.Path
		d, dPermissions, err := oci.DevicesFromPath(path, path, "rwm")
		if err != nil {
			return nil, err
		}
		s.Linux.Devices = append(s.Linux.Devices, d...)
		s.Linux.Resources.Devices = append(s.Linux.Resources.Devices, dPermissions...)
	}

	envs := make([]string, 1, len(p.PluginObj.Settings.Env)+1)
	envs[0] = "PATH=" + system.DefaultPathEnv
	envs = append(envs, p.PluginObj.Settings.Env...)

	args := append(p.PluginObj.Config.Entrypoint, p.PluginObj.Settings.Args...)
	cwd := p.PluginObj.Config.WorkDir
	if len(cwd) == 0 {
		cwd = "/"
	}
	s.Process.Terminal = false
	s.Process.Args = args
	s.Process.Cwd = cwd
	s.Process.Env = envs

	s.Process.Capabilities = append(s.Process.Capabilities, p.PluginObj.Config.Linux.Capabilities...)

	return &s, nil
}
