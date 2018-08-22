// +build !windows

package oci

import (
	"context"
	"path"
	"sync"

	"github.com/containerd/containerd/containers"
	"github.com/containerd/containerd/mount"
	"github.com/containerd/containerd/namespaces"
	"github.com/containerd/containerd/oci"
	"github.com/mitchellh/hashstructure"
	"github.com/moby/buildkit/executor"
	"github.com/moby/buildkit/snapshot"
	"github.com/moby/buildkit/util/network"
	specs "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/pkg/errors"
)

// Ideally we don't have to import whole containerd just for the default spec

// GenerateSpec generates spec using containerd functionality.
func GenerateSpec(ctx context.Context, meta executor.Meta, mounts []executor.Mount, id, resolvConf, hostsFile string, namespace network.Namespace, opts ...oci.SpecOpts) (*specs.Spec, func(), error) {
	c := &containers.Container{
		ID: id,
	}
	_, ok := namespaces.Namespace(ctx)
	if !ok {
		ctx = namespaces.WithNamespace(ctx, "buildkit")
	}

	// Note that containerd.GenerateSpec is namespaced so as to make
	// specs.Linux.CgroupsPath namespaced
	s, err := oci.GenerateSpec(ctx, nil, c, opts...)
	if err != nil {
		return nil, nil, err
	}
	// set the networking information on the spec
	namespace.Set(s)

	s.Process.Args = meta.Args
	s.Process.Env = meta.Env
	s.Process.Cwd = meta.Cwd

	s.Mounts = GetMounts(ctx,
		withROBind(resolvConf, "/etc/resolv.conf"),
		withROBind(hostsFile, "/etc/hosts"),
	)
	// TODO: User

	sm := &submounts{}

	var releasers []func() error
	releaseAll := func() {
		sm.cleanup()
		for _, f := range releasers {
			f()
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
		mounts, err := mountable.Mount()
		if err != nil {
			releaseAll()
			return nil, nil, errors.WithStack(err)
		}
		releasers = append(releasers, mountable.Release)
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
		return sub(mr.mount, subPath), nil
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

	return sub(s.m[h].mount, subPath), nil
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

func sub(m mount.Mount, subPath string) mount.Mount {
	m.Source = path.Join(m.Source, subPath)
	return m
}
