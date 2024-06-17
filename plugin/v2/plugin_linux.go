// FIXME(thaJeztah): remove once we are a module; the go:build directive prevents go from downgrading language version to go1.16:
//go:build go1.21

package v2 // import "github.com/docker/docker/plugin/v2"

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/containerd/containerd/pkg/userns"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/internal/rootless/mountopts"
	"github.com/docker/docker/internal/sliceutil"
	"github.com/docker/docker/oci"
	specs "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/pkg/errors"
)

// InitSpec creates an OCI spec from the plugin's config.
func (p *Plugin) InitSpec(execRoot string) (*specs.Spec, error) {
	s := oci.DefaultSpec()

	s.Root = &specs.Root{
		Path:     p.Rootfs,
		Readonly: false, // TODO: all plugins should be readonly? settable in config?
	}

	userMounts := make(map[string]struct{}, len(p.PluginObj.Settings.Mounts))
	for _, m := range p.PluginObj.Settings.Mounts {
		userMounts[m.Destination] = struct{}{}
	}

	execRoot = filepath.Join(execRoot, p.PluginObj.ID)
	if err := os.MkdirAll(execRoot, 0o700); err != nil {
		return nil, errors.WithStack(err)
	}

	if p.PluginObj.Config.PropagatedMount != "" {
		pRoot := filepath.Join(filepath.Dir(p.Rootfs), "propagated-mount")
		s.Mounts = append(s.Mounts, specs.Mount{
			Source:      pRoot,
			Destination: p.PluginObj.Config.PropagatedMount,
			Type:        "bind",
			Options:     []string{"rbind", "rw", "rshared"},
		})
		s.Linux.RootfsPropagation = "rshared"
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
			oci.RemoveNamespace(&s, specs.LinuxNamespaceType("network"))
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
	if p.PluginObj.Config.PidHost {
		oci.RemoveNamespace(&s, specs.LinuxNamespaceType("pid"))
	}

	if p.PluginObj.Config.IpcHost {
		oci.RemoveNamespace(&s, specs.LinuxNamespaceType("ipc"))
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

	if p.PluginObj.Config.Linux.AllowAllDevices {
		s.Linux.Resources.Devices = []specs.LinuxDeviceCgroup{{Allow: true, Access: "rwm"}}
	}
	for _, dev := range p.PluginObj.Settings.Devices {
		path := *dev.Path
		d, dPermissions, err := oci.DevicesFromPath(path, path, "rwm")
		if err != nil {
			return nil, errors.WithStack(err)
		}
		s.Linux.Devices = append(s.Linux.Devices, d...)
		s.Linux.Resources.Devices = append(s.Linux.Resources.Devices, dPermissions...)
	}

	envs := make([]string, 1, len(p.PluginObj.Settings.Env)+1)
	envs[0] = "PATH=" + oci.DefaultPathEnv(runtime.GOOS)
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

	caps := s.Process.Capabilities
	caps.Bounding = append(caps.Bounding, p.PluginObj.Config.Linux.Capabilities...)
	caps.Permitted = append(caps.Permitted, p.PluginObj.Config.Linux.Capabilities...)
	caps.Inheritable = append(caps.Inheritable, p.PluginObj.Config.Linux.Capabilities...)
	caps.Effective = append(caps.Effective, p.PluginObj.Config.Linux.Capabilities...)

	if p.modifyRuntimeSpec != nil {
		p.modifyRuntimeSpec(&s)
	}

	// Rootless mode requires modifying the mount flags
	// https://github.com/moby/moby/issues/47248#issuecomment-1927776700
	// https://github.com/moby/moby/pull/47558
	if userns.RunningInUserNS() {
		for i := range s.Mounts {
			m := &s.Mounts[i]
			for _, o := range m.Options {
				switch o {
				case "bind", "rbind":
					if _, err := os.Lstat(m.Source); err != nil {
						if errors.Is(err, os.ErrNotExist) {
							continue
						}
						return nil, err
					}
					// UnprivilegedMountFlags gets the set of mount flags that are set on the mount that contains the given
					// path and are locked by CL_UNPRIVILEGED. This is necessary to ensure that
					// bind-mounting "with options" will not fail with user namespaces, due to
					// kernel restrictions that require user namespace mounts to preserve
					// CL_UNPRIVILEGED locked flags.
					unpriv, err := mountopts.UnprivilegedMountFlags(m.Source)
					if err != nil {
						return nil, errors.Wrapf(err, "failed to get unprivileged mount flags for %+v", m)
					}
					m.Options = sliceutil.Dedup(append(m.Options, unpriv...))
				}
			}
		}
	}

	return &s, nil
}
