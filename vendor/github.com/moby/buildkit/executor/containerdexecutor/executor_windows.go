package containerdexecutor

import (
	"context"
	"os"
	"strings"

	ctd "github.com/containerd/containerd/v2/client"
	containerdoci "github.com/containerd/containerd/v2/pkg/oci"
	"github.com/containerd/continuity/fs"
	"github.com/moby/buildkit/executor"
	"github.com/moby/buildkit/executor/oci"
	"github.com/moby/buildkit/snapshot"
	"github.com/moby/buildkit/solver/pb"
	"github.com/moby/buildkit/util/network"
	"github.com/moby/sys/user"
	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/pkg/errors"
)

func getUserSpec(user, _ string) (specs.User, error) {
	return specs.User{
		Username: user,
	}, nil
}

func (w *containerdExecutor) prepareExecutionEnv(ctx context.Context, rootMount executor.Mount, _ []executor.Mount, _ executor.Meta, details *containerState, _ pb.NetMode) (string, string, func(), error) {
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

func (w *containerdExecutor) ensureCWD(details *containerState, meta executor.Meta) (err error) {
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
		// uid and gid are not used on windows.
		if err := user.MkdirAllAndChown(newp, 0755, 0, 0); err != nil {
			return errors.Wrapf(err, "failed to create working directory %s", newp)
		}
	}
	return nil
}

func (w *containerdExecutor) createOCISpec(ctx context.Context, id, _, _ string, namespace network.Namespace, mounts []executor.Mount, meta executor.Meta, _ *containerState) (*specs.Spec, func(), error) {
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
	spec, cleanup, err := oci.GenerateSpec(ctx, meta, mounts, id, "", "", namespace, "", processMode, nil, "", false, w.traceSocket, nil, opts...)
	if err != nil {
		releaseAll()
		return nil, nil, err
	}
	releasers = append(releasers, cleanup)
	return spec, releaseAll, nil
}

func (d *containerState) getTaskOpts() ([]ctd.NewTaskOpts, error) {
	return []ctd.NewTaskOpts{ctd.WithRootFS(d.rootMounts)}, nil
}

func setArgs(spec *specs.Process, args []string) {
	spec.CommandLine = strings.Join(args, " ")
}
