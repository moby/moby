package swarm

import (
	"bytes"
	"io/ioutil"
	"strings"
	"testing"

	"github.com/docker/docker/cli/internal/test"
	"github.com/docker/docker/pkg/testutil/assert"
	"github.com/pkg/errors"
)

func TestSwarmLeaveErrors(t *testing.T) {
	testCases := []struct {
		name           string
		args           []string
		swarmLeaveFunc func() error
		expectedError  string
	}{
		{
			name:          "too-many-args",
			args:          []string{"foo"},
			expectedError: "accepts no argument(s)",
		},
		{
			name: "leave-failed",
			swarmLeaveFunc: func() error {
				return errors.Errorf("error leaving the swarm")
			},
			expectedError: "error leaving the swarm",
		},
	}
	for _, tc := range testCases {
		buf := new(bytes.Buffer)
		cmd := newLeaveCommand(
			test.NewFakeCli(&fakeClient{
				swarmLeaveFunc: tc.swarmLeaveFunc,
			}, buf))
		cmd.SetArgs(tc.args)
		cmd.SetOutput(ioutil.Discard)
		assert.Error(t, cmd.Execute(), tc.expectedError)
	}
}

func TestSwarmLeave(t *testing.T) {
	buf := new(bytes.Buffer)
	cmd := newLeaveCommand(
		test.NewFakeCli(&fakeClient{}, buf))
	assert.NilError(t, cmd.Execute())
	assert.Equal(t, strings.TrimSpace(buf.String()), "Node left the swarm.")
}
