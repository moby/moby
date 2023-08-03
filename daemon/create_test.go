package daemon // import "github.com/docker/docker/daemon"

import (
	"testing"

	"github.com/docker/docker/api/types/network"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

// Test case for 35752
func TestVerifyNetworkingConfig(t *testing.T) {
	name := "mynet"
	endpoints := make(map[string]*network.EndpointSettings, 1)
	endpoints[name] = nil
	nwConfig := &network.NetworkingConfig{
		EndpointsConfig: endpoints,
	}
	err := verifyNetworkingConfig(nwConfig)
	assert.Check(t, is.Error(err, "no EndpointSettings for mynet"), "should produce an error because no EndpointSettings were passed")
}
