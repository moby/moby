package swarm

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"strings"
	"testing"

	"github.com/docker/docker/api/types/swarm"
	"github.com/docker/docker/cli/test"
	"github.com/docker/docker/pkg/testutil/assert"
)

func TestSwarmUnlockShouldReturnAnErrorIfAPIfail(t *testing.T) {
	testCases := []struct {
		name            string
		args            []string
		input           string
		swarmUnlockFunc func(req swarm.UnlockRequest) error
		expectedError   string
	}{
		{
			name:          "too-much-args",
			args:          []string{"foo"},
			expectedError: "accepts no argument(s)",
		},
		{
			name: "leave-failed",
			swarmUnlockFunc: func(req swarm.UnlockRequest) error {
				return fmt.Errorf("error unlocking the swarm")
			},
			expectedError: "error unlocking the swarm",
		},
	}
	for _, tc := range testCases {
		buf := new(bytes.Buffer)
		cmd := newUnlockCommand(
			test.NewFakeCli(&fakeClient{
				swarmUnlockFunc: tc.swarmUnlockFunc,
			}, buf, ioutil.NopCloser(strings.NewReader(tc.input))))
		cmd.SetArgs(tc.args)
		cmd.SetOutput(ioutil.Discard)
		assert.Error(t, cmd.Execute(), tc.expectedError)
	}
}

func TestSwarmUnlock(t *testing.T) {
	input := "unlockKey"
	buf := new(bytes.Buffer)
	cmd := newUnlockCommand(
		test.NewFakeCli(&fakeClient{
			swarmUnlockFunc: func(req swarm.UnlockRequest) error {
				if req.UnlockKey != input {
					return fmt.Errorf("Invalid unlock key")
				}
				return nil
			},
		}, buf, ioutil.NopCloser(strings.NewReader(input))))
	assert.NilError(t, cmd.Execute())
}
