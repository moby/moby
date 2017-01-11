package opts

import (
	"testing"

	"github.com/docker/docker/api/types/swarm"
	"github.com/docker/docker/pkg/testutil/assert"
)

func TestPortOptValidSimpleSyntax(t *testing.T) {
	testCases := []struct {
		value    string
		expected []swarm.PortConfig
	}{
		{
			value: "80",
			expected: []swarm.PortConfig{
				{
					Protocol:    "tcp",
					TargetPort:  80,
					PublishMode: swarm.PortConfigPublishModeIngress,
				},
			},
		},
		{
			value: "80:8080",
			expected: []swarm.PortConfig{
				{
					Protocol:      "tcp",
					TargetPort:    8080,
					PublishedPort: 80,
					PublishMode:   swarm.PortConfigPublishModeIngress,
				},
			},
		},
		{
			value: "8080:80/tcp",
			expected: []swarm.PortConfig{
				{
					Protocol:      "tcp",
					TargetPort:    80,
					PublishedPort: 8080,
					PublishMode:   swarm.PortConfigPublishModeIngress,
				},
			},
		},
		{
			value: "80:8080/udp",
			expected: []swarm.PortConfig{
				{
					Protocol:      "udp",
					TargetPort:    8080,
					PublishedPort: 80,
					PublishMode:   swarm.PortConfigPublishModeIngress,
				},
			},
		},
		{
			value: "80-81:8080-8081/tcp",
			expected: []swarm.PortConfig{
				{
					Protocol:      "tcp",
					TargetPort:    8080,
					PublishedPort: 80,
					PublishMode:   swarm.PortConfigPublishModeIngress,
				},
				{
					Protocol:      "tcp",
					TargetPort:    8081,
					PublishedPort: 81,
					PublishMode:   swarm.PortConfigPublishModeIngress,
				},
			},
		},
		{
			value: "80-82:8080-8082/udp",
			expected: []swarm.PortConfig{
				{
					Protocol:      "udp",
					TargetPort:    8080,
					PublishedPort: 80,
					PublishMode:   swarm.PortConfigPublishModeIngress,
				},
				{
					Protocol:      "udp",
					TargetPort:    8081,
					PublishedPort: 81,
					PublishMode:   swarm.PortConfigPublishModeIngress,
				},
				{
					Protocol:      "udp",
					TargetPort:    8082,
					PublishedPort: 82,
					PublishMode:   swarm.PortConfigPublishModeIngress,
				},
			},
		},
	}
	for _, tc := range testCases {
		var port PortOpt
		assert.NilError(t, port.Set(tc.value))
		assert.Equal(t, len(port.Value()), len(tc.expected))
		for _, expectedPortConfig := range tc.expected {
			assertContains(t, port.Value(), expectedPortConfig)
		}
	}
}

func TestPortOptValidComplexSyntax(t *testing.T) {
	testCases := []struct {
		value    string
		expected []swarm.PortConfig
	}{
		{
			value: "target=80",
			expected: []swarm.PortConfig{
				{
					TargetPort:  80,
					Protocol:    "tcp",
					PublishMode: swarm.PortConfigPublishModeIngress,
				},
			},
		},
		{
			value: "target=80,protocol=tcp",
			expected: []swarm.PortConfig{
				{
					Protocol:    "tcp",
					TargetPort:  80,
					PublishMode: swarm.PortConfigPublishModeIngress,
				},
			},
		},
		{
			value: "target=80,published=8080,protocol=tcp",
			expected: []swarm.PortConfig{
				{
					Protocol:      "tcp",
					TargetPort:    80,
					PublishedPort: 8080,
					PublishMode:   swarm.PortConfigPublishModeIngress,
				},
			},
		},
		{
			value: "published=80,target=8080,protocol=tcp",
			expected: []swarm.PortConfig{
				{
					Protocol:      "tcp",
					TargetPort:    8080,
					PublishedPort: 80,
					PublishMode:   swarm.PortConfigPublishModeIngress,
				},
			},
		},
		{
			value: "target=80,published=8080,protocol=tcp,mode=host",
			expected: []swarm.PortConfig{
				{
					Protocol:      "tcp",
					TargetPort:    80,
					PublishedPort: 8080,
					PublishMode:   "host",
				},
			},
		},
		{
			value: "target=80,published=8080,mode=host",
			expected: []swarm.PortConfig{
				{
					TargetPort:    80,
					PublishedPort: 8080,
					PublishMode:   "host",
					Protocol:      "tcp",
				},
			},
		},
		{
			value: "target=80,published=8080,mode=ingress",
			expected: []swarm.PortConfig{
				{
					TargetPort:    80,
					PublishedPort: 8080,
					PublishMode:   "ingress",
					Protocol:      "tcp",
				},
			},
		},
	}
	for _, tc := range testCases {
		var port PortOpt
		assert.NilError(t, port.Set(tc.value))
		assert.Equal(t, len(port.Value()), len(tc.expected))
		for _, expectedPortConfig := range tc.expected {
			assertContains(t, port.Value(), expectedPortConfig)
		}
	}
}

func TestPortOptInvalidComplexSyntax(t *testing.T) {
	testCases := []struct {
		value         string
		expectedError string
	}{
		{
			value:         "invalid,target=80",
			expectedError: "invalid field",
		},
		{
			value:         "invalid=field",
			expectedError: "invalid field",
		},
		{
			value:         "protocol=invalid",
			expectedError: "invalid protocol value",
		},
		{
			value:         "target=invalid",
			expectedError: "invalid syntax",
		},
		{
			value:         "published=invalid",
			expectedError: "invalid syntax",
		},
		{
			value:         "mode=invalid",
			expectedError: "invalid publish mode value",
		},
		{
			value:         "published=8080,protocol=tcp,mode=ingress",
			expectedError: "missing mandatory field",
		},
		{
			value:         `target=80,protocol="tcp,mode=ingress"`,
			expectedError: "non-quoted-field",
		},
		{
			value:         `target=80,"protocol=tcp,mode=ingress"`,
			expectedError: "invalid protocol value",
		},
	}
	for _, tc := range testCases {
		var port PortOpt
		assert.Error(t, port.Set(tc.value), tc.expectedError)
	}
}

func assertContains(t *testing.T, portConfigs []swarm.PortConfig, expected swarm.PortConfig) {
	var contains = false
	for _, portConfig := range portConfigs {
		if portConfig == expected {
			contains = true
			break
		}
	}
	if !contains {
		t.Errorf("expected %v to contain %v, did not", portConfigs, expected)
	}
}
