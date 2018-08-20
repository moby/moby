// +build !windows

package buildkit

import (
	"fmt"
	"path/filepath"
	"sync"

	"github.com/docker/libnetwork"
	"github.com/moby/buildkit/executor"
	"github.com/moby/buildkit/executor/runcexecutor"
	"github.com/moby/buildkit/identity"
	"github.com/moby/buildkit/util/network"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

const networkName = "bridge"

func init() {
	// FIXME: https://github.com/moby/moby/issues/37676
	runcexecutor.DisableSubReaper()
}

func newExecutor(root string, net libnetwork.NetworkController) (executor.Executor, error) {
	return runcexecutor.New(runcexecutor.Opt{
		Root:              filepath.Join(root, "executor"),
		CommandCandidates: []string{"docker-runc", "runc"},
	}, &bridgeProvider{NetworkController: net})
}

type bridgeProvider struct {
	libnetwork.NetworkController
}

func (p *bridgeProvider) NewInterface() (network.Interface, error) {
	n, err := p.NetworkByName(networkName)
	if err != nil {
		return nil, err
	}

	iface := &lnInterface{ready: make(chan struct{})}
	iface.Once.Do(func() {
		go iface.init(p.NetworkController, n)
	})

	return iface, nil
}

func (p *bridgeProvider) Release(iface network.Interface) error {
	go func() {
		if err := p.release(iface); err != nil {
			logrus.Errorf("%s", err)
		}
	}()
	return nil
}

func (p *bridgeProvider) release(iface network.Interface) error {
	li, ok := iface.(*lnInterface)
	if !ok {
		return errors.Errorf("invalid interface %T", iface)
	}
	err := li.sbx.Delete()
	if err1 := li.ep.Delete(true); err1 != nil && err == nil {
		err = err1
	}
	return err
}

type lnInterface struct {
	ep  libnetwork.Endpoint
	sbx libnetwork.Sandbox
	sync.Once
	err   error
	ready chan struct{}
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

func (iface *lnInterface) Set(pid int) error {
	<-iface.ready
	if iface.err != nil {
		return iface.err
	}
	return iface.sbx.SetKey(fmt.Sprintf("/proc/%d/ns/net", pid))
}

func (iface *lnInterface) Remove(pid int) error {
	return nil
}
