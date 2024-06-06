//go:build !windows
// +build !windows

package containerdexecutor

import (
	"context"
	"os"
	"runtime"

	"github.com/containerd/containerd"
	"github.com/containerd/containerd/mount"
	containerdoci "github.com/containerd/containerd/oci"
	"github.com/containerd/continuity/fs"
	"github.com/docker/docker/pkg/idtools"
	"github.com/moby/buildkit/executor"
	"github.com/moby/buildkit/executor/oci"
	"github.com/moby/buildkit/snapshot"
	"github.com/moby/buildkit/solver/pb"
	"github.com/moby/buildkit/util/bklog"
	"github.com/moby/buildkit/util/network"
	rootlessspecconv "github.com/moby/buildkit/util/rootless/specconv"
	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/pkg/errors"
)

func getUserSpec(user, rootfsPath string) (specs.User, error) {
	var err error
	var uid, gid uint32
	var sgids []uint32
	if rootfsPath != "" {
		uid, gid, sgids, err = oci.GetUser(rootfsPath, user)
	} else {
		uid, gid, err = oci.ParseUIDGID(user)
	}
	if err != nil {
		return specs.User{}, errors.WithStack(err)
	}
	return specs.User{
		UID:            uid,
		GID:            gid,
		AdditionalGids: sgids,
	}, nil
}

func (w *containerdExecutor) prepareExecutionEnv(ctx context.Context, rootMount executor.Mount, mounts []executor.Mount, meta executor.Meta, details *containerState, netMode pb.NetMode) (string, string, func(), error) {
	var releasers []func()
	releaseAll := func() {
		for i := len(releasers) - 1; i >= 0; i-- {
			releasers[i]()
		}
	}

	resolvConf, err := oci.GetResolvConf(ctx, w.root, nil, w.dnsConfig, netMode)
	if err != nil {
		releaseAll()
		return "", "", nil, err
	}

	hostsFile, clean, err := oci.GetHostsFile(ctx, w.root, meta.ExtraHosts, nil, meta.Hostname)
	if err != nil {
		releaseAll()
		return "", "", nil, err
	}
	if clean != nil {
		releasers = append(releasers, clean)
	}
	mountable, err := rootMount.Src.Mount(ctx, false)
	if err != nil {
		releaseAll()
		return "", "", nil, err
	}

	rootMounts, release, err := mountable.Mount()
	if err != nil {
		releaseAll()
		return "", "", nil, err
	}
	details.rootMounts = rootMounts

	if release != nil {
		releasers = append(releasers, func() {
			if err := release(); err != nil {
				bklog.G(ctx).WithError(err).Error("failed to release root mount")
			}
		})
	}
	lm := snapshot.LocalMounterWithMounts(rootMounts)
	rootfsPath, err := lm.Mount()
	if err != nil {
		releaseAll()
		return "", "", nil, err
	}
	details.rootfsPath = rootfsPath
	releasers = append(releasers, func() {
		if err := lm.Unmount(); err != nil {
			bklog.G(ctx).WithError(err).Error("failed to unmount rootfs")
		}
	})
	releasers = append(releasers, executor.MountStubsCleaner(ctx, details.rootfsPath, mounts, meta.RemoveMountStubsRecursive))

	return resolvConf, hostsFile, releaseAll, nil
}

func (w *containerdExecutor) ensureCWD(_ context.Context, details *containerState, meta executor.Meta) error {
	newp, err := fs.RootPath(details.rootfsPath, meta.Cwd)
	if err != nil {
		return errors.Wrapf(err, "working dir %s points to invalid target", newp)
	}

	uid, gid, _, err := oci.GetUser(details.rootfsPath, meta.User)
	if err != nil {
		return err
	}

	identity := idtools.Identity{
		UID: int(uid),
		GID: int(gid),
	}

	if _, err := os.Stat(newp); err != nil {
		if err := idtools.MkdirAllAndChown(newp, 0755, identity); err != nil {
			return errors.Wrapf(err, "failed to create working directory %s", newp)
		}
	}
	return nil
}

func (w *containerdExecutor) createOCISpec(ctx context.Context, id, resolvConf, hostsFile string, namespace network.Namespace, mounts []executor.Mount, meta executor.Meta, details *containerState) (*specs.Spec, func(), error) {
	var releasers []func()
	releaseAll := func() {
		for i := len(releasers) - 1; i >= 0; i-- {
			releasers[i]()
		}
	}

	uid, gid, sgids, err := oci.GetUser(details.rootfsPath, meta.User)
	if err != nil {
		releaseAll()
		return nil, nil, err
	}

	opts := []containerdoci.SpecOpts{oci.WithUIDGID(uid, gid, sgids)}
	if meta.ReadonlyRootFS {
		opts = append(opts, containerdoci.WithRootFSReadonly())
	}

	processMode := oci.ProcessSandbox // FIXME(AkihiroSuda)
	spec, cleanup, err := oci.GenerateSpec(ctx, meta, mounts, id, resolvConf, hostsFile, namespace, w.cgroupParent, processMode, nil, w.apparmorProfile, w.selinux, w.traceSocket, opts...)
	if err != nil {
		releaseAll()
		return nil, nil, err
	}
	releasers = append(releasers, cleanup)
	spec.Process.Terminal = meta.Tty
	if w.rootless {
		if err := rootlessspecconv.ToRootless(spec); err != nil {
			releaseAll()
			return nil, nil, err
		}
	}
	return spec, releaseAll, nil
}

func (d *containerState) getTaskOpts() ([]containerd.NewTaskOpts, error) {
	rootfs := containerd.WithRootFS([]mount.Mount{{
		Source:  d.rootfsPath,
		Type:    "bind",
		Options: []string{"rbind"},
	}})
	if runtime.GOOS == "freebsd" {
		rootfs = containerd.WithRootFS([]mount.Mount{{
			Source:  d.rootfsPath,
			Type:    "nullfs",
			Options: []string{},
		}})
	}
	return []containerd.NewTaskOpts{rootfs}, nil
}

func setArgs(spec *specs.Process, args []string) {
	spec.Args = args
}
