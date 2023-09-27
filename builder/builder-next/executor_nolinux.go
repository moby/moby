//go:build !linux

package buildkit

import (
	"github.com/containerd/containerd"
	"github.com/docker/docker/daemon/config"
	"github.com/docker/docker/libnetwork"
	"github.com/docker/docker/pkg/idtools"
	"github.com/moby/buildkit/executor"
	"github.com/moby/buildkit/executor/containerdexecutor"
	"github.com/moby/buildkit/executor/oci"
	"github.com/moby/buildkit/util/network/netproviders"
)

func newExecutor(root string, containerdClient *containerd.Client, cgroupParent string, net *libnetwork.Controller, dnsConfig *oci.DNSConfig, rootless bool, idmap idtools.IdentityMapping, apparmorProfile string) (executor.Executor, error) {
	nc := netproviders.Opt{
		Mode: "host",
	}
	np, _, err := netproviders.Providers(nc)
	if err != nil {
		return nil, err
	}

	return containerdexecutor.New(containerdClient, root, cgroupParent, np, dnsConfig, apparmorProfile, false, "", false, nil), nil
}

func getDNSConfig(config.DNSConfig) *oci.DNSConfig {
	return nil
}
