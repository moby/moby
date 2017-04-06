package controlapi

import (
	"net"

	"github.com/docker/docker/pkg/plugingetter"
	"github.com/docker/libnetwork/driverapi"
	"github.com/docker/libnetwork/ipamapi"
	"github.com/docker/swarmkit/api"
	"github.com/docker/swarmkit/identity"
	"github.com/docker/swarmkit/manager/allocator"
	"github.com/docker/swarmkit/manager/state/store"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
)

func validateIPAMConfiguration(ipamConf *api.IPAMConfig) error {
	if ipamConf == nil {
		return grpc.Errorf(codes.InvalidArgument, "ipam configuration: cannot be empty")
	}

	_, subnet, err := net.ParseCIDR(ipamConf.Subnet)
	if err != nil {
		return grpc.Errorf(codes.InvalidArgument, "ipam configuration: invalid subnet %s", ipamConf.Subnet)
	}

	if ipamConf.Range != "" {
		ip, _, err := net.ParseCIDR(ipamConf.Range)
		if err != nil {
			return grpc.Errorf(codes.InvalidArgument, "ipam configuration: invalid range %s", ipamConf.Range)
		}

		if !subnet.Contains(ip) {
			return grpc.Errorf(codes.InvalidArgument, "ipam configuration: subnet %s does not contain range %s", ipamConf.Subnet, ipamConf.Range)
		}
	}

	if ipamConf.Gateway != "" {
		ip := net.ParseIP(ipamConf.Gateway)
		if ip == nil {
			return grpc.Errorf(codes.InvalidArgument, "ipam configuration: invalid gateway %s", ipamConf.Gateway)
		}

		if !subnet.Contains(ip) {
			return grpc.Errorf(codes.InvalidArgument, "ipam configuration: subnet %s does not contain gateway %s", ipamConf.Subnet, ipamConf.Gateway)
		}
	}

	return nil
}

func validateIPAM(ipam *api.IPAMOptions, pg plugingetter.PluginGetter) error {
	if ipam == nil {
		// It is ok to not specify any IPAM configurations. We
		// will choose good defaults.
		return nil
	}

	if err := validateDriver(ipam.Driver, pg, ipamapi.PluginEndpointType); err != nil {
		return err
	}

	for _, ipamConf := range ipam.Configs {
		if err := validateIPAMConfiguration(ipamConf); err != nil {
			return err
		}
	}

	return nil
}

func validateNetworkSpec(spec *api.NetworkSpec, pg plugingetter.PluginGetter) error {
	if spec == nil {
		return grpc.Errorf(codes.InvalidArgument, errInvalidArgument.Error())
	}

	if spec.Ingress && spec.DriverConfig != nil && spec.DriverConfig.Name != "overlay" {
		return grpc.Errorf(codes.Unimplemented, "only overlay driver is currently supported for ingress network")
	}

	if spec.Attachable && spec.Ingress {
		return grpc.Errorf(codes.InvalidArgument, "ingress network cannot be attachable")
	}

	if err := validateAnnotations(spec.Annotations); err != nil {
		return err
	}

	if err := validateDriver(spec.DriverConfig, pg, driverapi.NetworkPluginEndpointType); err != nil {
		return err
	}

	if err := validateIPAM(spec.IPAM, pg); err != nil {
		return err
	}

	return nil
}

// CreateNetwork creates and returns a Network based on the provided NetworkSpec.
// - Returns `InvalidArgument` if the NetworkSpec is malformed.
// - Returns an error if the creation fails.
func (s *Server) CreateNetwork(ctx context.Context, request *api.CreateNetworkRequest) (*api.CreateNetworkResponse, error) {
	if err := validateNetworkSpec(request.Spec, s.pg); err != nil {
		return nil, err
	}

	// TODO(mrjana): Consider using `Name` as a primary key to handle
	// duplicate creations. See #65
	n := &api.Network{
		ID:   identity.NewID(),
		Spec: *request.Spec,
	}

	err := s.store.Update(func(tx store.Tx) error {
		if request.Spec.Ingress {
			if n, err := allocator.GetIngressNetwork(s.store); err == nil {
				return grpc.Errorf(codes.AlreadyExists, "ingress network (%s) is already present", n.ID)
			} else if err != allocator.ErrNoIngress {
				return grpc.Errorf(codes.Internal, "failed ingress network presence check: %v", err)
			}
		}
		return store.CreateNetwork(tx, n)
	})
	if err != nil {
		return nil, err
	}

	return &api.CreateNetworkResponse{
		Network: n,
	}, nil
}

// GetNetwork returns a Network given a NetworkID.
// - Returns `InvalidArgument` if NetworkID is not provided.
// - Returns `NotFound` if the Network is not found.
func (s *Server) GetNetwork(ctx context.Context, request *api.GetNetworkRequest) (*api.GetNetworkResponse, error) {
	if request.NetworkID == "" {
		return nil, grpc.Errorf(codes.InvalidArgument, errInvalidArgument.Error())
	}

	var n *api.Network
	s.store.View(func(tx store.ReadTx) {
		n = store.GetNetwork(tx, request.NetworkID)
	})
	if n == nil {
		return nil, grpc.Errorf(codes.NotFound, "network %s not found", request.NetworkID)
	}
	return &api.GetNetworkResponse{
		Network: n,
	}, nil
}

