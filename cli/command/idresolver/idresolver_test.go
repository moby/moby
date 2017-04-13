package idresolver

import (
	"testing"

	"github.com/docker/docker/api/types/swarm"
	// Import builders to get the builder function as package function
	. "github.com/docker/docker/cli/internal/test/builders"
	"github.com/docker/docker/pkg/testutil/assert"
	"github.com/pkg/errors"
	"golang.org/x/net/context"
)

func TestResolveError(t *testing.T) {
	cli := &fakeClient{
		nodeInspectFunc: func(nodeID string) (swarm.Node, []byte, error) {
			return swarm.Node{}, []byte{}, errors.Errorf("error inspecting node")
		},
	}

	idResolver := New(cli, false)
	_, err := idResolver.Resolve(context.Background(), struct{}{}, "nodeID")

	assert.Error(t, err, "unsupported type")
}

func TestResolveWithNoResolveOption(t *testing.T) {
	resolved := false
	cli := &fakeClient{
		nodeInspectFunc: func(nodeID string) (swarm.Node, []byte, error) {
			resolved = true
			return swarm.Node{}, []byte{}, nil
		},
		serviceInspectFunc: func(serviceID string) (swarm.Service, []byte, error) {
			resolved = true
			return swarm.Service{}, []byte{}, nil
		},
	}

	idResolver := New(cli, true)
	id, err := idResolver.Resolve(context.Background(), swarm.Node{}, "nodeID")

	assert.NilError(t, err)
	assert.Equal(t, id, "nodeID")
	assert.Equal(t, resolved, false)
}

func TestResolveWithCache(t *testing.T) {
	inspectCounter := 0
	cli := &fakeClient{
		nodeInspectFunc: func(nodeID string) (swarm.Node, []byte, error) {
			inspectCounter++
			return *Node(NodeName("node-foo")), []byte{}, nil
		},
	}

	idResolver := New(cli, false)

	ctx := context.Background()
	for i := 0; i < 2; i++ {
		id, err := idResolver.Resolve(ctx, swarm.Node{}, "nodeID")
		assert.NilError(t, err)
		assert.Equal(t, id, "node-foo")
	}

	assert.Equal(t, inspectCounter, 1)
}

func TestResolveNode(t *testing.T) {
	testCases := []struct {
		nodeID          string
		nodeInspectFunc func(string) (swarm.Node, []byte, error)
		expectedID      string
	}{
		{
			nodeID: "nodeID",
			nodeInspectFunc: func(string) (swarm.Node, []byte, error) {
				return swarm.Node{}, []byte{}, errors.Errorf("error inspecting node")
			},
			expectedID: "nodeID",
		},
		{
			nodeID: "nodeID",
			nodeInspectFunc: func(string) (swarm.Node, []byte, error) {
				return *Node(NodeName("node-foo")), []byte{}, nil
			},
			expectedID: "node-foo",
		},
		{
			nodeID: "nodeID",
			nodeInspectFunc: func(string) (swarm.Node, []byte, error) {
				return *Node(NodeName(""), Hostname("node-hostname")), []byte{}, nil
			},
			expectedID: "node-hostname",
		},
	}

	ctx := context.Background()
	for _, tc := range testCases {
		cli := &fakeClient{
			nodeInspectFunc: tc.nodeInspectFunc,
		}
		idResolver := New(cli, false)
		id, err := idResolver.Resolve(ctx, swarm.Node{}, tc.nodeID)

		assert.NilError(t, err)
		assert.Equal(t, id, tc.expectedID)
	}
}

func TestResolveService(t *testing.T) {
	testCases := []struct {
		serviceID          string
		serviceInspectFunc func(string) (swarm.Service, []byte, error)
		expectedID         string
	}{
		{
			serviceID: "serviceID",
			serviceInspectFunc: func(string) (swarm.Service, []byte, error) {
				return swarm.Service{}, []byte{}, errors.Errorf("error inspecting service")
			},
			expectedID: "serviceID",
		},
		{
			serviceID: "serviceID",
			serviceInspectFunc: func(string) (swarm.Service, []byte, error) {
				return *Service(ServiceName("service-foo")), []byte{}, nil
			},
			expectedID: "service-foo",
		},
	}

	ctx := context.Background()
	for _, tc := range testCases {
		cli := &fakeClient{
			serviceInspectFunc: tc.serviceInspectFunc,
		}
		idResolver := New(cli, false)
		id, err := idResolver.Resolve(ctx, swarm.Service{}, tc.serviceID)

		assert.NilError(t, err)
		assert.Equal(t, id, tc.expectedID)
	}
}
