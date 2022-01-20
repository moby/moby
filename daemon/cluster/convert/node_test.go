package convert

import (
	"testing"

	types "github.com/docker/docker/api/types/swarm"
	swarmapi "github.com/moby/swarmkit/v2/api"
	"gotest.tools/v3/assert"
)

// TestNodeCSIInfoFromGRPC tests that conversion of the NodeCSIInfo from the
// gRPC to the Docker types is correct.
func TestNodeCSIInfoFromGRPC(t *testing.T) {
	node := &swarmapi.Node{
		ID: "someID",
		Description: &swarmapi.NodeDescription{
			CSIInfo: []*swarmapi.NodeCSIInfo{
				{
					PluginName:        "plugin1",
					NodeID:            "p1n1",
					MaxVolumesPerNode: 1,
				},
				{
					PluginName:        "plugin2",
					NodeID:            "p2n1",
					MaxVolumesPerNode: 2,
					AccessibleTopology: &swarmapi.Topology{
						Segments: map[string]string{
							"a": "1",
							"b": "2",
						},
					},
				},
			},
		},
	}

	expected := []types.NodeCSIInfo{
		{
			PluginName:        "plugin1",
			NodeID:            "p1n1",
			MaxVolumesPerNode: 1,
		},
		{
			PluginName:        "plugin2",
			NodeID:            "p2n1",
			MaxVolumesPerNode: 2,
			AccessibleTopology: &types.Topology{
				Segments: map[string]string{
					"a": "1",
					"b": "2",
				},
			},
		},
	}

	actual := NodeFromGRPC(*node)

	assert.DeepEqual(t, actual.Description.CSIInfo, expected)
}