// RemoveNetwork removes a Network referenced by NetworkID.
// - Returns `InvalidArgument` if NetworkID is not provided.
// - Returns `NotFound` if the Network is not found.
// - Returns an error if the deletion fails.
func (s *Server) RemoveNetwork(ctx context.Context, request *api.RemoveNetworkRequest) (*api.RemoveNetworkResponse, error) {
	if request.NetworkID == "" {
		return nil, grpc.Errorf(codes.InvalidArgument, errInvalidArgument.Error())
	}

	var (
		n  *api.Network
		rm = s.removeNetwork
	)

	s.store.View(func(tx store.ReadTx) {
		n = store.GetNetwork(tx, request.NetworkID)
	})
	if n == nil {
		return nil, grpc.Errorf(codes.NotFound, "network %s not found", request.NetworkID)
	}

	if allocator.IsIngressNetwork(n) {
		rm = s.removeIngressNetwork
	}

	if err := rm(n.ID); err != nil {
		if err == store.ErrNotExist {
			return nil, grpc.Errorf(codes.NotFound, "network %s not found", request.NetworkID)
		}
		return nil, err
	}
	return &api.RemoveNetworkResponse{}, nil
}

func (s *Server) removeNetwork(id string) error {
	return s.store.Update(func(tx store.Tx) error {
		services, err := store.FindServices(tx, store.ByReferencedNetworkID(id))
		if err != nil {
			return grpc.Errorf(codes.Internal, "could not find services using network %s: %v", id, err)
		}

		if len(services) != 0 {
			return grpc.Errorf(codes.FailedPrecondition, "network %s is in use by service %s", id, services[0].ID)
		}

		tasks, err := store.FindTasks(tx, store.ByReferencedNetworkID(id))
		if err != nil {
			return grpc.Errorf(codes.Internal, "could not find tasks using network %s: %v", id, err)
		}

		for _, t := range tasks {
			if t.DesiredState <= api.TaskStateRunning && t.Status.State <= api.TaskStateRunning {
				return grpc.Errorf(codes.FailedPrecondition, "network %s is in use by task %s", id, t.ID)
			}
		}

		return store.DeleteNetwork(tx, id)
	})
}

func (s *Server) removeIngressNetwork(id string) error {
	return s.store.Update(func(tx store.Tx) error {
		services, err := store.FindServices(tx, store.All)
		if err != nil {
			return grpc.Errorf(codes.Internal, "could not find services using network %s: %v", id, err)
		}
		for _, srv := range services {
			if allocator.IsIngressNetworkNeeded(srv) {
				return grpc.Errorf(codes.FailedPrecondition, "ingress network cannot be removed because service %s depends on it", srv.ID)
			}
		}
		return store.DeleteNetwork(tx, id)
	})
}

func filterNetworks(candidates []*api.Network, filters ...func(*api.Network) bool) []*api.Network {
	result := []*api.Network{}

	for _, c := range candidates {
		match := true
		for _, f := range filters {
			if !f(c) {
				match = false
				break
			}
		}
		if match {
			result = append(result, c)
		}
	}

	return result
}

// ListNetworks returns a list of all networks.
func (s *Server) ListNetworks(ctx context.Context, request *api.ListNetworksRequest) (*api.ListNetworksResponse, error) {
	var (
		networks []*api.Network
		err      error
	)

	s.store.View(func(tx store.ReadTx) {
		switch {
		case request.Filters != nil && len(request.Filters.Names) > 0:
			networks, err = store.FindNetworks(tx, buildFilters(store.ByName, request.Filters.Names))
		case request.Filters != nil && len(request.Filters.NamePrefixes) > 0:
			networks, err = store.FindNetworks(tx, buildFilters(store.ByNamePrefix, request.Filters.NamePrefixes))
		case request.Filters != nil && len(request.Filters.IDPrefixes) > 0:
			networks, err = store.FindNetworks(tx, buildFilters(store.ByIDPrefix, request.Filters.IDPrefixes))
		default:
			networks, err = store.FindNetworks(tx, store.All)
		}
	})
	if err != nil {
		return nil, err
	}

	if request.Filters != nil {
		networks = filterNetworks(networks,
			func(e *api.Network) bool {
				return filterContains(e.Spec.Annotations.Name, request.Filters.Names)
			},
			func(e *api.Network) bool {
				return filterContainsPrefix(e.Spec.Annotations.Name, request.Filters.NamePrefixes)
			},
			func(e *api.Network) bool {
				return filterContainsPrefix(e.ID, request.Filters.IDPrefixes)
			},
			func(e *api.Network) bool {
				return filterMatchLabels(e.Spec.Annotations.Labels, request.Filters.Labels)
			},
		)
	}

	return &api.ListNetworksResponse{
		Networks: networks,
	}, nil
}
