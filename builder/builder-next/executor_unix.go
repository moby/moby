//go:build !windows
// +build !windows

package buildkit

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/containerd/containerd"
	"github.com/docker/docker/daemon/config"
	"github.com/docker/docker/libcontainerd/remote"
	"github.com/docker/docker/libnetwork"
	"github.com/moby/buildkit/executor"
	"github.com/moby/buildkit/executor/containerdexecutor"
	"github.com/moby/buildkit/executor/oci"
	"github.com/moby/buildkit/executor/runcexecutor"
	"github.com/moby/buildkit/identity"
	"github.com/moby/buildkit/solver/pb"
	"github.com/moby/buildkit/util/network"
	specs "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/sirupsen/logrus"
)

const networkName = "bridge"

func newExecutor(opt Opt) (executor.Executor, error) {
	netRoot := filepath.Join(opt.Root, "net")
	bridge := &bridgeProvider{Controller: opt.NetworkController, Root: netRoot}
	networkProviders := map[pb.NetMode]network.Provider{
		pb.NetMode_UNSET: bridge,
		pb.NetMode_HOST:  network.NewHostProvider(),
		pb.NetMode_NONE:  network.NewNoneProvider(),
	}

	// make sure net state directory is cleared from previous state
	fis, err := os.ReadDir(netRoot)
	if err == nil {
		for _, fi := range fis {
			fp := filepath.Join(netRoot, fi.Name())
			if err := os.RemoveAll(fp); err != nil {
				logrus.WithError(err).Errorf("failed to delete old network state: %v", fp)
			}
		}
	}

	// Returning a non-nil but empty *IdentityMapping breaks BuildKit:
	// https://github.com/moby/moby/pull/39444
	pidmap := &opt.IdentityMapping
	if pidmap.Empty() {
		pidmap = nil
	}

	executorRoot := filepath.Join(opt.Root, "executor")
	dnsConfig := getDNSConfig(opt.DNSConfig)

	if pidmap != nil {
		// A container cannot be created which both reuses an existing network namespace
		// and uses identity mapping (i.e. user namespaces). When identity mapping is
		// used, we have to defer creation of the network namespace to the runtime.
		// https://github.com/moby/moby/pull/44385#issuecomment-1300619182 The runtime
		// creates the namespace during the 'create' OCI lifecycle operation, and
		// libnetwork has to configure the namespace before the 'start' OCI operation.
		// Buildkit's runc executor uses the 'runc run' command, which combines the
		// 'create' and 'start' OCI lifecycle operations. We would have to use an OCI
		// createRuntime hook to configure the network namespace.
		// https://github.com/opencontainers/runtime-spec/blob/main/config.md#createRuntime-hooks
		// OCI hooks are processes invoked by the runtime, so a bunch of dedicated
		// multi-call binary (i.e. having the daemon binary behave differently depending
		// on argv[0], like busybox) and IPC infrastructure would need to be maintained
		// in order to wire the hook up to libnetwork. Instead use buildkit's containerd
		// executor when identity-mapping is required as it supports in-process hooks in
		// the form of OnCreateRuntime callbacks for network.Providers.
		bridge.UseExternalKey = true
		client, err := remote.NewContainerdClient(opt.ContainerdAddr, containerd.WithDefaultNamespace(opt.ContainerdNamespace))
		if err != nil {
			return nil, err
		}
		// The runc executor does this itself, but not the containerd executor.
		if err := os.MkdirAll(executorRoot, 0o711); err != nil {
			return nil, err
		}
		executorRoot, err = filepath.Abs(executorRoot)
		if err != nil {
			return nil, err
		}
		executorRoot, err = filepath.EvalSymlinks(executorRoot)
		if err != nil {
			return nil, err
		}
		return containerdexecutor.New(
			client,
			executorRoot,
			opt.DefaultCgroupParent,
			networkProviders,
			dnsConfig,
			opt.ApparmorProfile,
			false,
			"",
			opt.Rootless,
		), nil
	}

	return runcexecutor.New(runcexecutor.Opt{
		Root:                executorRoot,
		CommandCandidates:   []string{"runc"},
		DefaultCgroupParent: opt.DefaultCgroupParent,
		Rootless:            opt.Rootless,
		NoPivot:             os.Getenv("DOCKER_RAMDISK") != "",
		IdentityMapping:     pidmap,
		DNS:                 dnsConfig,
		ApparmorProfile:     opt.ApparmorProfile,
	}, networkProviders)
}

type bridgeProvider struct {
	*libnetwork.Controller
	Root           string
	UseExternalKey bool
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

func (iface *lnInterface) init(c *libnetwork.Controller, n libnetwork.Network) {
	defer close(iface.ready)
	id := identity.NewID()

	ep, err := n.CreateEndpoint(id, libnetwork.CreateOptionDisableResolution())
	if err != nil {
		iface.err = err
		return
	}

	sbopts := []libnetwork.SandboxOption{
		libnetwork.OptionHostsPath(filepath.Join(iface.provider.Root, id, "hosts")),
		libnetwork.OptionResolvConfPath(filepath.Join(iface.provider.Root, id, "resolv.conf")),
	}
	if iface.provider.UseExternalKey {
		sbopts = append(sbopts, libnetwork.OptionUseExternalKey())
	}
	sbx, err := c.NewSandbox(id, sbopts...)
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

	if iface.provider.UseExternalKey {
		return nil
	}

	for i, n := range s.Linux.Namespaces {
		if n.Type == specs.NetworkNamespace {
			s.Linux.Namespaces[i].Path = iface.sbx.Key()
			return nil
		}
	}
	s.Linux.Namespaces = append(s.Linux.Namespaces, specs.LinuxNamespace{
		Type: specs.NetworkNamespace,
		Path: iface.sbx.Key(),
	})
	return nil
}

var _ containerdexecutor.OnCreateRuntimer = (*lnInterface)(nil)

func (iface *lnInterface) OnCreateRuntime(pid uint32) error {
	<-iface.ready
	if iface.err != nil {
		return iface.err
	}

	if !iface.provider.UseExternalKey {
		return nil
	}

	return iface.sbx.SetKey(fmt.Sprintf("/proc/%d/ns/net", pid))
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
