package buildkit

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/containerd/containerd/v2/client"
	"github.com/containerd/log"
	"github.com/moby/buildkit/executor"
	"github.com/moby/buildkit/executor/containerdexecutor"
	"github.com/moby/buildkit/executor/resources"
	"github.com/moby/buildkit/executor/runcexecutor"
	"github.com/moby/buildkit/solver/pb"
	"github.com/moby/buildkit/util/network"
	"github.com/opencontainers/runtime-spec/specs-go"
)

const networkName = "bridge"

func newExecutor(opts executorOpts) (executor.Executor, error) {
	client, err := client.New(opts.containerdAddr, client.WithDefaultNamespace(opts.containerdNamespace))
	if err != nil {
		return nil, err
	}

	return containerdexecutor.New(containerdexecutor.ExecutorOptions{
		Client:           client,
		Root:             opts.root,
		CgroupParent:     opts.cgroupParent,
		NetworkProviders: getNetworkProviders(opts),
		DNSConfig:        opts.dnsConfig,
		ApparmorProfile:  opts.apparmorProfile,
		Rootless:         opts.rootless,
		CDIManager:       opts.cdiManager,
	}), nil
}

func getNetworkProviders(opts executorOpts) map[pb.NetMode]network.Provider {
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

	return networkProviders
}

func newRuncExecutor(opts executorOpts) (executor.Executor, error) {
	networkProviders := getNetworkProviders(opts)

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

// newExecutorGD uses the runc executor for the graphdriver worker. It returns
// a stubExecutor on other platforms.
func newExecutorGD(opts executorOpts) (executor.Executor, error) {
	return newRuncExecutor(opts)
}

func (iface *lnInterface) Set(s *specs.Spec) error {
	<-iface.ready
	if iface.err != nil {
		log.G(context.TODO()).WithError(iface.err).Error("failed to set networking spec")
		return iface.err
	}
	nsPath, ok := iface.sbx.NetnsPath()
	if !ok {
		return fmt.Errorf("buildkit sandbox %s has no network namespace", iface.sbx.ContainerID())
	}
	// Tell runc to join the daemon-owned netns instead of creating a new one.
	// This replaces the previous approach of using a "libnetwork-setkey" reexec
	// prestart hook that bind-mounted /proc/<pid>/ns/net after container creation.
	return setLinuxNamespace(s, specs.LinuxNamespace{
		Type: specs.NetworkNamespace,
		Path: nsPath,
	})
}

// setLinuxNamespace sets or replaces a namespace entry in the OCI spec.
func setLinuxNamespace(s *specs.Spec, ns specs.LinuxNamespace) error {
	for i, n := range s.Linux.Namespaces {
		if n.Type == ns.Type {
			if n.Path != "" {
				return fmt.Errorf("network namespace already set to %s", n.Path)
			}
			s.Linux.Namespaces[i] = ns
			return nil
		}
	}
	s.Linux.Namespaces = append(s.Linux.Namespaces, ns)
	return nil
}
