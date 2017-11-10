package swarm

import (
	"testing"

	"github.com/docker/docker/api/types/swarm"
	"github.com/docker/docker/testutil/request"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/skip"
)

func TestSwarmUpdate(t *testing.T) {
	skip.If(t, testEnv.IsLocalDaemon())
	ctx := setupTest(t)

	client := request.NewAPIClient(t)

	_, err := client.SwarmInit(ctx, swarm.InitRequest{ListenAddr: "127.0.0.1:2478", AdvertiseAddr: "127.0.0.1"})
	assert.NilError(t, err)

	swarmInspect, err := client.SwarmInspect(ctx)
	assert.NilError(t, err)
	err = client.SwarmUpdate(ctx, swarmInspect.Version, swarm.Spec{Annotations: swarm.Annotations{Name: "whoops"}}, swarm.UpdateFlags{})
	assert.Error(t, err, `Error response from daemon: invalid Name "whoops": swarm spec must be named "default"`)
}
