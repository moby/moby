package secret

import (
	"testing"

	swarmtypes "github.com/docker/docker/api/types/swarm"
	"github.com/docker/docker/client"
	"github.com/docker/docker/integration/util/swarm"
	"github.com/gotestyourself/gotestyourself/skip"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/net/context"
)

func TestSecretInspect(t *testing.T) {
	skip.If(t, testEnv.DaemonInfo.OSType != "linux")

	defer setupTest(t)()
	d := swarm.NewSwarm(t, testEnv)
	defer d.Stop(t)
	client, err := client.NewClientWithOpts(client.WithHost((d.Sock())))
	require.NoError(t, err)

	ctx := context.Background()

	testName := "test_secret"
	secretResp, err := client.SecretCreate(ctx, swarmtypes.SecretSpec{
		Annotations: swarmtypes.Annotations{
			Name: testName,
		},
		Data: []byte("TESTINGDATA"),
	})
	require.NoError(t, err)
	assert.NotEqual(t, secretResp.ID, "")

	secret, _, err := client.SecretInspectWithRaw(context.Background(), secretResp.ID)
	require.NoError(t, err)
	assert.Equal(t, secret.Spec.Name, testName)

	secret, _, err = client.SecretInspectWithRaw(context.Background(), testName)
	require.NoError(t, err)
	assert.Equal(t, secret.ID, secretResp.ID)
}
