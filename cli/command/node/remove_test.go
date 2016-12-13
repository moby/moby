package node

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"strings"
	"testing"

	"github.com/docker/docker/cli/test"
	"github.com/docker/docker/pkg/testutil/assert"
)

func TestNodeRemoveErrors(t *testing.T) {
	testCases := []struct {
		args           []string
		nodeRemoveFunc func() error
		expectedError  string
	}{
		{
			expectedError: "requires at least 1 argument",
		},
		{
			args: []string{"nodeID"},
			nodeRemoveFunc: func() error {
				return fmt.Errorf("error removing the node")
			},
			expectedError: "error removing the node",
		},
	}
	for _, tc := range testCases {
		buf := new(bytes.Buffer)
		cmd := newRemoveCommand(
			test.NewFakeCli(&fakeClient{
				nodeRemoveFunc: tc.nodeRemoveFunc,
			}, buf, ioutil.NopCloser(strings.NewReader(""))))
		cmd.SetArgs(tc.args)
		assert.Error(t, cmd.Execute(), tc.expectedError)
	}
}

func TestNodeRemoveMultipleNode(t *testing.T) {
	buf := new(bytes.Buffer)
	cmd := newRemoveCommand(test.NewFakeCli(&fakeClient{}, buf, ioutil.NopCloser(strings.NewReader(""))))
	cmd.SetArgs([]string{"nodeID1", "nodeID2"})
	assert.NilError(t, cmd.Execute())
}
