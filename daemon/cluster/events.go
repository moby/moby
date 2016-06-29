package cluster

import (
	"github.com/docker/engine-api/types/events"
	swarmagent "github.com/docker/swarmkit/agent"
)

// LogSwarmEvent generates an event related to the Swarm.
func (c *Cluster) LogSwarmEvent(node *swarmagent.Node, action string, attributes map[string]string) {
	c.config.Backend.LogEventWithAttributes(events.SwarmEventType, node.NodeID(), action, attributes)
}
