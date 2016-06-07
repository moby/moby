package controlapi

import (
	"github.com/docker/swarmkit/api"
	"github.com/docker/swarmkit/identity"
	"github.com/docker/swarmkit/manager/state/store"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
)

func validateNodeSpec(spec *api.NodeSpec) error {
	if spec == nil {
		return grpc.Errorf(codes.InvalidArgument, errInvalidArgument.Error())
	}
	return nil
}

// GetNode returns a Node given a NodeID.
// - Returns `InvalidArgument` if NodeID is not provided.
// - Returns `NotFound` if the Node is not found.
func (s *Server) GetNode(ctx context.Context, request *api.GetNodeRequest) (*api.GetNodeResponse, error) {
	if request.NodeID == "" {
		return nil, grpc.Errorf(codes.InvalidArgument, errInvalidArgument.Error())
	}

	var node *api.Node
	s.store.View(func(tx store.ReadTx) {
		node = store.GetNode(tx, request.NodeID)
	})
	if node == nil {
		return nil, grpc.Errorf(codes.NotFound, "node %s not found", request.NodeID)
	}

	if s.raft != nil {
		memberlist := s.raft.GetMemberlist()
		raftID, err := identity.ParseNodeID(request.NodeID)
		if err == nil && memberlist[raftID] != nil {
			node.Manager = &api.Manager{Raft: *memberlist[raftID]}
		}
	}

	return &api.GetNodeResponse{
		Node: node,
	}, nil
}

func filterNodes(candidates []*api.Node, filters ...func(*api.Node) bool) []*api.Node {
	result := []*api.Node{}

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

// ListNodes returns a list of all nodes.
func (s *Server) ListNodes(ctx context.Context, request *api.ListNodesRequest) (*api.ListNodesResponse, error) {
	var (
		nodes []*api.Node
		err   error
	)
	s.store.View(func(tx store.ReadTx) {
		switch {
		case request.Filters != nil && len(request.Filters.Names) > 0:
			nodes, err = store.FindNodes(tx, buildFilters(store.ByName, request.Filters.Names))
		case request.Filters != nil && len(request.Filters.IDPrefixes) > 0:
			nodes, err = store.FindNodes(tx, buildFilters(store.ByIDPrefix, request.Filters.IDPrefixes))
		case request.Filters != nil && len(request.Filters.Roles) > 0:
			filters := make([]store.By, 0, len(request.Filters.Roles))
			for _, v := range request.Filters.Roles {
				filters = append(filters, store.ByRole(v))
			}
			nodes, err = store.FindNodes(tx, store.Or(filters...))
		case request.Filters != nil && len(request.Filters.Memberships) > 0:
			filters := make([]store.By, 0, len(request.Filters.Memberships))
			for _, v := range request.Filters.Memberships {
				filters = append(filters, store.ByMembership(v))
			}
			nodes, err = store.FindNodes(tx, store.Or(filters...))
		default:
			nodes, err = store.FindNodes(tx, store.All)
		}
	})
	if err != nil {
		return nil, err
	}

	if request.Filters != nil {
		nodes = filterNodes(nodes,
			func(e *api.Node) bool {
				if len(request.Filters.Names) == 0 {
					return true
				}
				if e.Description == nil {
					return false
				}
				return filterContains(e.Description.Hostname, request.Filters.Names)
			},
			func(e *api.Node) bool {
				return filterContainsPrefix(e.ID, request.Filters.IDPrefixes)
			},
			func(e *api.Node) bool {
				if len(request.Filters.Labels) == 0 {
					return true
				}
				if e.Description == nil {
					return false
				}
				return filterMatchLabels(e.Description.Engine.Labels, request.Filters.Labels)
			},
			func(e *api.Node) bool {
				if len(request.Filters.Roles) == 0 {
					return true
				}
				for _, c := range request.Filters.Roles {
					if c == e.Spec.Role {
						return true
					}
				}
				return false
			},
			func(e *api.Node) bool {
				if len(request.Filters.Memberships) == 0 {
					return true
				}
				for _, c := range request.Filters.Memberships {
					if c == e.Spec.Membership {
						return true
					}
				}
				return false
			},
		)
	}

	// Add in manager information on nodes that are managers
	if s.raft != nil {
		memberlist := s.raft.GetMemberlist()

		for _, n := range nodes {
			raftID, err := identity.ParseNodeID(n.ID)
			if err == nil && memberlist[raftID] != nil {
				n.Manager = &api.Manager{Raft: *memberlist[raftID]}
			}
		}
	}

	return &api.ListNodesResponse{
		Nodes: nodes,
	}, nil
}

// UpdateNode updates a Node referenced by NodeID with the given NodeSpec.
// - Returns `NotFound` if the Node is not found.
// - Returns `InvalidArgument` if the NodeSpec is malformed.
// - Returns an error if the update fails.
func (s *Server) UpdateNode(ctx context.Context, request *api.UpdateNodeRequest) (*api.UpdateNodeResponse, error) {
	if request.NodeID == "" || request.NodeVersion == nil {
		return nil, grpc.Errorf(codes.InvalidArgument, errInvalidArgument.Error())
	}
	if err := validateNodeSpec(request.Spec); err != nil {
		return nil, err
	}

	var node *api.Node
	err := s.store.Update(func(tx store.Tx) error {
		node = store.GetNode(tx, request.NodeID)
		if node == nil {
			return nil
		}
		node.Meta.Version = *request.NodeVersion
		node.Spec = *request.Spec.Copy()
		return store.UpdateNode(tx, node)
	})
	if err != nil {
		return nil, err
	}
	if node == nil {
		return nil, grpc.Errorf(codes.NotFound, "node %s not found", request.NodeID)
	}
	return &api.UpdateNodeResponse{
		Node: node,
	}, nil
}

// RemoveNode updates a Node referenced by NodeID with the given NodeSpec.
// - Returns NotFound if the Node is not found.
// - Returns FailedPrecondition if the Node has manager role or not shut down.
// - Returns InvalidArgument if NodeID or NodeVersion is not valid.
// - Returns an error if the delete fails.
func (s *Server) RemoveNode(ctx context.Context, request *api.RemoveNodeRequest) (*api.RemoveNodeResponse, error) {
	if request.NodeID == "" {
		return nil, grpc.Errorf(codes.InvalidArgument, errInvalidArgument.Error())
	}
	if s.raft != nil {
		memberlist := s.raft.GetMemberlist()
		raftID, err := identity.ParseNodeID(request.NodeID)
		if err == nil && memberlist[raftID] != nil {
			return nil, grpc.Errorf(codes.FailedPrecondition, "node %s is a cluster manager and is part of the quorum. It must be demoted to worker before removal", request.NodeID)
		}
	}

	err := s.store.Update(func(tx store.Tx) error {
		node := store.GetNode(tx, request.NodeID)
		if node == nil {
			return grpc.Errorf(codes.NotFound, "node %s not found", request.NodeID)
		}
		if node.Spec.Role == api.NodeRoleManager {
			return grpc.Errorf(codes.FailedPrecondition, "node %s role is set to manager. It should be demoted to worker for safe removal", request.NodeID)
		}
		if node.Status.State == api.NodeStatus_READY {
			return grpc.Errorf(codes.FailedPrecondition, "node %s is not down and can't be removed", request.NodeID)
		}
		return store.DeleteNode(tx, request.NodeID)
	})
	if err != nil {
		return nil, err
	}
	return &api.RemoveNodeResponse{}, nil
}
