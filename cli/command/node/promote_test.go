package node

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"strings"
	"testing"

	"github.com/docker/docker/api/types/swarm"
	"github.com/docker/docker/cli/test"
	"github.com/docker/docker/cli/test/builder"
	"github.com/docker/docker/pkg/testutil/assert"
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
				return swarm.Node{}, []byte{}, fmt.Errorf("error inspecting the node")
			},
			expectedError: "error inspecting the node",
		},
		{
			args: []string{"nodeID"},
			nodeUpdateFunc: func(nodeID string, version swarm.Version, node swarm.NodeSpec) error {
				return fmt.Errorf("error updating the node")
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
			}, buf, ioutil.NopCloser(strings.NewReader(""))))
		cmd.SetArgs(tc.args)
		assert.Error(t, cmd.Execute(), tc.expectedError)
	}
}

func TestNodePromoteNoChange(t *testing.T) {
	buf := new(bytes.Buffer)
	cmd := newPromoteCommand(
		test.NewFakeCli(&fakeClient{
			nodeInspectFunc: func() (swarm.Node, []byte, error) {
				return builder.ANode("nodeID").Manager().Build(), []byte{}, nil
			},
			nodeUpdateFunc: func(nodeID string, version swarm.Version, node swarm.NodeSpec) error {
				if node.Role != swarm.NodeRoleManager {
					return fmt.Errorf("expected role manager, got %s", node.Role)
				}
				return nil
			},
		}, buf, ioutil.NopCloser(strings.NewReader(""))))
	cmd.SetArgs([]string{"nodeID"})
	assert.NilError(t, cmd.Execute())
}

func TestNodePromoteMultipleNode(t *testing.T) {
	buf := new(bytes.Buffer)
	cmd := newPromoteCommand(
		test.NewFakeCli(&fakeClient{
			nodeInspectFunc: func() (swarm.Node, []byte, error) {
				return builder.ANode("nodeID").Build(), []byte{}, nil
			},
			nodeUpdateFunc: func(nodeID string, version swarm.Version, node swarm.NodeSpec) error {
				if node.Role != swarm.NodeRoleManager {
					return fmt.Errorf("expected role manager, got %s", node.Role)
				}
				return nil
			},
		}, buf, ioutil.NopCloser(strings.NewReader(""))))
	cmd.SetArgs([]string{"nodeID1", "nodeID2"})
	assert.NilError(t, cmd.Execute())
}
