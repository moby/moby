package containerdexecutor

import (
	"context"
	"os"
	"strings"

	"github.com/containerd/containerd"
	containerdoci "github.com/containerd/containerd/oci"
	"github.com/containerd/continuity/fs"
	"github.com/docker/docker/pkg/idtools"
	"github.com/moby/buildkit/executor"
	"github.com/moby/buildkit/executor/oci"
	"github.com/moby/buildkit/snapshot"
	"github.com/moby/buildkit/solver/pb"
	"github.com/moby/buildkit/util/network"
	"github.com/moby/buildkit/util/windows"
	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/pkg/errors"
)

func getUserSpec(user, rootfsPath string) (specs.User, error) {
	return specs.User{
		Username: user,
	}, nil
}

func (w *containerdExecutor) prepareExecutionEnv(ctx context.Context, rootMount executor.Mount, mounts []executor.Mount, meta executor.Meta, details *containerState, netMode pb.NetMode) (string, string, func(), error) {
	var releasers []func() error
	releaseAll := func() {
		for _, release := range releasers {
			release()
		}
	}

	mountable, err := rootMount.Src.Mount(ctx, false)
	if err != nil {
		return "", "", releaseAll, err
	}

	rootMounts, release, err := mountable.Mount()
	if err != nil {
		return "", "", releaseAll, err
	}
	details.rootMounts = rootMounts
	releasers = append(releasers, release)

	return "", "", releaseAll, nil
}

func (w *containerdExecutor) ensureCWD(ctx context.Context, details *containerState, meta executor.Meta) (err error) {
	// TODO(gabriel-samfira): Use a snapshot?
	identity, err := windows.ResolveUsernameToSID(ctx, w, details.rootMounts, meta.User)
	if err != nil {
		return errors.Wrap(err, "getting user SID")
	}

	lm := snapshot.LocalMounterWithMounts(details.rootMounts)
	rootfsPath, err := lm.Mount()
	if err != nil {
		return err
	}
	defer lm.Unmount()

	newp, err := fs.RootPath(rootfsPath, meta.Cwd)
	if err != nil {
		return errors.Wrapf(err, "working dir %s points to invalid target", newp)
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
		for _, release := range releasers {
			release()
		}
	}

	opts := []containerdoci.SpecOpts{
		containerdoci.WithUser(meta.User),
	}

	processMode := oci.ProcessSandbox // FIXME(AkihiroSuda)
	spec, cleanup, err := oci.GenerateSpec(ctx, meta, mounts, id, "", "", namespace, "", processMode, nil, "", false, w.traceSocket, opts...)
	if err != nil {
		releaseAll()
		return nil, nil, err
	}
	releasers = append(releasers, cleanup)
	return spec, releaseAll, nil
}

func (d *containerState) getTaskOpts() ([]containerd.NewTaskOpts, error) {
	return []containerd.NewTaskOpts{containerd.WithRootFS(d.rootMounts)}, nil
}

func setArgs(spec *specs.Process, args []string) {
	spec.CommandLine = strings.Join(args, " ")
}
