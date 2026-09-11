package swarm_test

import (
	"slices"
	"testing"

	"github.com/moby/moby/api/types/network"
	"github.com/moby/moby/api/types/swarm"
)

func TestPortConfigCompareSort(t *testing.T) {
	tests := []struct {
		doc      string
		ports    []swarm.PortConfig
		expected []swarm.PortConfig
	}{
		{
			doc: "sort ports lexicographically",
			ports: []swarm.PortConfig{
				{
					PublishedPort: 443,
					TargetPort:    443,
					Protocol:      network.TCP,
					PublishMode:   swarm.PortConfigPublishModeIngress,
				},
				{
					PublishedPort: 80,
					TargetPort:    8080,
					Protocol:      network.UDP,
					PublishMode:   swarm.PortConfigPublishModeIngress,
				},
				{
					PublishedPort: 80,
					TargetPort:    9090,
					Protocol:      network.TCP,
					PublishMode:   swarm.PortConfigPublishModeIngress,
				},
				{
					PublishedPort: 80,
					TargetPort:    8080,
					Protocol:      network.TCP,
					PublishMode:   swarm.PortConfigPublishModeIngress,
				},
				{
					PublishedPort: 80,
					TargetPort:    8080,
					Protocol:      network.TCP,
					PublishMode:   swarm.PortConfigPublishModeHost,
				},
			},
			expected: []swarm.PortConfig{
				{
					PublishedPort: 80,
					TargetPort:    8080,
					Protocol:      network.TCP,
					PublishMode:   swarm.PortConfigPublishModeHost,
				},
				{
					PublishedPort: 80,
					TargetPort:    8080,
					Protocol:      network.TCP,
					PublishMode:   swarm.PortConfigPublishModeIngress,
				},
				{
					PublishedPort: 80,
					TargetPort:    8080,
					Protocol:      network.UDP,
					PublishMode:   swarm.PortConfigPublishModeIngress,
				},
				{
					PublishedPort: 80,
					TargetPort:    9090,
					Protocol:      network.TCP,
					PublishMode:   swarm.PortConfigPublishModeIngress,
				},
				{
					PublishedPort: 443,
					TargetPort:    443,
					Protocol:      network.TCP,
					PublishMode:   swarm.PortConfigPublishModeIngress,
				},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.doc, func(t *testing.T) {
			got := slices.Clone(tc.ports)
			slices.SortFunc(got, swarm.PortConfig.Compare)
			if !slices.Equal(got, tc.expected) {
				t.Errorf("sorted ports = %#v, want %#v", got, tc.expected)
			}
		})
	}
}
