package convert

import (
	"maps"
	"net/netip"
	"strings"
	"time"

	gogotypes "github.com/gogo/protobuf/types"
	"github.com/moby/moby/api/types/network"
	types "github.com/moby/moby/api/types/swarm"
	"github.com/moby/moby/v2/daemon/cluster/convert/netextra"
	"github.com/moby/moby/v2/daemon/internal/netipstringer"
	"github.com/moby/moby/v2/daemon/internal/netiputil"
	"github.com/moby/moby/v2/daemon/libnetwork/scope"
	"github.com/moby/moby/v2/internal/iterutil"
	"github.com/moby/moby/v2/internal/sliceutil"
	swarmapi "github.com/moby/swarmkit/v2/api"
)

func networkAttachmentFromGRPC(na *swarmapi.NetworkAttachment) types.NetworkAttachment {
	if na != nil {
		return types.NetworkAttachment{
			Network:   networkFromGRPC(na.Network),
			Addresses: sliceutil.Map(na.Addresses, func(s string) netip.Prefix { a, _ := netip.ParsePrefix(s); return a }),
		}
	}
	return types.NetworkAttachment{}
}

func networkFromGRPC(n *swarmapi.Network) types.Network {
	if n != nil {
		nw := types.Network{
			ID: n.ID,
			Spec: types.NetworkSpec{
				IPv6Enabled: n.Spec.Ipv6Enabled,
				Internal:    n.Spec.Internal,
				Attachable:  n.Spec.Attachable,
				Ingress:     IsIngressNetwork(n),
				IPAMOptions: ipamFromGRPC(n.Spec.IPAM),
				Scope:       scope.Swarm,
			},
			IPAMOptions: ipamFromGRPC(n.IPAM),
		}

		if n.Spec.GetNetwork() != "" {
			nw.Spec.ConfigFrom = &network.ConfigReference{
				Network: n.Spec.GetNetwork(),
			}
		}

		// Meta
		nw.Version.Index = n.Meta.Version.Index
		nw.CreatedAt, _ = gogotypes.TimestampFromProto(n.Meta.CreatedAt)
		nw.UpdatedAt, _ = gogotypes.TimestampFromProto(n.Meta.UpdatedAt)

		// Annotations
		nw.Spec.Annotations = annotationsFromGRPC(n.Spec.Annotations)

		// DriverConfiguration
		if n.Spec.DriverConfig != nil {
			nw.Spec.DriverConfiguration = &types.Driver{
				Name:    n.Spec.DriverConfig.Name,
				Options: n.Spec.DriverConfig.Options,
			}
		}

		// DriverState
		if n.DriverState != nil {
			nw.DriverState = types.Driver{
				Name:    n.DriverState.Name,
				Options: n.DriverState.Options,
			}
		}

		return nw
	}
	return types.Network{}
}

func ipamFromGRPC(i *swarmapi.IPAMOptions) *types.IPAMOptions {
	var ipam *types.IPAMOptions
	if i != nil {
		ipam = &types.IPAMOptions{}
		if i.Driver != nil {
			ipam.Driver.Name = i.Driver.Name
			ipam.Driver.Options = i.Driver.Options
		}

		for _, config := range i.Configs {
			// Best-effort parse of user suppplied values that have
			// been round-tripped through Swarm's Raft store. It is
			// far too late to reject bogus values.
			subnet, _ := netiputil.ParseCIDR(config.Subnet)
			iprange, _ := netiputil.ParseCIDR(config.Range)
			gw, _ := netip.ParseAddr(config.Gateway)
			ipam.Configs = append(ipam.Configs, types.IPAMConfig{
				Subnet:  subnet.Masked(),
				Range:   iprange.Masked(),
				Gateway: gw.Unmap(),
			})
		}
	}
	return ipam
}

func endpointSpecFromGRPC(es *swarmapi.EndpointSpec) *types.EndpointSpec {
	var endpointSpec *types.EndpointSpec
	if es != nil {
		endpointSpec = &types.EndpointSpec{
			Mode: types.ResolutionMode(strings.ToLower(es.Mode.String())),
		}

		for _, portState := range es.Ports {
			endpointSpec.Ports = append(endpointSpec.Ports, swarmPortConfigToAPIPortConfig(portState))
		}
	}
	return endpointSpec
}

func endpointFromGRPC(e *swarmapi.Endpoint) types.Endpoint {
	endpoint := types.Endpoint{}
	if e != nil {
		if espec := endpointSpecFromGRPC(e.Spec); espec != nil {
			endpoint.Spec = *espec
		}

		for _, portState := range e.Ports {
			endpoint.Ports = append(endpoint.Ports, swarmPortConfigToAPIPortConfig(portState))
		}

		for _, v := range e.VirtualIPs {
			vip, _ := netip.ParsePrefix(v.Addr)
			endpoint.VirtualIPs = append(endpoint.VirtualIPs, types.EndpointVirtualIP{
				NetworkID: v.NetworkID,
				Addr:      vip,
			})
		}
	}

	return endpoint
}

func swarmPortConfigToAPIPortConfig(portConfig *swarmapi.PortConfig) types.PortConfig {
	return types.PortConfig{
		Name:          portConfig.Name,
		Protocol:      network.IPProtocol(strings.ToLower(swarmapi.PortConfig_Protocol_name[int32(portConfig.Protocol)])),
		PublishMode:   types.PortConfigPublishMode(strings.ToLower(swarmapi.PortConfig_PublishMode_name[int32(portConfig.PublishMode)])),
		TargetPort:    portConfig.TargetPort,
		PublishedPort: portConfig.PublishedPort,
	}
}

