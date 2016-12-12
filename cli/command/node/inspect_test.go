package node

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/swarm"
	"github.com/docker/docker/pkg/testutil/assert"
	"github.com/docker/docker/pkg/testutil/golden"
)

func TestNodeInspectShouldReturnAnErrorIfAPIfail(t *testing.T) {
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
				return types.Info{}, fmt.Errorf("error asking for node info")
			},
			expectedError: "error asking for node info",
		},
		{
			args: []string{"nodeID"},
			nodeInspectFunc: func() (swarm.Node, []byte, error) {
				return swarm.Node{}, []byte{}, fmt.Errorf("error inspecting the node")
			},
			infoFunc: func() (types.Info, error) {
				return types.Info{}, fmt.Errorf("error asking for node info")
			},
			expectedError: "error inspecting the node",
		},
		{
			args: []string{"self"},
			nodeInspectFunc: func() (swarm.Node, []byte, error) {
				return swarm.Node{}, []byte{}, fmt.Errorf("error inspecting the node")
			},
			infoFunc: func() (types.Info, error) {
				return types.Info{}, nil
			},
			expectedError: "error inspecting the node",
		},
		{
			args: []string{"self"},
			flags: map[string]string{
				"pretty": "true",
			},
			infoFunc: func() (types.Info, error) {
				return types.Info{}, fmt.Errorf("error asking for node info")
			},
			expectedError: "error asking for node info",
		},
	}
	for _, tc := range testCases {
		buf := new(bytes.Buffer)
		cmd := newInspectCommand(&fakeCli{
			out: buf,
			client: &fakeClient{
				nodeInspectFunc: tc.nodeInspectFunc,
				infoFunc:        tc.infoFunc,
			},
		})
		cmd.SetArgs(tc.args)
		for key, value := range tc.flags {
			cmd.Flags().Set(key, value)
		}
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
				return aNode("nodeID1").labels(map[string]string{
					"lbl1": "value1",
				}).build(), []byte{}, nil
			},
		},
		{
			name: "manager",
			nodeInspectFunc: func() (swarm.Node, []byte, error) {
				return aNode("nodeID1").manager().build(), []byte{}, nil
			},
		},
		{
			name: "manager-leader",
			nodeInspectFunc: func() (swarm.Node, []byte, error) {
				return aNode("nodeID1").manager().leader().build(), []byte{}, nil
			},
		},
	}
	for _, tc := range testCases {
		buf := new(bytes.Buffer)
		cmd := newInspectCommand(&fakeCli{
			out: buf,
			client: &fakeClient{
				nodeInspectFunc: tc.nodeInspectFunc,
			},
		})
		cmd.SetArgs([]string{"nodeID"})
		cmd.Flags().Set("pretty", "true")
		assert.NilError(t, cmd.Execute())
		actual := buf.String()
		expected := golden.Get(t, []byte(actual), fmt.Sprintf("node-inspect-pretty.%s.golden", tc.name))
		assert.Equal(t, actual, string(expected))
	}
}
