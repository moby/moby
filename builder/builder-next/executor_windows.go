package buildkit

import (
	ctd "github.com/containerd/containerd/v2/client"
	"github.com/docker/docker/libnetwork"
	"github.com/moby/buildkit/executor"
	"github.com/moby/buildkit/executor/containerdexecutor"
	"github.com/moby/buildkit/executor/oci"
	"github.com/moby/buildkit/solver/llbsolver/cdidevices"
	"github.com/moby/buildkit/solver/pb"
	"github.com/moby/buildkit/util/network"
	"github.com/moby/buildkit/util/network/cniprovider"
	"github.com/moby/sys/user"
)

func newExecutor(
	root string,
	_ string,
	_ *libnetwork.Controller,
	dns *oci.DNSConfig,
	_ bool,
	_ user.IdentityMapping,
	_ string,
	cdiManager *cdidevices.Manager,
	containerdAddr string,
	containerdNamespace string,
	cniOpt cniprovider.Opt,
) (executor.Executor, error) {
	cniProvider, err := cniprovider.New(cniOpt)
	if err != nil {
		return nil, err
	}

	np := map[pb.NetMode]network.Provider{
		pb.NetMode_UNSET: cniProvider,
		pb.NetMode_NONE:  network.NewNoneProvider(),
	}

	opt := ctd.WithDefaultNamespace(containerdNamespace)
	client, err := ctd.New(containerdAddr, opt)
	if err != nil {
		return nil, err
	}

	executorOpts := containerdexecutor.ExecutorOptions{
		Client:           client,
		Root:             root,
		DNSConfig:        dns,
		CDIManager:       cdiManager,
		NetworkProviders: np,
	}
	return containerdexecutor.New(executorOpts), nil
}
