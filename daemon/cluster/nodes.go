package cluster // import "github.com/docker/docker/daemon/cluster"

import (
	"context"

	apitypes "github.com/docker/docker/api/types"
	types "github.com/docker/docker/api/types/swarm"
	"github.com/docker/docker/daemon/cluster/convert"
	"github.com/docker/docker/errdefs"
	swarmapi "github.com/moby/swarmkit/v2/api"
	"google.golang.org/grpc"
)

// GetNodes returns a list of all nodes known to a cluster.
func (c *Cluster) GetNodes(options apitypes.NodeListOptions) ([]types.Node, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	state := c.currentNodeState()
	if !state.IsActiveManager() {
		return nil, c.errNoManager(state)
	}

	filters, err := newListNodesFilters(options.Filters)
	if err != nil {
		return nil, err
	}

	ctx := context.TODO()
	ctx, cancel := context.WithTimeout(ctx, swarmRequestTimeout)
	defer cancel()

	r, err := state.controlClient.ListNodes(
		ctx,
		&swarmapi.ListNodesRequest{Filters: filters},
		grpc.MaxCallRecvMsgSize(defaultRecvSizeForListResponse),
	)
	if err != nil {
		return nil, err
	}

	nodes := make([]types.Node, 0, len(r.Nodes))

	for _, node := range r.Nodes {
		nodes = append(nodes, convert.NodeFromGRPC(*node))
	}
	return nodes, nil
}

// GetNode returns a node based on an ID.
func (c *Cluster) GetNode(input string) (types.Node, error) {
	var node *swarmapi.Node

	if err := c.lockedManagerAction(func(ctx context.Context, state nodeState) error {
		n, err := getNode(ctx, state.controlClient, input)
		if err != nil {
			return err
		}
		node = n
		return nil
	}); err != nil {
		return types.Node{}, err
	}

	return convert.NodeFromGRPC(*node), nil
}

// UpdateNode updates existing nodes properties.
func (c *Cluster) UpdateNode(input string, version uint64, spec types.NodeSpec) error {
	return c.lockedManagerAction(func(_ context.Context, state nodeState) error {
		nodeSpec, err := convert.NodeSpecToGRPC(spec)
		if err != nil {
			return errdefs.InvalidParameter(err)
		}

		ctx := context.TODO()
		ctx, cancel := context.WithTimeout(ctx, swarmRequestTimeout)
		defer cancel()

		currentNode, err := getNode(ctx, state.controlClient, input)
		if err != nil {
			return err
		}

		_, err = state.controlClient.UpdateNode(
			ctx,
			&swarmapi.UpdateNodeRequest{
				NodeID: currentNode.ID,
				Spec:   &nodeSpec,
				NodeVersion: &swarmapi.Version{
					Index: version,
				},
			},
		)
		return err
	})
}

// RemoveNode removes a node from a cluster
func (c *Cluster) RemoveNode(input string, force bool) error {
	return c.lockedManagerAction(func(ctx context.Context, state nodeState) error {
		node, err := getNode(ctx, state.controlClient, input)
		if err != nil {
			return err
		}

		_, err = state.controlClient.RemoveNode(ctx, &swarmapi.RemoveNodeRequest{NodeID: node.ID, Force: force})
		return err
	})
}
