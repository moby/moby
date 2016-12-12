package node

import (
	"bytes"
	"fmt"
	"testing"

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
		cmd := newRemoveCommand(&fakeCli{
			out: buf,
			client: &fakeClient{
				nodeRemoveFunc: tc.nodeRemoveFunc,
			},
		})
		cmd.SetArgs(tc.args)
		assert.Error(t, cmd.Execute(), tc.expectedError)
	}
}

func TestNodeRemoveMultipleNode(t *testing.T) {
	buf := new(bytes.Buffer)
	cmd := newRemoveCommand(&fakeCli{
		out:    buf,
		client: &fakeClient{},
	})
	cmd.SetArgs([]string{"nodeID1", "nodeID2"})
	assert.NilError(t, cmd.Execute())
}
