package buildkit

import (
	"context"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"sync"

	"github.com/containerd/log"
	"github.com/docker/docker/daemon/config"
	"github.com/docker/docker/libnetwork"
	"github.com/docker/docker/pkg/idtools"
	"github.com/docker/docker/pkg/stringid"
	"github.com/moby/buildkit/executor"
	"github.com/moby/buildkit/executor/oci"
	"github.com/moby/buildkit/executor/resources"
	resourcestypes "github.com/moby/buildkit/executor/resources/types"
	"github.com/moby/buildkit/executor/runcexecutor"
	"github.com/moby/buildkit/identity"
	"github.com/moby/buildkit/solver/pb"
	"github.com/moby/buildkit/util/network"
	specs "github.com/opencontainers/runtime-spec/specs-go"
)

const networkName = "bridge"

func newExecutor(root, cgroupParent string, net *libnetwork.Controller, dnsConfig *oci.DNSConfig, rootless bool, idmap idtools.IdentityMapping, apparmorProfile string) (executor.Executor, error) {
	netRoot := filepath.Join(root, "net")
	networkProviders := map[pb.NetMode]network.Provider{
		pb.NetMode_UNSET: &bridgeProvider{Controller: net, Root: netRoot},
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
	pidmap := &idmap
	if idmap.Empty() {
		pidmap = nil
	}

	rm, err := resources.NewMonitor()
	if err != nil {
		return nil, err
	}

	runcCmds := []string{"runc"}

	// TODO: FIXME: testing env var, replace with something better or remove in a major version or two
	if runcOverride := os.Getenv("DOCKER_BUILDKIT_RUNC_COMMAND"); runcOverride != "" {
		runcCmds = []string{runcOverride}
	}

	return runcexecutor.New(runcexecutor.Opt{
		Root:                filepath.Join(root, "executor"),
		CommandCandidates:   runcCmds,
		DefaultCgroupParent: cgroupParent,
		Rootless:            rootless,
		NoPivot:             os.Getenv("DOCKER_RAMDISK") != "",
		IdentityMapping:     pidmap,
		DNS:                 dnsConfig,
		ApparmorProfile:     apparmorProfile,
		ResourceMonitor:     rm,
	}, networkProviders)
}

type bridgeProvider struct {
	*libnetwork.Controller
	Root string
}

func (p *bridgeProvider) New(ctx context.Context, hostname string) (network.Namespace, error) {
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

type lnInterface struct {
	ep  *libnetwork.Endpoint
	sbx *libnetwork.Sandbox
	sync.Once
	err      error
	ready    chan struct{}
	provider *bridgeProvider
}

func (iface *lnInterface) init(c *libnetwork.Controller, n *libnetwork.Network) {
	defer close(iface.ready)
	id := identity.NewID()

	ep, err := n.CreateEndpoint(context.TODO(), id, libnetwork.CreateOptionDisableResolution())
	if err != nil {
		iface.err = err
		return
	}

	sbx, err := c.NewSandbox(context.TODO(), id, libnetwork.OptionUseExternalKey(), libnetwork.OptionHostsPath(filepath.Join(iface.provider.Root, id, "hosts")),
		libnetwork.OptionResolvConfPath(filepath.Join(iface.provider.Root, id, "resolv.conf")))
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

func ipAddresses(ips []net.IP) []string {
	var addrs []string
	for _, ip := range ips {
		addrs = append(addrs, ip.String())
	}
	return addrs
}
