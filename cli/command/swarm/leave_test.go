package swarm

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"strings"
	"testing"

	"github.com/docker/docker/cli/test"
	"github.com/docker/docker/pkg/testutil/assert"
)

func TestSwarmLeaveShouldReturnAnErrorIfAPIfail(t *testing.T) {
	testCases := []struct {
		name           string
		args           []string
		swarmLeaveFunc func() error
		expectedError  string
	}{
		{
			name:          "too-much-args",
			args:          []string{"foo"},
			expectedError: "accepts no argument(s)",
		},
		{
			name: "leave-failed",
			swarmLeaveFunc: func() error {
				return fmt.Errorf("error leaving the swarm")
			},
			expectedError: "error leaving the swarm",
		},
	}
	for _, tc := range testCases {
		buf := new(bytes.Buffer)
		cmd := newLeaveCommand(
			test.NewFakeCli(&fakeClient{
				swarmLeaveFunc: tc.swarmLeaveFunc,
			}, buf, ioutil.NopCloser(strings.NewReader(""))))
		cmd.SetArgs(tc.args)
		cmd.SetOutput(ioutil.Discard)
		assert.Error(t, cmd.Execute(), tc.expectedError)
	}
}

func TestSwarmLeave(t *testing.T) {
	buf := new(bytes.Buffer)
	cmd := newLeaveCommand(
		test.NewFakeCli(&fakeClient{}, buf, ioutil.NopCloser(strings.NewReader(""))))
	assert.NilError(t, cmd.Execute())
	assert.Equal(t, strings.TrimSpace(buf.String()), "Node left the swarm.")
}
