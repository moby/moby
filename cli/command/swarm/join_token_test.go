package swarm

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"testing"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/swarm"
	"github.com/docker/docker/cli/internal/test"
	// Import builders to get the builder function as package function
	. "github.com/docker/docker/cli/internal/test/builders"
	"github.com/docker/docker/pkg/testutil/assert"
	"github.com/docker/docker/pkg/testutil/golden"
)

func TestSwarmJoinTokenErrors(t *testing.T) {
	testCases := []struct {
		name             string
		args             []string
		flags            map[string]string
		infoFunc         func() (types.Info, error)
		swarmInspectFunc func() (swarm.Swarm, error)
		swarmUpdateFunc  func(swarm swarm.Spec, flags swarm.UpdateFlags) error
		nodeInspectFunc  func() (swarm.Node, []byte, error)
		expectedError    string
	}{
		{
			name:          "not-enough-args",
			expectedError: "requires exactly 1 argument",
		},
		{
			name:          "too-many-args",
			args:          []string{"worker", "manager"},
			expectedError: "requires exactly 1 argument",
		},
		{
			name:          "invalid-args",
			args:          []string{"foo"},
			expectedError: "unknown role foo",
		},
		{
			name: "swarm-inspect-failed",
			args: []string{"worker"},
			swarmInspectFunc: func() (swarm.Swarm, error) {
				return swarm.Swarm{}, fmt.Errorf("error inspecting the swarm")
			},
			expectedError: "error inspecting the swarm",
		},
		{
			name: "swarm-inspect-rotate-failed",
			args: []string{"worker"},
			flags: map[string]string{
				flagRotate: "true",
			},
			swarmInspectFunc: func() (swarm.Swarm, error) {
				return swarm.Swarm{}, fmt.Errorf("error inspecting the swarm")
			},
			expectedError: "error inspecting the swarm",
		},
		{
			name: "swarm-update-failed",
			args: []string{"worker"},
			flags: map[string]string{
				flagRotate: "true",
			},
			swarmUpdateFunc: func(swarm swarm.Spec, flags swarm.UpdateFlags) error {
				return fmt.Errorf("error updating the swarm")
			},
			expectedError: "error updating the swarm",
		},
		{
			name: "node-inspect-failed",
			args: []string{"worker"},
			nodeInspectFunc: func() (swarm.Node, []byte, error) {
				return swarm.Node{}, []byte{}, fmt.Errorf("error inspecting node")
			},
			expectedError: "error inspecting node",
		},
		{
			name: "info-failed",
			args: []string{"worker"},
			infoFunc: func() (types.Info, error) {
				return types.Info{}, fmt.Errorf("error asking for node info")
			},
			expectedError: "error asking for node info",
		},
	}
	for _, tc := range testCases {
		buf := new(bytes.Buffer)
		cmd := newJoinTokenCommand(
			test.NewFakeCli(&fakeClient{
				swarmInspectFunc: tc.swarmInspectFunc,
				swarmUpdateFunc:  tc.swarmUpdateFunc,
				infoFunc:         tc.infoFunc,
				nodeInspectFunc:  tc.nodeInspectFunc,
			}, buf))
		cmd.SetArgs(tc.args)
		for key, value := range tc.flags {
			cmd.Flags().Set(key, value)
		}
		cmd.SetOutput(ioutil.Discard)
		assert.Error(t, cmd.Execute(), tc.expectedError)
	}
}

func TestSwarmJoinToken(t *testing.T) {
	testCases := []struct {
		name             string
		args             []string
		flags            map[string]string
		infoFunc         func() (types.Info, error)
		swarmInspectFunc func() (swarm.Swarm, error)
		nodeInspectFunc  func() (swarm.Node, []byte, error)
	}{
		{
			name: "worker",
			args: []string{"worker"},
			infoFunc: func() (types.Info, error) {
				return types.Info{
					Swarm: swarm.Info{
						NodeID: "nodeID",
					},
				}, nil
			},
			nodeInspectFunc: func() (swarm.Node, []byte, error) {
				return *Node(Manager()), []byte{}, nil
			},
			swarmInspectFunc: func() (swarm.Swarm, error) {
				return *Swarm(), nil
			},
		},
		{
			name: "manager",
			args: []string{"manager"},
			infoFunc: func() (types.Info, error) {
				return types.Info{
					Swarm: swarm.Info{
						NodeID: "nodeID",
					},
				}, nil
			},
			nodeInspectFunc: func() (swarm.Node, []byte, error) {
				return *Node(Manager()), []byte{}, nil
			},
			swarmInspectFunc: func() (swarm.Swarm, error) {
				return *Swarm(), nil
			},
		},
		{
			name: "manager-rotate",
			args: []string{"manager"},
			flags: map[string]string{
				flagRotate: "true",
			},
			infoFunc: func() (types.Info, error) {
				return types.Info{
					Swarm: swarm.Info{
						NodeID: "nodeID",
					},
				}, nil
			},
			nodeInspectFunc: func() (swarm.Node, []byte, error) {
				return *Node(Manager()), []byte{}, nil
			},
			swarmInspectFunc: func() (swarm.Swarm, error) {
				return *Swarm(), nil
			},
		},
		{
			name: "worker-quiet",
			args: []string{"worker"},
			flags: map[string]string{
				flagQuiet: "true",
			},
			nodeInspectFunc: func() (swarm.Node, []byte, error) {
				return *Node(Manager()), []byte{}, nil
			},
			swarmInspectFunc: func() (swarm.Swarm, error) {
				return *Swarm(), nil
			},
		},
		{
			name: "manager-quiet",
			args: []string{"manager"},
			flags: map[string]string{
				flagQuiet: "true",
			},
			nodeInspectFunc: func() (swarm.Node, []byte, error) {
				return *Node(Manager()), []byte{}, nil
			},
			swarmInspectFunc: func() (swarm.Swarm, error) {
				return *Swarm(), nil
			},
		},
	}
	for _, tc := range testCases {
		buf := new(bytes.Buffer)
		cmd := newJoinTokenCommand(
			test.NewFakeCli(&fakeClient{
				swarmInspectFunc: tc.swarmInspectFunc,
				infoFunc:         tc.infoFunc,
				nodeInspectFunc:  tc.nodeInspectFunc,
			}, buf))
		cmd.SetArgs(tc.args)
		for key, value := range tc.flags {
			cmd.Flags().Set(key, value)
		}
		assert.NilError(t, cmd.Execute())
		actual := buf.String()
		expected := golden.Get(t, []byte(actual), fmt.Sprintf("jointoken-%s.golden", tc.name))
		assert.EqualNormalizedString(t, assert.RemoveSpace, actual, string(expected))
	}
}
