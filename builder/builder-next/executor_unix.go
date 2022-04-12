//go:build !windows
// +build !windows

package buildkit

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"sync"

	"github.com/docker/docker/daemon/config"
	"github.com/docker/docker/pkg/idtools"
	"github.com/docker/docker/pkg/stringid"
	"github.com/docker/libnetwork"
	"github.com/moby/buildkit/executor"
	"github.com/moby/buildkit/executor/oci"
	"github.com/moby/buildkit/executor/runcexecutor"
	"github.com/moby/buildkit/identity"
	"github.com/moby/buildkit/solver/pb"
	"github.com/moby/buildkit/util/network"
	specs "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/sirupsen/logrus"
)

const networkName = "bridge"

func newExecutor(root, cgroupParent string, net libnetwork.NetworkController, dnsConfig *oci.DNSConfig, rootless bool, idmap *idtools.IdentityMapping, apparmorProfile string) (executor.Executor, error) {
	netRoot := filepath.Join(root, "net")
	networkProviders := map[pb.NetMode]network.Provider{
		pb.NetMode_UNSET: &bridgeProvider{NetworkController: net, Root: netRoot},
		pb.NetMode_HOST:  network.NewHostProvider(),
		pb.NetMode_NONE:  network.NewNoneProvider(),
	}

	// make sure net state directory is cleared from previous state
	fis, err := ioutil.ReadDir(netRoot)
	if err == nil {
		for _, fi := range fis {
			fp := filepath.Join(netRoot, fi.Name())
			if err := os.RemoveAll(fp); err != nil {
				logrus.WithError(err).Errorf("failed to delete old network state: %v", fp)
			}
		}
	}

	return runcexecutor.New(runcexecutor.Opt{
		Root:                filepath.Join(root, "executor"),
		CommandCandidates:   []string{"runc"},
		DefaultCgroupParent: cgroupParent,
		Rootless:            rootless,
		NoPivot:             os.Getenv("DOCKER_RAMDISK") != "",
		IdentityMapping:     idmap,
		DNS:                 dnsConfig,
		ApparmorProfile:     apparmorProfile,
	}, networkProviders)
}

type bridgeProvider struct {
	libnetwork.NetworkController
	Root string
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

	ep, err := n.CreateEndpoint(id, libnetwork.CreateOptionDisableResolution())
	if err != nil {
		iface.err = err
		return
	}

	sbx, err := c.NewSandbox(id, libnetwork.OptionUseExternalKey(), libnetwork.OptionHostsPath(filepath.Join(iface.provider.Root, id, "hosts")),
		libnetwork.OptionResolvConfPath(filepath.Join(iface.provider.Root, id, "resolv.conf")))
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

func (iface *lnInterface) Set(s *specs.Spec) error {
	<-iface.ready
	if iface.err != nil {
		logrus.WithError(iface.err).Error("failed to set networking spec")
		return iface.err
	}
	shortNetCtlrID := stringid.TruncateID(iface.provider.NetworkController.ID())
	// attach netns to bridge within the container namespace, using reexec in a prestart hook
	s.Hooks = &specs.Hooks{
		Prestart: []specs.Hook{{
			Path: filepath.Join("/proc", strconv.Itoa(os.Getpid()), "exe"),
			Args: []string{"libnetwork-setkey", "-exec-root=" + iface.provider.Config().Daemon.ExecRoot, iface.sbx.ContainerID(), shortNetCtlrID},
		}},
	}
	return nil
}

func (iface *lnInterface) Close() error {
	<-iface.ready
	if iface.sbx != nil {
		go func() {
			if err := iface.sbx.Delete(); err != nil {
				logrus.WithError(err).Errorf("failed to delete builder network sandbox")
			}
			if err := os.RemoveAll(filepath.Join(iface.provider.Root, iface.sbx.ContainerID())); err != nil {
				logrus.WithError(err).Errorf("failed to delete builder sandbox directory")
			}
		}()
	}
	return iface.err
}

func getDNSConfig(cfg config.DNSConfig) *oci.DNSConfig {
	if cfg.DNS != nil || cfg.DNSSearch != nil || cfg.DNSOptions != nil {
		return &oci.DNSConfig{
			Nameservers:   cfg.DNS,
			SearchDomains: cfg.DNSSearch,
			Options:       cfg.DNSOptions,
		}
	}
	return nil
}
