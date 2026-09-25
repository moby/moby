package buildkit

import (
	"context"
	"encoding/json"
	"path/filepath"

	ctd "github.com/containerd/containerd/v2/client"
	"github.com/containerd/log"
	"github.com/moby/buildkit/executor"
	"github.com/moby/buildkit/executor/containerdexecutor"
	"github.com/moby/buildkit/solver/pb"
	"github.com/moby/buildkit/util/network"
	"github.com/opencontainers/runtime-spec/specs-go"
	"google.golang.org/grpc"
)

const networkName = "nat"

func newExecutor(opts executorOpts) (executor.Executor, network.ProxyProvider, error) {
	netRoot := filepath.Join(opts.root, "net")
	np := map[pb.NetMode]network.Provider{
		pb.NetMode_UNSET: &bridgeProvider{Controller: opts.networkController, Root: netRoot},
		pb.NetMode_NONE:  network.NewNoneProvider(),
	}

	ctdOpts := []ctd.Opt{ctd.WithDefaultNamespace(opts.containerdNamespace)}
	if opts.containerdDialer != nil {
		// Embedded containerd: connect over the in-memory pipe.
		ctdOpts = append(ctdOpts, ctd.WithExtraDialOpts([]grpc.DialOption{
			grpc.WithContextDialer(opts.containerdDialer),
		}))
	}
	client, err := ctd.New(opts.containerdAddr, ctdOpts...)
	if err != nil {
		return nil, nil, err
	}

	return containerdexecutor.New(containerdexecutor.ExecutorOptions{
		Client:           client,
		Root:             opts.root,
		DNSConfig:        opts.dnsConfig,
		CDIManager:       opts.cdiManager,
		NetworkProviders: np,
		ProxyProvider:    opts.proxyProvider,
		HyperVIsolation:  opts.hypervIsolation,
	}), opts.proxyProvider, nil
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
