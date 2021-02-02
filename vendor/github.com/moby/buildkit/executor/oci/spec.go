package oci

import (
	"context"
	"path"
	"sync"

	"github.com/containerd/containerd/containers"
	"github.com/containerd/containerd/mount"
	"github.com/containerd/containerd/namespaces"
	"github.com/containerd/containerd/oci"
	"github.com/containerd/continuity/fs"
	"github.com/docker/docker/pkg/idtools"
	"github.com/mitchellh/hashstructure"
	"github.com/moby/buildkit/executor"
	"github.com/moby/buildkit/snapshot"
	"github.com/moby/buildkit/util/network"
	specs "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/opencontainers/selinux/go-selinux"
	"github.com/pkg/errors"
)

// ProcessMode configures PID namespaces
type ProcessMode int

const (
	// ProcessSandbox unshares pidns and mount procfs.
	ProcessSandbox ProcessMode = iota
	// NoProcessSandbox uses host pidns and bind-mount procfs.
	// Note that NoProcessSandbox allows build containers to kill (and potentially ptrace) an arbitrary process in the BuildKit host namespace.
	// NoProcessSandbox should be enabled only when the BuildKit is running in a container as an unprivileged user.
	NoProcessSandbox
)

// Ideally we don't have to import whole containerd just for the default spec

// GenerateSpec generates spec using containerd functionality.
// opts are ignored for s.Process, s.Hostname, and s.Mounts .
func GenerateSpec(ctx context.Context, meta executor.Meta, mounts []executor.Mount, id, resolvConf, hostsFile string, namespace network.Namespace, processMode ProcessMode, idmap *idtools.IdentityMapping, apparmorProfile string, opts ...oci.SpecOpts) (*specs.Spec, func(), error) {
	c := &containers.Container{
		ID: id,
	}

	// containerd/oci.GenerateSpec requires a namespace, which
	// will be used to namespace specs.Linux.CgroupsPath if generated
	if _, ok := namespaces.Namespace(ctx); !ok {
		ctx = namespaces.WithNamespace(ctx, "buildkit")
	}

	if mountOpts, err := generateMountOpts(resolvConf, hostsFile); err == nil {
		opts = append(opts, mountOpts...)
	} else {
		return nil, nil, err
	}

	if securityOpts, err := generateSecurityOpts(meta.SecurityMode, apparmorProfile); err == nil {
		opts = append(opts, securityOpts...)
	} else {
		return nil, nil, err
	}

	if processModeOpts, err := generateProcessModeOpts(processMode); err == nil {
		opts = append(opts, processModeOpts...)
	} else {
		return nil, nil, err
	}

	if idmapOpts, err := generateIDmapOpts(idmap); err == nil {
		opts = append(opts, idmapOpts...)
	} else {
		return nil, nil, err
	}

	hostname := defaultHostname
	if meta.Hostname != "" {
		hostname = meta.Hostname
	}

	opts = append(opts,
		oci.WithProcessArgs(meta.Args...),
		oci.WithEnv(meta.Env),
		oci.WithProcessCwd(meta.Cwd),
		oci.WithNewPrivileges,
		oci.WithHostname(hostname),
	)

	s, err := oci.GenerateSpec(ctx, nil, c, opts...)
	if err != nil {
		return nil, nil, err
	}

	// set the networking information on the spec
	if err := namespace.Set(s); err != nil {
		return nil, nil, err
	}

	s.Process.Rlimits = nil // reset open files limit

	sm := &submounts{}

	var releasers []func() error
	releaseAll := func() {
		sm.cleanup()
		for _, f := range releasers {
			f()
		}
		if s.Process.SelinuxLabel != "" {
			selinux.ReleaseLabel(s.Process.SelinuxLabel)
		}
	}

	for _, m := range mounts {
		if m.Src == nil {
			return nil, nil, errors.Errorf("mount %s has no source", m.Dest)
		}
		mountable, err := m.Src.Mount(ctx, m.Readonly)
		if err != nil {
			releaseAll()
			return nil, nil, errors.Wrapf(err, "failed to mount %s", m.Dest)
		}
		mounts, release, err := mountable.Mount()
		if err != nil {
			releaseAll()
			return nil, nil, errors.WithStack(err)
		}
		releasers = append(releasers, release)
		for _, mount := range mounts {
			mount, err = sm.subMount(mount, m.Selector)
			if err != nil {
				releaseAll()
				return nil, nil, err
			}
			s.Mounts = append(s.Mounts, specs.Mount{
				Destination: m.Dest,
				Type:        mount.Type,
				Source:      mount.Source,
				Options:     mount.Options,
			})
		}
	}

	return s, releaseAll, nil
}

type mountRef struct {
	mount   mount.Mount
	unmount func() error
}

type submounts struct {
	m map[uint64]mountRef
}

func (s *submounts) subMount(m mount.Mount, subPath string) (mount.Mount, error) {
	if path.Join("/", subPath) == "/" {
		return m, nil
	}
	if s.m == nil {
		s.m = map[uint64]mountRef{}
	}
	h, err := hashstructure.Hash(m, nil)
	if err != nil {
		return mount.Mount{}, nil
	}
	if mr, ok := s.m[h]; ok {
		sm, err := sub(mr.mount, subPath)
		if err != nil {
			return mount.Mount{}, nil
		}
		return sm, nil
	}

	lm := snapshot.LocalMounterWithMounts([]mount.Mount{m})

	mp, err := lm.Mount()
	if err != nil {
		return mount.Mount{}, err
	}

	opts := []string{"rbind"}
	for _, opt := range m.Options {
		if opt == "ro" {
			opts = append(opts, opt)
		}
	}

	s.m[h] = mountRef{
		mount: mount.Mount{
			Source:  mp,
			Type:    "bind",
			Options: opts,
		},
		unmount: lm.Unmount,
	}

	sm, err := sub(s.m[h].mount, subPath)
	if err != nil {
		return mount.Mount{}, err
	}
	return sm, nil
}

func (s *submounts) cleanup() {
	var wg sync.WaitGroup
	wg.Add(len(s.m))
	for _, m := range s.m {
		func(m mountRef) {
			go func() {
				m.unmount()
				wg.Done()
			}()
		}(m)
	}
	wg.Wait()
}

func sub(m mount.Mount, subPath string) (mount.Mount, error) {
	src, err := fs.RootPath(m.Source, subPath)
	if err != nil {
		return mount.Mount{}, err
	}
	m.Source = src
	return m, nil
}

func specMapping(s []idtools.IDMap) []specs.LinuxIDMapping {
	var ids []specs.LinuxIDMapping
	for _, item := range s {
		ids = append(ids, specs.LinuxIDMapping{
			HostID:      uint32(item.HostID),
			ContainerID: uint32(item.ContainerID),
			Size:        uint32(item.Size),
		})
	}
	return ids
}
