// +build !windows

package buildkit

import (
	"os"
	"path/filepath"
	"strconv"
	"sync"

	"github.com/docker/libnetwork"
	"github.com/moby/buildkit/executor"
	"github.com/moby/buildkit/executor/runcexecutor"
	"github.com/moby/buildkit/identity"
	"github.com/moby/buildkit/solver/pb"
	"github.com/moby/buildkit/util/network"
	specs "github.com/opencontainers/runtime-spec/specs-go"
)

const networkName = "bridge"

func newExecutor(root, cgroupParent string, net libnetwork.NetworkController) (executor.Executor, error) {
	networkProviders := map[pb.NetMode]network.Provider{
		pb.NetMode_UNSET: &bridgeProvider{NetworkController: net},
		pb.NetMode_HOST:  network.NewHostProvider(),
		pb.NetMode_NONE:  network.NewNoneProvider(),
	}
	return runcexecutor.New(runcexecutor.Opt{
		Root:                filepath.Join(root, "executor"),
		CommandCandidates:   []string{"runc"},
		DefaultCgroupParent: cgroupParent,
	}, networkProviders)
}

type bridgeProvider struct {
	libnetwork.NetworkController
}

func (p *bridgeProvider) New() (network.Namespace, error) {
	n, err := p.NetworkByName(networkName)
	if err != nil {
		return nil, err
	}

	iface := &lnInterface{ready: make(chan struct{}), provider: p}
	iface.Once.Do(func() {
		go iface.init(p.NetworkController, n)
	})

	return iface, nil
}

type lnInterface struct {
	ep  libnetwork.Endpoint
	sbx libnetwork.Sandbox
	sync.Once
	err      error
	ready    chan struct{}
	provider *bridgeProvider
}

func (iface *lnInterface) init(c libnetwork.NetworkController, n libnetwork.Network) {
	defer close(iface.ready)
	id := identity.NewID()

	ep, err := n.CreateEndpoint(id)
	if err != nil {
		iface.err = err
		return
	}

	sbx, err := c.NewSandbox(id)
	if err != nil {
		iface.err = err
		return
	}

	if err := ep.Join(sbx); err != nil {
		iface.err = err
		return
	}

	iface.sbx = sbx
	iface.ep = ep
}

func (iface *lnInterface) Set(s *specs.Spec) {
	<-iface.ready
	if iface.err != nil {
		return
	}
	// attach netns to bridge within the container namespace, using reexec in a prestart hook
	s.Hooks = &specs.Hooks{
		Prestart: []specs.Hook{{
			Path: filepath.Join("/proc", strconv.Itoa(os.Getpid()), "exe"),
			Args: []string{"libnetwork-setkey", iface.sbx.ContainerID(), iface.provider.NetworkController.ID()},
		}},
	}
}

func (iface *lnInterface) Close() error {
	<-iface.ready
	err := iface.sbx.Delete()
	if iface.err != nil {
		// iface.err takes precedence over cleanup errors
		return iface.err
	}
	return err
}
