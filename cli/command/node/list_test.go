package node

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/swarm"
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
		cmd := newListCommand(&fakeCli{
			out: buf,
			client: &fakeClient{
				nodeListFunc: tc.nodeListFunc,
				infoFunc:     tc.infoFunc,
			},
		})
		assert.Error(t, cmd.Execute(), tc.expectedError)
	}
}

func TestNodeList(t *testing.T) {
	buf := new(bytes.Buffer)
	cmd := newListCommand(&fakeCli{
		out: buf,
		client: &fakeClient{
			nodeListFunc: func() ([]swarm.Node, error) {
				return []swarm.Node{
					aNode("nodeID1").hostname("nodeHostname1").manager().leader().build(),
					aNode("nodeID2").hostname("nodeHostname2").manager().build(),
					aNode("nodeID3").hostname("nodeHostname3").build(),
				}, nil
			},
			infoFunc: func() (types.Info, error) {
				return types.Info{
					Swarm: swarm.Info{
						NodeID: "nodeID1",
					},
				}, nil
			},
		},
	})
	assert.NilError(t, cmd.Execute())
	assert.Contains(t, buf.String(), `nodeID1 *  nodeHostname1  Ready   Active        Leader`)
	assert.Contains(t, buf.String(), `nodeID2    nodeHostname2  Ready   Active        Reachable`)
	assert.Contains(t, buf.String(), `nodeID3    nodeHostname3  Ready   Active`)
}

func TestNodeListQuietShouldOnlyPrintIDs(t *testing.T) {
	buf := new(bytes.Buffer)
	cmd := newListCommand(&fakeCli{
		out: buf,
		client: &fakeClient{
			nodeListFunc: func() ([]swarm.Node, error) {
				return []swarm.Node{
					aNode("nodeID1").build(),
				}, nil
			},
		},
	})
	cmd.Flags().Set("quiet", "true")
	assert.NilError(t, cmd.Execute())
	assert.Contains(t, buf.String(), "nodeID")
}

// Test case for #24090
func TestNodeListContainsHostname(t *testing.T) {
	buf := new(bytes.Buffer)
	cmd := newListCommand(&fakeCli{
		out:    buf,
		client: &fakeClient{},
	})
	assert.NilError(t, cmd.Execute())
	assert.Contains(t, buf.String(), "HOSTNAME")
}
