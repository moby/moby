package node

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"testing"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/swarm"
	"github.com/docker/docker/cli/internal/test"
	"github.com/pkg/errors"
	// Import builders to get the builder function as package function
	. "github.com/docker/docker/cli/internal/test/builders"
	"github.com/docker/docker/pkg/testutil"
	"github.com/docker/docker/pkg/testutil/golden"
	"github.com/stretchr/testify/assert"
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
				return types.Info{}, errors.Errorf("error asking for node info")
			},
			expectedError: "error asking for node info",
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
			taskListFunc: func(options types.TaskListOptions) ([]swarm.Task, error) {
				return []swarm.Task{}, errors.Errorf("error returning the task list")
			},
			expectedError: "error returning the task list",
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
			}, buf))
		cmd.SetArgs(tc.args)
		for key, value := range tc.flags {
			cmd.Flags().Set(key, value)
		}
		cmd.SetOutput(ioutil.Discard)
		assert.EqualError(t, cmd.Execute(), tc.expectedError)
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
				return *Node(), []byte{}, nil
			},
			taskListFunc: func(options types.TaskListOptions) ([]swarm.Task, error) {
				return []swarm.Task{
					*Task(WithStatus(Timestamp(time.Now().Add(-2*time.Hour)), PortStatus([]swarm.PortConfig{
						{
							TargetPort:    80,
							PublishedPort: 80,
							Protocol:      "tcp",
						},
					}))),
				}, nil
			},
		},
		{
			name: "with-errors",
			args: []string{"nodeID"},
			nodeInspectFunc: func() (swarm.Node, []byte, error) {
				return *Node(), []byte{}, nil
			},
			taskListFunc: func(options types.TaskListOptions) ([]swarm.Task, error) {
				return []swarm.Task{
					*Task(TaskID("taskID1"), ServiceID("failure"),
						WithStatus(Timestamp(time.Now().Add(-2*time.Hour)), StatusErr("a task error"))),
					*Task(TaskID("taskID2"), ServiceID("failure"),
						WithStatus(Timestamp(time.Now().Add(-3*time.Hour)), StatusErr("a task error"))),
					*Task(TaskID("taskID3"), ServiceID("failure"),
						WithStatus(Timestamp(time.Now().Add(-4*time.Hour)), StatusErr("a task error"))),
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
			}, buf))
		cmd.SetArgs(tc.args)
		for key, value := range tc.flags {
			cmd.Flags().Set(key, value)
		}
		assert.NoError(t, cmd.Execute())
		actual := buf.String()
		expected := golden.Get(t, []byte(actual), fmt.Sprintf("node-ps.%s.golden", tc.name))
		testutil.EqualNormalizedString(t, testutil.RemoveSpace, actual, string(expected))
	}
}
