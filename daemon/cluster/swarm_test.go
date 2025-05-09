package cluster

import (
	"testing"

	"github.com/docker/docker/api/types/swarm"
	"gotest.tools/v3/assert"
)

func TestClusterUpdateNameInvalid(t *testing.T) {
	c := &Cluster{}

	err := c.Update(1, swarm.Spec{Annotations: swarm.Annotations{Name: "whoops"}}, swarm.UpdateFlags{})
	assert.Error(t, err, `invalid Name "whoops": swarm spec must be named "default"`)
}
