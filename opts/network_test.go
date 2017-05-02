package opts

import (
	"github.com/docker/docker/api/types/swarm"
	"github.com/docker/docker/pkg/testutil"
	"github.com/stretchr/testify/assert"
	"strings"
	"testing"
)

func TestNetworkOptLegacySyntax(t *testing.T) {
	testCases := []struct {
		value    string
		expected []swarm.NetworkAttachmentConfig
	}{
		{
			value: "docknet1",
			expected: []swarm.NetworkAttachmentConfig{
				{
					Target: "docknet1",
				},
			},
		},
	}
	for _, tc := range testCases {
		var network NetworkOpt
		assert.NoError(t, network.Set(tc.value))
		assert.Len(t, network.Value(), len(tc.expected))
		for _, expectedNetConfig := range tc.expected {
			verifyNetworkOpt(t, network.Value(), expectedNetConfig)
		}
	}
}

func TestNetworkOptCompleteSyntax(t *testing.T) {
	testCases := []struct {
		value    string
		expected []swarm.NetworkAttachmentConfig
	}{
		{
			value: "name=docknet1,alias=web",
			expected: []swarm.NetworkAttachmentConfig{
				{
					Target:  "docknet1",
					Aliases: []string{"web"},
				},
			},
		},
		{
			value: "name=docknet1,alias=web1,alias=web2",
			expected: []swarm.NetworkAttachmentConfig{
				{
					Target:  "docknet1",
					Aliases: []string{"web1", "web2"},
				},
			},
		},
		{
			value: "name=docknet1",
			expected: []swarm.NetworkAttachmentConfig{
				{
					Target:  "docknet1",
					Aliases: []string{},
				},
			},
		},
	}
	for _, tc := range testCases {
		var network NetworkOpt
		assert.NoError(t, network.Set(tc.value))
		assert.Len(t, network.Value(), len(tc.expected))
		for _, expectedNetConfig := range tc.expected {
			verifyNetworkOpt(t, network.Value(), expectedNetConfig)
		}
	}
}

func TestNetworkOptInvalidSyntax(t *testing.T) {
	testCases := []struct {
		value         string
		expectedError string
	}{
		{
			value:         "invalidField=docknet1",
			expectedError: "invalid field",
		},
		{
			value:         "network=docknet1,invalid=web",
			expectedError: "invalid field",
		},
	}
	for _, tc := range testCases {
		var network NetworkOpt
		testutil.ErrorContains(t, network.Set(tc.value), tc.expectedError)
	}
}

func verifyNetworkOpt(t *testing.T, netConfigs []swarm.NetworkAttachmentConfig, expected swarm.NetworkAttachmentConfig) {
	var contains = false
	for _, netConfig := range netConfigs {
		if netConfig.Target == expected.Target {
			if strings.Join(netConfig.Aliases, ",") == strings.Join(expected.Aliases, ",") {
				contains = true
				break
			}
		}
	}
	if !contains {
		t.Errorf("expected %v to contain %v, did not", netConfigs, expected)
	}
}
