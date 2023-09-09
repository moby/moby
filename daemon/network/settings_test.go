package network // import "github.com/docker/docker/daemon/network"

import (
	"testing"

	networktypes "github.com/docker/docker/api/types/network"
	"gotest.tools/v3/assert"
)

func TestSortNetworks(t *testing.T) {
	networks := map[string]*EndpointSettings{
		"net1": &EndpointSettings{
			EndpointSettings: &networktypes.EndpointSettings{
				Priority: 1,
			},
		},
		"net100": &EndpointSettings{
			EndpointSettings: &networktypes.EndpointSettings{
				Priority: 100,
			},
		},
		"net50": &EndpointSettings{
			EndpointSettings: &networktypes.EndpointSettings{
				Priority: 50,
			},
		},
	}

	sortedNets := SortNetworks(networks)
	assert.DeepEqual(t, sortedNets, []string{"net100", "net50", "net1"})
}
