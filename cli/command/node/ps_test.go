package node

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"strings"
	"testing"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/swarm"
	"github.com/docker/docker/cli/test"
	"github.com/docker/docker/cli/test/builder"
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
		cmd := newPsCommand(
			test.NewFakeCli(&fakeClient{
				infoFunc:        tc.infoFunc,
				nodeInspectFunc: tc.nodeInspectFunc,
				taskInspectFunc: tc.taskInspectFunc,
				taskListFunc:    tc.taskListFunc,
			}, buf, ioutil.NopCloser(strings.NewReader(""))))
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
				return builder.ANode("nodeID1").Build(), []byte{}, nil
			},
			taskListFunc: func(options types.TaskListOptions) ([]swarm.Task, error) {
				return []swarm.Task{
					builder.ATask("taskID").StatusTimestamp(time.Now().Add(-2 * time.Hour)).StatusPortStatus([]swarm.PortConfig{
						{
							TargetPort:    80,
							PublishedPort: 80,
							Protocol:      "tcp",
						},
					}).Build(),
				}, nil
			},
		},
		{
			name: "with-errors",
			args: []string{"nodeID"},
			nodeInspectFunc: func() (swarm.Node, []byte, error) {
				return builder.ANode("nodeID1").Build(), []byte{}, nil
			},
			taskListFunc: func(options types.TaskListOptions) ([]swarm.Task, error) {
				return []swarm.Task{
					builder.ATask("taskID1").ServiceID("failure").StatusTimestamp(time.Now().Add(-2 * time.Hour)).StatusErr("a task error").Build(),
					builder.ATask("taskID2").ServiceID("failure").StatusTimestamp(time.Now().Add(-3 * time.Hour)).StatusErr("a task error").Build(),
					builder.ATask("taskID3").ServiceID("failure").StatusTimestamp(time.Now().Add(-4 * time.Hour)).StatusErr("a task error").Build(),
				}, nil
			},
		},
	}
	for _, tc := range testCases {
		buf := new(bytes.Buffer)
		cmd := newPsCommand(
			test.NewFakeCli(&fakeClient{
				infoFunc:        tc.infoFunc,
				nodeInspectFunc: tc.nodeInspectFunc,
				taskInspectFunc: tc.taskInspectFunc,
				taskListFunc:    tc.taskListFunc,
			}, buf, ioutil.NopCloser(strings.NewReader(""))))
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
