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

func TestNodeUpdateErrors(t *testing.T) {
	testCases := []struct {
		args            []string
		flags           map[string]string
		nodeInspectFunc func() (swarm.Node, []byte, error)
		nodeUpdateFunc  func(nodeID string, version swarm.Version, node swarm.NodeSpec) error
		expectedError   string
	}{
		{
			expectedError: "requires exactly 1 argument",
		},
		{
			args:          []string{"node1", "node2"},
			expectedError: "requires exactly 1 argument",
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
		{
			args: []string{"nodeID"},
			nodeInspectFunc: func() (swarm.Node, []byte, error) {
				return *Node(NodeLabels(map[string]string{
					"key": "value",
				})), []byte{}, nil
			},
			flags: map[string]string{
				"label-rm": "notpresent",
			},
			expectedError: "key notpresent doesn't exist in node's labels",
		},
	}
	for _, tc := range testCases {
		buf := new(bytes.Buffer)
		cmd := newUpdateCommand(
			test.NewFakeCli(&fakeClient{
				nodeInspectFunc: tc.nodeInspectFunc,
				nodeUpdateFunc:  tc.nodeUpdateFunc,
			}, buf))
		cmd.SetArgs(tc.args)
		for key, value := range tc.flags {
			cmd.Flags().Set(key, value)
		}
		cmd.SetOutput(ioutil.Discard)
		testutil.ErrorContains(t, cmd.Execute(), tc.expectedError)
	}
}

func TestNodeUpdate(t *testing.T) {
	testCases := []struct {
		args            []string
		flags           map[string]string
		nodeInspectFunc func() (swarm.Node, []byte, error)
		nodeUpdateFunc  func(nodeID string, version swarm.Version, node swarm.NodeSpec) error
	}{
		{
			args: []string{"nodeID"},
			flags: map[string]string{
				"role": "manager",
			},
			nodeInspectFunc: func() (swarm.Node, []byte, error) {
				return *Node(), []byte{}, nil
			},
			nodeUpdateFunc: func(nodeID string, version swarm.Version, node swarm.NodeSpec) error {
				if node.Role != swarm.NodeRoleManager {
					return errors.Errorf("expected role manager, got %s", node.Role)
				}
				return nil
			},
		},
		{
			args: []string{"nodeID"},
			flags: map[string]string{
				"availability": "drain",
			},
			nodeInspectFunc: func() (swarm.Node, []byte, error) {
				return *Node(), []byte{}, nil
			},
			nodeUpdateFunc: func(nodeID string, version swarm.Version, node swarm.NodeSpec) error {
				if node.Availability != swarm.NodeAvailabilityDrain {
					return errors.Errorf("expected drain availability, got %s", node.Availability)
				}
				return nil
			},
		},
		{
			args: []string{"nodeID"},
			flags: map[string]string{
				"label-add": "lbl",
			},
			nodeInspectFunc: func() (swarm.Node, []byte, error) {
				return *Node(), []byte{}, nil
			},
			nodeUpdateFunc: func(nodeID string, version swarm.Version, node swarm.NodeSpec) error {
				if _, present := node.Annotations.Labels["lbl"]; !present {
					return errors.Errorf("expected 'lbl' label, got %v", node.Annotations.Labels)
				}
				return nil
			},
		},
		{
			args: []string{"nodeID"},
			flags: map[string]string{
				"label-add": "key=value",
			},
			nodeInspectFunc: func() (swarm.Node, []byte, error) {
				return *Node(), []byte{}, nil
			},
			nodeUpdateFunc: func(nodeID string, version swarm.Version, node swarm.NodeSpec) error {
				if value, present := node.Annotations.Labels["key"]; !present || value != "value" {
					return errors.Errorf("expected 'key' label to be 'value', got %v", node.Annotations.Labels)
				}
				return nil
			},
		},
		{
			args: []string{"nodeID"},
			flags: map[string]string{
				"label-rm": "key",
			},
			nodeInspectFunc: func() (swarm.Node, []byte, error) {
				return *Node(NodeLabels(map[string]string{
					"key": "value",
				})), []byte{}, nil
			},
			nodeUpdateFunc: func(nodeID string, version swarm.Version, node swarm.NodeSpec) error {
				if len(node.Annotations.Labels) > 0 {
					return errors.Errorf("expected no labels, got %v", node.Annotations.Labels)
				}
				return nil
			},
		},
	}
	for _, tc := range testCases {
		buf := new(bytes.Buffer)
		cmd := newUpdateCommand(
			test.NewFakeCli(&fakeClient{
				nodeInspectFunc: tc.nodeInspectFunc,
				nodeUpdateFunc:  tc.nodeUpdateFunc,
			}, buf))
		cmd.SetArgs(tc.args)
		for key, value := range tc.flags {
			cmd.Flags().Set(key, value)
		}
		assert.NoError(t, cmd.Execute())
	}
}
