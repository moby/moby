package node

import (
	"bytes"
	"io/ioutil"
	"testing"

	"github.com/docker/docker/api/types/swarm"
	"github.com/docker/docker/cli/internal/test"
	"github.com/pkg/errors"
	// Import builders to get the builder function as package function
	. "github.com/docker/docker/cli/internal/test/builders"
	"github.com/docker/docker/pkg/testutil"
	"github.com/stretchr/testify/assert"
)

func TestNodePromoteErrors(t *testing.T) {
	testCases := []struct {
		args            []string
		nodeInspectFunc func() (swarm.Node, []byte, error)
		nodeUpdateFunc  func(nodeID string, version swarm.Version, node swarm.NodeSpec) error
		expectedError   string
	}{
		{
			expectedError: "requires at least 1 argument",
		},
		{
			args: []string{"nodeID"},
			nodeInspectFunc: func() (swarm.Node, []byte, error) {
				return swarm.Node{}, []byte{}, errors.Errorf("error inspecting the node")
			},
			expectedError: "error inspecting the node",
		},
		{
			args: []string{"nodeID"},
			nodeUpdateFunc: func(nodeID string, version swarm.Version, node swarm.NodeSpec) error {
				return errors.Errorf("error updating the node")
			},
			expectedError: "error updating the node",
		},
	}
	for _, tc := range testCases {
		buf := new(bytes.Buffer)
		cmd := newPromoteCommand(
			test.NewFakeCli(&fakeClient{
				nodeInspectFunc: tc.nodeInspectFunc,
				nodeUpdateFunc:  tc.nodeUpdateFunc,
			}, buf))
		cmd.SetArgs(tc.args)
		cmd.SetOutput(ioutil.Discard)
		testutil.ErrorContains(t, cmd.Execute(), tc.expectedError)
	}
}

func TestNodePromoteNoChange(t *testing.T) {
	buf := new(bytes.Buffer)
	cmd := newPromoteCommand(
		test.NewFakeCli(&fakeClient{
			nodeInspectFunc: func() (swarm.Node, []byte, error) {
				return *Node(Manager()), []byte{}, nil
			},
			nodeUpdateFunc: func(nodeID string, version swarm.Version, node swarm.NodeSpec) error {
				if node.Role != swarm.NodeRoleManager {
					return errors.Errorf("expected role manager, got %s", node.Role)
				}
				return nil
			},
		}, buf))
	cmd.SetArgs([]string{"nodeID"})
	assert.NoError(t, cmd.Execute())
}

func TestNodePromoteMultipleNode(t *testing.T) {
	buf := new(bytes.Buffer)
	cmd := newPromoteCommand(
		test.NewFakeCli(&fakeClient{
			nodeInspectFunc: func() (swarm.Node, []byte, error) {
				return *Node(), []byte{}, nil
			},
			nodeUpdateFunc: func(nodeID string, version swarm.Version, node swarm.NodeSpec) error {
				if node.Role != swarm.NodeRoleManager {
					return errors.Errorf("expected role manager, got %s", node.Role)
				}
				return nil
			},
		}, buf))
	cmd.SetArgs([]string{"nodeID1", "nodeID2"})
	assert.NoError(t, cmd.Execute())
}
