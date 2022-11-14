package oci

import (
	"context"
	"path"
	"path/filepath"
	"strings"
	"sync"

	"github.com/containerd/containerd/containers"
	"github.com/containerd/containerd/mount"
	"github.com/containerd/containerd/namespaces"
	"github.com/containerd/containerd/oci"
	"github.com/containerd/continuity/fs"
	"github.com/docker/docker/pkg/idtools"
	"github.com/mitchellh/hashstructure/v2"
	"github.com/moby/buildkit/executor"
	"github.com/moby/buildkit/snapshot"
	"github.com/moby/buildkit/util/network"
	traceexec "github.com/moby/buildkit/util/tracing/exec"
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

func (pm ProcessMode) String() string {
	switch pm {
	case ProcessSandbox:
		return "sandbox"
	case NoProcessSandbox:
		return "no-sandbox"
	default:
		return ""
	}
}

// Ideally we don't have to import whole containerd just for the default spec

// GenerateSpec generates spec using containerd functionality.
// opts are ignored for s.Process, s.Hostname, and s.Mounts .
func GenerateSpec(ctx context.Context, meta executor.Meta, mounts []executor.Mount, id, resolvConf, hostsFile string, namespace network.Namespace, cgroupParent string, processMode ProcessMode, idmap *idtools.IdentityMapping, apparmorProfile string, selinuxB bool, tracingSocket string, opts ...oci.SpecOpts) (*specs.Spec, func(), error) {
	c := &containers.Container{
		ID: id,
	}

	if len(meta.CgroupParent) > 0 {
		cgroupParent = meta.CgroupParent
	}
	if cgroupParent != "" {
		var cgroupsPath string
		lastSeparator := cgroupParent[len(cgroupParent)-1:]
		if strings.Contains(cgroupParent, ".slice") && lastSeparator == ":" {
			cgroupsPath = cgroupParent + id
		} else {
			cgroupsPath = filepath.Join("/", cgroupParent, "buildkit", id)
		}
		opts = append(opts, oci.WithCgroup(cgroupsPath))
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

	if securityOpts, err := generateSecurityOpts(meta.SecurityMode, apparmorProfile, selinuxB); err == nil {
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

	if rlimitsOpts, err := generateRlimitOpts(meta.Ulimit); err == nil {
		opts = append(opts, rlimitsOpts...)
	} else {
		return nil, nil, err
	}

	hostname := defaultHostname
	if meta.Hostname != "" {
		hostname = meta.Hostname
	}

	if tracingSocket != "" {
		// https://github.com/open-telemetry/opentelemetry-specification/blob/main/specification/protocol/exporter.md
		meta.Env = append(meta.Env, "OTEL_TRACES_EXPORTER=otlp", "OTEL_EXPORTER_OTLP_TRACES_ENDPOINT=unix:///dev/otel-grpc.sock", "OTEL_EXPORTER_OTLP_TRACES_PROTOCOL=grpc")
		meta.Env = append(meta.Env, traceexec.Environ(ctx)...)
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

	if len(meta.Ulimit) == 0 {
		// reset open files limit
		s.Process.Rlimits = nil
	}

	// set the networking information on the spec
	if err := namespace.Set(s); err != nil {
		return nil, nil, err
	}

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

	if tracingSocket != "" {
		s.Mounts = append(s.Mounts, specs.Mount{
			Destination: "/dev/otel-grpc.sock",
			Type:        "bind",
			Source:      tracingSocket,
			Options:     []string{"ro", "rbind"},
		})
	}

	s.Mounts = dedupMounts(s.Mounts)
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
	h, err := hashstructure.Hash(m, hashstructure.FormatV2, nil)
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
