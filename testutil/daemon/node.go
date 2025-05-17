package daemon

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/swarm"
	"gotest.tools/v3/assert"
)

// NodeConstructor defines a swarm node constructor
type NodeConstructor func(*swarm.Node)

// GetNode returns a swarm node identified by the specified id
func (d *Daemon) GetNode(ctx context.Context, tb testing.TB, id string, errCheck ...func(error) bool) *swarm.Node {
	tb.Helper()
	cli := d.NewClientT(tb)
	defer cli.Close()

	node, _, err := cli.NodeInspectWithRaw(ctx, id)
	if err != nil {
		for _, f := range errCheck {
			if f(err) {
				return nil
			}
		}
	}
	assert.NilError(tb, err, "[%s] (*Daemon).GetNode: NodeInspectWithRaw(%q) failed", d.id, id)
	assert.Check(tb, node.ID == id)
	return &node
}

// RemoveNode removes the specified node
func (d *Daemon) RemoveNode(ctx context.Context, tb testing.TB, id string, force bool) {
	tb.Helper()
	cli := d.NewClientT(tb)
	defer cli.Close()

	options := types.NodeRemoveOptions{
		Force: force,
	}
	err := cli.NodeRemove(ctx, id, options)
	assert.NilError(tb, err)
}

// UpdateNode updates a swarm node with the specified node constructor
func (d *Daemon) UpdateNode(ctx context.Context, tb testing.TB, id string, f ...NodeConstructor) {
	tb.Helper()
	cli := d.NewClientT(tb)
	defer cli.Close()

	for i := 0; ; i++ {
		node := d.GetNode(ctx, tb, id)
		for _, fn := range f {
			fn(node)
		}

		err := cli.NodeUpdate(ctx, node.ID, node.Version, node.Spec)
		if i < 10 && err != nil && strings.Contains(err.Error(), "update out of sequence") {
			time.Sleep(100 * time.Millisecond)
			continue
		}
		assert.NilError(tb, err)
		return
	}
}

// ListNodes returns the list of the current swarm nodes
func (d *Daemon) ListNodes(ctx context.Context, tb testing.TB) []swarm.Node {
	tb.Helper()
	cli := d.NewClientT(tb)
	defer cli.Close()

	nodes, err := cli.NodeList(ctx, types.NodeListOptions{})
	assert.NilError(tb, err)

	return nodes
}
