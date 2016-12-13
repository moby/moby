package builder

import (
	"time"

	"github.com/docker/docker/api/types/swarm"
)

// ASwarm creates a swarm builder with default values for a swarm.
// Use the Build method to get the built swarm.
func ASwarm() *SwarmBuilder {
	t1 := time.Date(2009, time.November, 10, 23, 0, 0, 0, time.UTC)
	return &SwarmBuilder{
		swarm: swarm.Swarm{
			ClusterInfo: swarm.ClusterInfo{
				ID: "swarm",
				Meta: swarm.Meta{
					CreatedAt: t1,
				},
				Spec: swarm.Spec{},
			},
			JoinTokens: swarm.JoinTokens{
				Worker:  "worker-join-token",
				Manager: "manager-join-token",
			},
		},
	}
}

// SwarmBuilder holds a swarm to be built
type SwarmBuilder struct {
	swarm swarm.Swarm
}

// Autolock set the swarm into autolock mode
func (b *SwarmBuilder) Autolock() *SwarmBuilder {
	b.swarm.Spec.EncryptionConfig.AutoLockManagers = true
	return b
}

// Build returns the built swarm
func (b *SwarmBuilder) Build() swarm.Swarm {
	return b.swarm
}
