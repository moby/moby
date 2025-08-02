package buildkit

import (
	"context"
	"net/netip"
	"os"
	"path/filepath"
	"sync"

	"github.com/containerd/log"
	"github.com/moby/buildkit/executor/oci"
	resourcestypes "github.com/moby/buildkit/executor/resources/types"
	"github.com/moby/buildkit/identity"
	"github.com/moby/buildkit/util/network"
	"github.com/moby/moby/v2/daemon/config"
	"github.com/moby/moby/v2/daemon/libnetwork"
)

type bridgeProvider struct {
	*libnetwork.Controller
	Root string
}

type lnInterface struct {
	ep  *libnetwork.Endpoint
	sbx *libnetwork.Sandbox
	sync.Once
	err      error
	ready    chan struct{}
	provider *bridgeProvider
}

func (p *bridgeProvider) New(_ context.Context, _ string) (network.Namespace, error) {
	n, err := p.NetworkByName(networkName)
	if err != nil {
		return nil, err
	}

	iface := &lnInterface{ready: make(chan struct{}), provider: p}
	iface.Once.Do(func() {
		go iface.init(p.Controller, n)
	})

	return iface, nil
}

func (p *bridgeProvider) Close() error {
	return nil
}

func (iface *lnInterface) init(c *libnetwork.Controller, n *libnetwork.Network) {
	defer close(iface.ready)
	id := identity.NewID()

	ep, err := n.CreateEndpoint(context.TODO(), id, libnetwork.CreateOptionDisableResolution())
	if err != nil {
		iface.err = err
		return
	}

	sbx, err := c.NewSandbox(
		context.TODO(),
		id,
		libnetwork.OptionUseExternalKey(),
		libnetwork.OptionHostsPath(filepath.Join(iface.provider.Root, id, "hosts")),
		libnetwork.OptionResolvConfPath(filepath.Join(iface.provider.Root, id, "resolv.conf")),
	)
	if err != nil {
		iface.err = err
		return
	}

	if err := ep.Join(context.TODO(), sbx); err != nil {
		iface.err = err
		return
	}

	iface.sbx = sbx
	iface.ep = ep
}

// TODO(neersighted): Unstub Sample(), and collect data from the libnetwork Endpoint.
func (iface *lnInterface) Sample() (*resourcestypes.NetworkSample, error) {
	return &resourcestypes.NetworkSample{}, nil
}

func (iface *lnInterface) Close() error {
	<-iface.ready
	if iface.sbx != nil {
		go func() {
			if err := iface.sbx.Delete(context.TODO()); err != nil {
				log.G(context.TODO()).WithError(err).Errorf("failed to delete builder network sandbox")
			}
			if err := os.RemoveAll(filepath.Join(iface.provider.Root, iface.sbx.ContainerID())); err != nil {
				log.G(context.TODO()).WithError(err).Errorf("failed to delete builder sandbox directory")
			}
		}()
	}
	return iface.err
}

func getDNSConfig(cfg config.DNSConfig) *oci.DNSConfig {
	if cfg.DNS != nil || cfg.DNSSearch != nil || cfg.DNSOptions != nil {
		return &oci.DNSConfig{
			Nameservers:   ipAddresses(cfg.DNS),
			SearchDomains: cfg.DNSSearch,
			Options:       cfg.DNSOptions,
		}
	}
	return nil
}

func ipAddresses(ips []netip.Addr) []string {
	var addrs []string
	for _, ip := range ips {
		addrs = append(addrs, ip.String())
	}
	return addrs
}
