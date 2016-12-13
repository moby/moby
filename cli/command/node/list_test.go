package node

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"strings"
	"testing"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/swarm"
	"github.com/docker/docker/cli/test"
	"github.com/docker/docker/cli/test/builder"
	"github.com/docker/docker/pkg/testutil/assert"
)

func TestNodeListShouldReturnAnErrorIfAPIfail(t *testing.T) {
	testCases := []struct {
		nodeListFunc  func() ([]swarm.Node, error)
		infoFunc      func() (types.Info, error)
		expectedError string
	}{
		{
			nodeListFunc: func() ([]swarm.Node, error) {
				return []swarm.Node{}, fmt.Errorf("error listing nodes")
			},
			expectedError: "error listing nodes",
		},
		{
			nodeListFunc: func() ([]swarm.Node, error) {
				return []swarm.Node{
					{
						ID: "nodeID",
					},
				}, nil
			},
			infoFunc: func() (types.Info, error) {
				return types.Info{}, fmt.Errorf("error asking for node info")
			},
			expectedError: "error asking for node info",
		},
	}
	for _, tc := range testCases {
		buf := new(bytes.Buffer)
		cmd := newListCommand(
			test.NewFakeCli(&fakeClient{
				nodeListFunc: tc.nodeListFunc,
				infoFunc:     tc.infoFunc,
			}, buf, ioutil.NopCloser(strings.NewReader(""))))
		assert.Error(t, cmd.Execute(), tc.expectedError)
	}
}

func TestNodeList(t *testing.T) {
	buf := new(bytes.Buffer)
	cmd := newListCommand(
		test.NewFakeCli(&fakeClient{
			nodeListFunc: func() ([]swarm.Node, error) {
				return []swarm.Node{
					builder.ANode("nodeID1").Hostname("nodeHostname1").Manager().Leader().Build(),
					builder.ANode("nodeID2").Hostname("nodeHostname2").Manager().Build(),
					builder.ANode("nodeID3").Hostname("nodeHostname3").Build(),
				}, nil
			},
			infoFunc: func() (types.Info, error) {
				return types.Info{
					Swarm: swarm.Info{
						NodeID: "nodeID1",
					},
				}, nil
			},
		}, buf, ioutil.NopCloser(strings.NewReader(""))))
	assert.NilError(t, cmd.Execute())
	assert.Contains(t, buf.String(), `nodeID1 *  nodeHostname1  Ready   Active        Leader`)
	assert.Contains(t, buf.String(), `nodeID2    nodeHostname2  Ready   Active        Reachable`)
	assert.Contains(t, buf.String(), `nodeID3    nodeHostname3  Ready   Active`)
}

func TestNodeListQuietShouldOnlyPrintIDs(t *testing.T) {
	buf := new(bytes.Buffer)
	cmd := newListCommand(
		test.NewFakeCli(&fakeClient{
			nodeListFunc: func() ([]swarm.Node, error) {
				return []swarm.Node{
					builder.ANode("nodeID1").Build(),
				}, nil
			},
		}, buf, ioutil.NopCloser(strings.NewReader(""))))
	cmd.Flags().Set("quiet", "true")
	assert.NilError(t, cmd.Execute())
	assert.Contains(t, buf.String(), "nodeID")
}

// Test case for #24090
func TestNodeListContainsHostname(t *testing.T) {
	buf := new(bytes.Buffer)
	cmd := newListCommand(test.NewFakeCli(&fakeClient{}, buf, ioutil.NopCloser(strings.NewReader(""))))
	assert.NilError(t, cmd.Execute())
	assert.Contains(t, buf.String(), "HOSTNAME")
}
