package daemon

import (
	"context"
	"strings"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/swarm"
	"github.com/docker/docker/internal/test"
	"gotest.tools/assert"
)

// NodeConstructor defines a swarm node constructor
type NodeConstructor func(*swarm.Node)

// GetNode returns a swarm node identified by the specified id
func (d *Daemon) GetNode(t assert.TestingT, id string) *swarm.Node {
	if ht, ok := t.(test.HelperT); ok {
		ht.Helper()
	}
	cli := d.NewClientT(t)
	defer cli.Close()

	node, _, err := cli.NodeInspectWithRaw(context.Background(), id)
	assert.NilError(t, err)
	assert.Check(t, node.ID == id)
	return &node
}

// RemoveNode removes the specified node
func (d *Daemon) RemoveNode(t assert.TestingT, id string, force bool) {
	if ht, ok := t.(test.HelperT); ok {
		ht.Helper()
	}
	cli := d.NewClientT(t)
	defer cli.Close()

	options := types.NodeRemoveOptions{
		Force: force,
	}
	err := cli.NodeRemove(context.Background(), id, options)
	assert.NilError(t, err)
}

// UpdateNode updates a swarm node with the specified node constructor
func (d *Daemon) UpdateNode(t assert.TestingT, id string, f ...NodeConstructor) {
	if ht, ok := t.(test.HelperT); ok {
		ht.Helper()
	}
	cli := d.NewClientT(t)
	defer cli.Close()

	for i := 0; ; i++ {
		node := d.GetNode(t, id)
		for _, fn := range f {
			fn(node)
		}

		err := cli.NodeUpdate(context.Background(), node.ID, node.Version, node.Spec)
		if i < 10 && err != nil && strings.Contains(err.Error(), "update out of sequence") {
			time.Sleep(100 * time.Millisecond)
			continue
		}
		assert.NilError(t, err)
		return
	}
}

// ListNodes returns the list of the current swarm nodes
func (d *Daemon) ListNodes(t assert.TestingT) []swarm.Node {
	if ht, ok := t.(test.HelperT); ok {
		ht.Helper()
	}
	cli := d.NewClientT(t)
	defer cli.Close()

	nodes, err := cli.NodeList(context.Background(), types.NodeListOptions{})
	assert.NilError(t, err)

	return nodes
}
