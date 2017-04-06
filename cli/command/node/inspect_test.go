package node

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"testing"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/swarm"
	"github.com/docker/docker/cli/internal/test"
	"github.com/pkg/errors"
	// Import builders to get the builder function as package function
	. "github.com/docker/docker/cli/internal/test/builders"
	"github.com/docker/docker/pkg/testutil/assert"
	"github.com/docker/docker/pkg/testutil/golden"
)

func TestNodeInspectErrors(t *testing.T) {
	testCases := []struct {
		args            []string
		flags           map[string]string
		nodeInspectFunc func() (swarm.Node, []byte, error)
		infoFunc        func() (types.Info, error)
		expectedError   string
	}{
		{
			expectedError: "requires at least 1 argument",
		},
		{
			args: []string{"self"},
			infoFunc: func() (types.Info, error) {
				return types.Info{}, errors.Errorf("error asking for node info")
			},
			expectedError: "error asking for node info",
		},
		{
			args: []string{"nodeID"},
			nodeInspectFunc: func() (swarm.Node, []byte, error) {
				return swarm.Node{}, []byte{}, errors.Errorf("error inspecting the node")
			},
			infoFunc: func() (types.Info, error) {
				return types.Info{}, errors.Errorf("error asking for node info")
			},
			expectedError: "error inspecting the node",
		},
		{
			args: []string{"self"},
			nodeInspectFunc: func() (swarm.Node, []byte, error) {
				return swarm.Node{}, []byte{}, errors.Errorf("error inspecting the node")
			},
			infoFunc: func() (types.Info, error) {
				return types.Info{Swarm: swarm.Info{NodeID: "abc"}}, nil
			},
			expectedError: "error inspecting the node",
		},
		{
			args: []string{"self"},
			flags: map[string]string{
				"pretty": "true",
			},
			infoFunc: func() (types.Info, error) {
				return types.Info{}, errors.Errorf("error asking for node info")
			},
			expectedError: "error asking for node info",
		},
	}
	for _, tc := range testCases {
		buf := new(bytes.Buffer)
		cmd := newInspectCommand(
			test.NewFakeCli(&fakeClient{
				nodeInspectFunc: tc.nodeInspectFunc,
				infoFunc:        tc.infoFunc,
			}, buf))
		cmd.SetArgs(tc.args)
		for key, value := range tc.flags {
			cmd.Flags().Set(key, value)
		}
		cmd.SetOutput(ioutil.Discard)
		assert.Error(t, cmd.Execute(), tc.expectedError)
	}
}

func TestNodeInspectPretty(t *testing.T) {
	testCases := []struct {
		name            string
		nodeInspectFunc func() (swarm.Node, []byte, error)
	}{
		{
			name: "simple",
			nodeInspectFunc: func() (swarm.Node, []byte, error) {
				return *Node(NodeLabels(map[string]string{
					"lbl1": "value1",
				})), []byte{}, nil
			},
		},
		{
			name: "manager",
			nodeInspectFunc: func() (swarm.Node, []byte, error) {
				return *Node(Manager()), []byte{}, nil
			},
		},
		{
			name: "manager-leader",
			nodeInspectFunc: func() (swarm.Node, []byte, error) {
				return *Node(Manager(Leader())), []byte{}, nil
			},
		},
	}
	for _, tc := range testCases {
		buf := new(bytes.Buffer)
		cmd := newInspectCommand(
			test.NewFakeCli(&fakeClient{
				nodeInspectFunc: tc.nodeInspectFunc,
			}, buf))
		cmd.SetArgs([]string{"nodeID"})
		cmd.Flags().Set("pretty", "true")
		assert.NilError(t, cmd.Execute())
		actual := buf.String()
		expected := golden.Get(t, []byte(actual), fmt.Sprintf("node-inspect-pretty.%s.golden", tc.name))
		assert.EqualNormalizedString(t, assert.RemoveSpace, actual, string(expected))
	}
}
