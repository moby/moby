package node

import (
	"bytes"
	"fmt"
	"testing"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/swarm"
	"github.com/docker/docker/pkg/testutil/assert"
	"github.com/docker/docker/pkg/testutil/golden"
)

func TestNodePsErrors(t *testing.T) {
	testCases := []struct {
		args            []string
		flags           map[string]string
		infoFunc        func() (types.Info, error)
		nodeInspectFunc func() (swarm.Node, []byte, error)
		taskListFunc    func(options types.TaskListOptions) ([]swarm.Task, error)
		taskInspectFunc func(taskID string) (swarm.Task, []byte, error)
		expectedError   string
	}{
		{
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
			expectedError: "error inspecting the node",
		},
		{
			args: []string{"nodeID"},
			taskListFunc: func(options types.TaskListOptions) ([]swarm.Task, error) {
				return []swarm.Task{}, fmt.Errorf("error returning the task list")
			},
			expectedError: "error returning the task list",
		},
		/*
			{
				args: []string{"nodeID"},
				taskListFunc: func(options types.TaskListOptions) ([]swarm.Task, error) {
					return []swarm.Task{
						aTask("").build(),
					}, nil
				},
				taskInspectFunc: func(taskID string) (swarm.Task, []byte, error) {
					return swarm.Task{}, []byte{}, fmt.Errorf("error inspecting a task")
				},
				expectedError: "error returning the task list",
			},
		*/
	}
	for _, tc := range testCases {
		buf := new(bytes.Buffer)
		cmd := newPsCommand(&fakeCli{
			out: buf,
			client: &fakeClient{
				infoFunc:        tc.infoFunc,
				nodeInspectFunc: tc.nodeInspectFunc,
				taskInspectFunc: tc.taskInspectFunc,
				taskListFunc:    tc.taskListFunc,
			},
		})
		cmd.SetArgs(tc.args)
		for key, value := range tc.flags {
			cmd.Flags().Set(key, value)
		}
		assert.Error(t, cmd.Execute(), tc.expectedError)
	}
}

func TestNodePs(t *testing.T) {
	testCases := []struct {
		name            string
		args            []string
		flags           map[string]string
		infoFunc        func() (types.Info, error)
		nodeInspectFunc func() (swarm.Node, []byte, error)
		taskListFunc    func(options types.TaskListOptions) ([]swarm.Task, error)
		taskInspectFunc func(taskID string) (swarm.Task, []byte, error)
	}{
		{
			name: "simple",
			args: []string{"nodeID"},
			nodeInspectFunc: func() (swarm.Node, []byte, error) {
				return aNode("nodeID1").build(), []byte{}, nil
			},
			taskListFunc: func(options types.TaskListOptions) ([]swarm.Task, error) {
				return []swarm.Task{
					aTask("taskID").statusTimeStamp(time.Now().Add(-2 * time.Hour)).statusPortStatus([]swarm.PortConfig{
						{
							TargetPort:    80,
							PublishedPort: 80,
							Protocol:      "tcp",
						},
					}).build(),
				}, nil
			},
		},
		{
			name: "with-errors",
			args: []string{"nodeID"},
			nodeInspectFunc: func() (swarm.Node, []byte, error) {
				return aNode("nodeID1").build(), []byte{}, nil
			},
			taskListFunc: func(options types.TaskListOptions) ([]swarm.Task, error) {
				return []swarm.Task{
					aTask("taskID1").serviceID("failure").statusTimeStamp(time.Now().Add(-2 * time.Hour)).statusErr("a task error").build(),
					aTask("taskID2").serviceID("failure").statusTimeStamp(time.Now().Add(-3 * time.Hour)).statusErr("a task error").build(),
					aTask("taskID3").serviceID("failure").statusTimeStamp(time.Now().Add(-4 * time.Hour)).statusErr("a task error").build(),
				}, nil
			},
		},
	}
	for _, tc := range testCases {
		buf := new(bytes.Buffer)
		cmd := newPsCommand(&fakeCli{
			out: buf,
			client: &fakeClient{
				infoFunc:        tc.infoFunc,
				nodeInspectFunc: tc.nodeInspectFunc,
				taskInspectFunc: tc.taskInspectFunc,
				taskListFunc:    tc.taskListFunc,
			},
		})
		cmd.SetArgs(tc.args)
		for key, value := range tc.flags {
			cmd.Flags().Set(key, value)
		}
		assert.NilError(t, cmd.Execute())
		actual := buf.String()
		expected := golden.Get(t, []byte(actual), fmt.Sprintf("node-ps.%s.golden", tc.name))
		assert.Equal(t, actual, string(expected))
	}
}
