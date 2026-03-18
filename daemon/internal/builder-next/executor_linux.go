package buildkit

import (
	"context"
	"os"
	"path/filepath"
	"strconv"

	"github.com/containerd/log"
	"github.com/moby/buildkit/executor"
	"github.com/moby/buildkit/executor/resources"
	"github.com/moby/buildkit/executor/runcexecutor"
	"github.com/moby/buildkit/solver/pb"
	"github.com/moby/buildkit/util/network"
	"github.com/moby/moby/v2/daemon/internal/stringid"
	"github.com/opencontainers/runtime-spec/specs-go"
)

const networkName = "bridge"

func newExecutor(opts executorOpts) (executor.Executor, error) {
	netRoot := filepath.Join(opts.root, "net")
	networkProviders := map[pb.NetMode]network.Provider{
		pb.NetMode_UNSET: &bridgeProvider{Controller: opts.networkController, Root: netRoot},
		pb.NetMode_HOST:  network.NewHostProvider(),
		pb.NetMode_NONE:  network.NewNoneProvider(),
	}

	// make sure net state directory is cleared from previous state
	fis, err := os.ReadDir(netRoot)
	if err == nil {
		for _, fi := range fis {
			fp := filepath.Join(netRoot, fi.Name())
			if err := os.RemoveAll(fp); err != nil {
				log.G(context.TODO()).WithError(err).Errorf("failed to delete old network state: %v", fp)
			}
		}
	}

	// Returning a non-nil but empty *IdentityMapping breaks BuildKit:
	// https://github.com/moby/moby/pull/39444
	idmap := &opts.identityMapping
	if opts.identityMapping.Empty() {
		idmap = nil
	}

	rm, err := resources.NewMonitor()
	if err != nil {
		return nil, err
	}

	// TODO: FIXME: testing env var, replace with something better or remove in a major version or two
	runcCmds := []string{"runc"}
	if runcOverride := os.Getenv("DOCKER_BUILDKIT_RUNC_COMMAND"); runcOverride != "" {
		runcCmds = []string{runcOverride}
	}

	return runcexecutor.New(runcexecutor.Opt{
		Root:                filepath.Join(opts.root, "executor"),
		CommandCandidates:   runcCmds,
		DefaultCgroupParent: opts.cgroupParent,
		Rootless:            opts.rootless,
		NoPivot:             os.Getenv("DOCKER_RAMDISK") != "",
		IdentityMapping:     idmap,
		DNS:                 opts.dnsConfig,
		ApparmorProfile:     opts.apparmorProfile,
		ResourceMonitor:     rm,
		CDIManager:          opts.cdiManager,
	}, networkProviders)
}

// newExecutorGD calls newExecutor() on Linux. It returns a stubExecutor on
// other platforms.
func newExecutorGD(opts executorOpts) (executor.Executor, error) {
	return newExecutor(opts)
}

func (iface *lnInterface) Set(s *specs.Spec) error {
	<-iface.ready
	if iface.err != nil {
		log.G(context.TODO()).WithError(iface.err).Error("failed to set networking spec")
		return iface.err
	}
	shortNetCtlrID := stringid.TruncateID(iface.provider.Controller.ID())
	// attach netns to bridge within the container namespace, using reexec in a prestart hook
	s.Hooks = &specs.Hooks{
		Prestart: []specs.Hook{{
			Path: filepath.Join("/proc", strconv.Itoa(os.Getpid()), "exe"),
			Args: []string{"libnetwork-setkey", "-exec-root=" + iface.provider.Config().ExecRoot, iface.sbx.ContainerID(), shortNetCtlrID},
		}},
	}
	return nil
}
