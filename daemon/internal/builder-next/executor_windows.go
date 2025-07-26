package buildkit

import (
	"context"
	"encoding/json"
	"path/filepath"

	ctd "github.com/containerd/containerd/v2/client"
	"github.com/containerd/log"
	"github.com/moby/buildkit/executor"
	"github.com/moby/buildkit/executor/containerdexecutor"
	"github.com/moby/buildkit/executor/oci"
	"github.com/moby/buildkit/solver/llbsolver/cdidevices"
	"github.com/moby/buildkit/solver/pb"
	"github.com/moby/buildkit/util/network"
	"github.com/moby/moby/v2/daemon/libnetwork"
	"github.com/moby/sys/user"
	"github.com/opencontainers/runtime-spec/specs-go"
)

const networkName = "nat"

func newExecutor(
	root string,
	_ string,
	net *libnetwork.Controller,
	dns *oci.DNSConfig,
	_ bool,
	_ user.IdentityMapping,
	_ string,
	cdiManager *cdidevices.Manager,
	containerdAddr string,
	containerdNamespace string,
) (executor.Executor, error) {
	netRoot := filepath.Join(root, "net")
	np := map[pb.NetMode]network.Provider{
		pb.NetMode_UNSET: &bridgeProvider{Controller: net, Root: netRoot},
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

func (iface *lnInterface) Set(s *specs.Spec) error {
	<-iface.ready
	if iface.err != nil {
		log.G(context.TODO()).WithError(iface.err).Error("failed to set networking spec")
		return iface.err
	}

	allowUnqualifiedDNSQuery := false
	var epList []string
	for _, ep := range iface.sbx.Endpoints() {
		data, err := ep.DriverInfo()
		if err != nil {
			continue
		}

		if data["hnsid"] != nil {
			epList = append(epList, data["hnsid"].(string))
		}

		if data["AllowUnqualifiedDNSQuery"] != nil {
			allowUnqualifiedDNSQuery = true
		}
	}
	if s.Windows == nil {
		s.Windows = &specs.Windows{}
	}
	if s.Windows.Network == nil {
		s.Windows.Network = &specs.WindowsNetwork{}
	}
	s.Windows.Network.EndpointList = epList
	s.Windows.Network.AllowUnqualifiedDNSQuery = allowUnqualifiedDNSQuery

	if b, err := json.Marshal(s); err == nil {
		log.G(context.TODO()).Debugf("Generated spec: %s", string(b))
	}

	return nil
}
