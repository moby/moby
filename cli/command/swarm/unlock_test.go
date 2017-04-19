package swarm

import (
	"bytes"
	"io/ioutil"
	"strings"
	"testing"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/swarm"
	"github.com/docker/docker/cli/internal/test"
	"github.com/docker/docker/pkg/testutil"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
)

func TestSwarmUnlockErrors(t *testing.T) {
	testCases := []struct {
		name            string
		args            []string
		input           string
		swarmUnlockFunc func(req swarm.UnlockRequest) error
		infoFunc        func() (types.Info, error)
		expectedError   string
	}{
		{
			name:          "too-many-args",
			args:          []string{"foo"},
			expectedError: "accepts no argument(s)",
		},
		{
			name: "is-not-part-of-a-swarm",
			infoFunc: func() (types.Info, error) {
				return types.Info{
					Swarm: swarm.Info{
						LocalNodeState: swarm.LocalNodeStateInactive,
					},
				}, nil
			},
			expectedError: "This node is not part of a swarm",
		},
		{
			name: "is-not-locked",
			infoFunc: func() (types.Info, error) {
				return types.Info{
					Swarm: swarm.Info{
						LocalNodeState: swarm.LocalNodeStateActive,
					},
				}, nil
			},
			expectedError: "Error: swarm is not locked",
		},
		{
			name: "unlockrequest-failed",
			infoFunc: func() (types.Info, error) {
				return types.Info{
					Swarm: swarm.Info{
						LocalNodeState: swarm.LocalNodeStateLocked,
					},
				}, nil
			},
			swarmUnlockFunc: func(req swarm.UnlockRequest) error {
				return errors.Errorf("error unlocking the swarm")
			},
			expectedError: "error unlocking the swarm",
		},
	}
	for _, tc := range testCases {
		buf := new(bytes.Buffer)
		cmd := newUnlockCommand(
			test.NewFakeCli(&fakeClient{
				infoFunc:        tc.infoFunc,
				swarmUnlockFunc: tc.swarmUnlockFunc,
			}, buf))
		cmd.SetArgs(tc.args)
		cmd.SetOutput(ioutil.Discard)
		testutil.ErrorContains(t, cmd.Execute(), tc.expectedError)
	}
}

func TestSwarmUnlock(t *testing.T) {
	input := "unlockKey"
	buf := new(bytes.Buffer)
	dockerCli := test.NewFakeCli(&fakeClient{
		infoFunc: func() (types.Info, error) {
			return types.Info{
				Swarm: swarm.Info{
					LocalNodeState: swarm.LocalNodeStateLocked,
				},
			}, nil
		},
		swarmUnlockFunc: func(req swarm.UnlockRequest) error {
			if req.UnlockKey != input {
				return errors.Errorf("Invalid unlock key")
			}
			return nil
		},
	}, buf)
	dockerCli.SetIn(ioutil.NopCloser(strings.NewReader(input)))
	cmd := newUnlockCommand(dockerCli)
	assert.NoError(t, cmd.Execute())
}