// BasicNetworkFromGRPC converts a grpc Network to a NetworkResource.
func BasicNetworkFromGRPC(n swarmapi.Network) network.Network {
	spec := n.Spec
	var ipam network.IPAM
	if n.IPAM != nil {
		if n.IPAM.Driver != nil {
			ipam.Driver = n.IPAM.Driver.Name
			ipam.Options = n.IPAM.Driver.Options
		}
		ipam.Config = make([]network.IPAMConfig, 0, len(n.IPAM.Configs))
		for _, ic := range n.IPAM.Configs {
			// Best-effort parse of user suppplied values that have
			// been round-tripped through Swarm's Raft store. It is
			// far too late to reject bogus values.
			subnet, _ := netiputil.ParseCIDR(ic.Subnet)
			iprange, _ := netiputil.ParseCIDR(ic.Range)
			gw, _ := netip.ParseAddr(ic.Gateway)
			cfg := network.IPAMConfig{
				Subnet:  subnet.Masked(),
				IPRange: iprange.Masked(),
				Gateway: gw.Unmap(),
			}
			cfg.AuxAddress = maps.Collect(iterutil.Map2(maps.All(ic.Reserved), func(k, v string) (string, netip.Addr) {
				addr, _ := netip.ParseAddr(v)
				return k, addr.Unmap()
			}))
			ipam.Config = append(ipam.Config, cfg)
		}
	}

	nr := network.Network{
		ID:         n.ID,
		Name:       n.Spec.Annotations.Name,
		Scope:      scope.Swarm,
		EnableIPv4: true,
		EnableIPv6: spec.Ipv6Enabled,
		IPAM:       ipam,
		Internal:   spec.Internal,
		Attachable: spec.Attachable,
		Ingress:    IsIngressNetwork(&n),
		Labels:     n.Spec.Annotations.Labels,
	}
	nr.Created, _ = gogotypes.TimestampFromProto(n.Meta.CreatedAt)

	if n.Spec.GetNetwork() != "" {
		nr.ConfigFrom = network.ConfigReference{
			Network: n.Spec.GetNetwork(),
		}
	}

	if n.DriverState != nil {
		nr.Driver = n.DriverState.Name
		nr.Options = n.DriverState.Options
	}

	return nr
}

func NetworkInspectFromGRPC(n swarmapi.Network) (network.Inspect, error) {
	ni := network.Inspect{
		Network:    BasicNetworkFromGRPC(n),
		Containers: make(map[string]network.EndpointResource),
	}

	var err error
	ni.Status, err = netextra.StatusFrom(n.Extra)
	return ni, err
}

// BasicNetworkCreateToGRPC converts a NetworkCreateRequest to a grpc NetworkSpec.
func BasicNetworkCreateToGRPC(create network.CreateRequest) swarmapi.NetworkSpec {
	ns := swarmapi.NetworkSpec{
		Annotations: swarmapi.Annotations{
			Name:   create.Name,
			Labels: create.Labels,
		},
		DriverConfig: &swarmapi.Driver{
			Name:    create.Driver,
			Options: create.Options,
		},
		Internal:   create.Internal,
		Attachable: create.Attachable,
		Ingress:    create.Ingress,
	}
	if create.EnableIPv6 != nil {
		ns.Ipv6Enabled = *create.EnableIPv6
	}
	if create.IPAM != nil {
		driver := create.IPAM.Driver
		if driver == "" {
			driver = "default"
		}
		ns.IPAM = &swarmapi.IPAMOptions{
			Driver: &swarmapi.Driver{
				Name:    driver,
				Options: create.IPAM.Options,
			},
		}
		ipamSpec := make([]*swarmapi.IPAMConfig, 0, len(create.IPAM.Config))
		for _, ipamConfig := range create.IPAM.Config {
			ipamSpec = append(ipamSpec, &swarmapi.IPAMConfig{
				Subnet:  netipstringer.Prefix(netiputil.Unmap(ipamConfig.Subnet).Masked()),
				Range:   netipstringer.Prefix(netiputil.Unmap(ipamConfig.IPRange).Masked()),
				Gateway: netipstringer.Addr(ipamConfig.Gateway.Unmap()),
			})
		}
		ns.IPAM.Configs = ipamSpec
	}
	if create.ConfigFrom != nil {
		ns.ConfigFrom = &swarmapi.NetworkSpec_Network{
			Network: create.ConfigFrom.Network,
		}
	}
	return ns
}

// IsIngressNetwork check if the swarm network is an ingress network
func IsIngressNetwork(n *swarmapi.Network) bool {
	if n.Spec.Ingress {
		return true
	}
	// Check if legacy defined ingress network
	_, ok := n.Spec.Annotations.Labels["com.docker.swarm.internal"]
	return ok && n.Spec.Annotations.Name == "ingress"
}

// FilterNetwork adapts [swarmapi.Network] to the
// [github.com/moby/moby/v2/daemon/network.FilterNetwork] interface.
type FilterNetwork struct {
	N *swarmapi.Network
}

func (nw FilterNetwork) ID() string {
	return nw.N.ID
}

func (nw FilterNetwork) Name() string {
	return nw.N.Spec.Annotations.Name
}

func (nw FilterNetwork) Driver() string {
	if nw.N.DriverState != nil {
		return nw.N.DriverState.Name
	}
	return ""
}

func (nw FilterNetwork) Labels() map[string]string {
	return nw.N.Spec.Annotations.Labels
}

func (nw FilterNetwork) Scope() string {
	return scope.Swarm
}

func (nw FilterNetwork) Created() time.Time {
	t, _ := gogotypes.TimestampFromProto(nw.N.Meta.CreatedAt)
	return t
}

func (nw FilterNetwork) HasContainerAttachments() bool {
	// Not tracked in swarmkit
	return false
}

func (nw FilterNetwork) HasServiceAttachments() bool {
	// Not tracked in swarmkit
	return false
}
