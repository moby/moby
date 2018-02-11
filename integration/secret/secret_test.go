package secret

import (
	"sort"
	"testing"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	swarmtypes "github.com/docker/docker/api/types/swarm"
	"github.com/docker/docker/client"
	"github.com/docker/docker/integration/internal/swarm"
	"github.com/docker/docker/internal/testutil"
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
	secretID := createSecret(ctx, t, client, testName, []byte("TESTINGDATA"), nil)

	secret, _, err := client.SecretInspectWithRaw(context.Background(), secretID)
	require.NoError(t, err)
	assert.Equal(t, secret.Spec.Name, testName)

	secret, _, err = client.SecretInspectWithRaw(context.Background(), testName)
	require.NoError(t, err)
	assert.Equal(t, secretID, secretID)
}

func TestSecretList(t *testing.T) {
	skip.If(t, testEnv.DaemonInfo.OSType != "linux")

	defer setupTest(t)()
	d := swarm.NewSwarm(t, testEnv)
	defer d.Stop(t)
	client, err := client.NewClientWithOpts(client.WithHost((d.Sock())))
	require.NoError(t, err)

	ctx := context.Background()

	testName0 := "test0"
	testName1 := "test1"
	testNames := []string{testName0, testName1}
	sort.Strings(testNames)

	// create secret test0
	createSecret(ctx, t, client, testName0, []byte("TESTINGDATA0"), map[string]string{"type": "test"})

	// create secret test1
	secret1ID := createSecret(ctx, t, client, testName1, []byte("TESTINGDATA1"), map[string]string{"type": "production"})

	names := func(entries []swarmtypes.Secret) []string {
		values := []string{}
		for _, entry := range entries {
			values = append(values, entry.Spec.Name)
		}
		sort.Strings(values)
		return values
	}

	// test by `secret ls`
	entries, err := client.SecretList(ctx, types.SecretListOptions{})
	require.NoError(t, err)
	assert.Equal(t, names(entries), testNames)

	testCases := []struct {
		filters  filters.Args
		expected []string
	}{
		// test filter by name `secret ls --filter name=xxx`
		{
			filters:  filters.NewArgs(filters.Arg("name", testName0)),
			expected: []string{testName0},
		},
		// test filter by id `secret ls --filter id=xxx`
		{
			filters:  filters.NewArgs(filters.Arg("id", secret1ID)),
			expected: []string{testName1},
		},
		// test filter by label `secret ls --filter label=xxx`
		{
			filters:  filters.NewArgs(filters.Arg("label", "type")),
			expected: testNames,
		},
		{
			filters:  filters.NewArgs(filters.Arg("label", "type=test")),
			expected: []string{testName0},
		},
		{
			filters:  filters.NewArgs(filters.Arg("label", "type=production")),
			expected: []string{testName1},
		},
	}
	for _, tc := range testCases {
		entries, err = client.SecretList(ctx, types.SecretListOptions{
			Filters: tc.filters,
		})
		require.NoError(t, err)
		assert.Equal(t, names(entries), tc.expected)

	}
}

func createSecret(ctx context.Context, t *testing.T, client client.APIClient, name string, data []byte, labels map[string]string) string {
	secret, err := client.SecretCreate(ctx, swarmtypes.SecretSpec{
		Annotations: swarmtypes.Annotations{
			Name:   name,
			Labels: labels,
		},
		Data: data,
	})
	require.NoError(t, err)
	assert.NotEqual(t, secret.ID, "")
	return secret.ID
}

func TestSecretsCreate(t *testing.T) {
	skip.If(t, testEnv.DaemonInfo.OSType != "linux")

	defer setupTest(t)()
	d := swarm.NewSwarm(t, testEnv)
	defer d.Stop(t)
	client, err := client.NewClientWithOpts(client.WithHost((d.Sock())))
	require.NoError(t, err)

	ctx := context.Background()

	testName := "test_secret"
	createSecret(ctx, t, client, testName, []byte("TESTINGDATA"), nil)
	require.NoError(t, err)

	// create an already existin secret, daemon should return a status code of 409
	_, err = client.SecretCreate(ctx, swarmtypes.SecretSpec{
		Annotations: swarmtypes.Annotations{
			Name: testName,
		},
		Data: []byte("TESTINGDATA"),
	})
	testutil.ErrorContains(t, err, "already exists")
}

func TestSecretsDelete(t *testing.T) {
	skip.If(t, testEnv.DaemonInfo.OSType != "linux")

	defer setupTest(t)()
	d := swarm.NewSwarm(t, testEnv)
	defer d.Stop(t)
	client, err := client.NewClientWithOpts(client.WithHost((d.Sock())))
	require.NoError(t, err)

	ctx := context.Background()

	testName := "test_secret"
	secretID := createSecret(ctx, t, client, testName, []byte("TESTINGDATA"), nil)
	require.NoError(t, err)

	insp, _, err := client.SecretInspectWithRaw(ctx, secretID)
	require.NoError(t, err)
	assert.Equal(t, insp.ID, secretID)

	err = client.SecretRemove(ctx, secretID)
	require.NoError(t, err)

	_, _, err = client.SecretInspectWithRaw(ctx, secretID)
	testutil.ErrorContains(t, err, "No such secret")

	err = client.SecretRemove(ctx, "non-existin")
	testutil.ErrorContains(t, err, "No such secret: non-existin")
}

func TestSecretsUpdate(t *testing.T) {
	skip.If(t, testEnv.DaemonInfo.OSType != "linux")

	defer setupTest(t)()
	d := swarm.NewSwarm(t, testEnv)
	defer d.Stop(t)
	client, err := client.NewClientWithOpts(client.WithHost((d.Sock())))
	require.NoError(t, err)

	ctx := context.Background()

	testName := "test_secret"
	secretID := createSecret(ctx, t, client, testName, []byte("TESTINGDATA"), nil)
	require.NoError(t, err)

	insp, _, err := client.SecretInspectWithRaw(ctx, secretID)
	require.NoError(t, err)
	assert.Equal(t, insp.ID, secretID)

	// test UpdateSecret with full ID
	insp.Spec.Labels = map[string]string{"test": "test1"}
	err = client.SecretUpdate(ctx, secretID, insp.Version, insp.Spec)
	require.NoError(t, err)

	insp, _, err = client.SecretInspectWithRaw(ctx, secretID)
	require.NoError(t, err)
	assert.Equal(t, insp.Spec.Labels["test"], "test1")

	// test UpdateSecret with full name
	insp.Spec.Labels = map[string]string{"test": "test2"}
	err = client.SecretUpdate(ctx, testName, insp.Version, insp.Spec)
	require.NoError(t, err)

	insp, _, err = client.SecretInspectWithRaw(ctx, secretID)
	require.NoError(t, err)
	assert.Equal(t, insp.Spec.Labels["test"], "test2")

	// test UpdateSecret with prefix ID
	insp.Spec.Labels = map[string]string{"test": "test3"}
	err = client.SecretUpdate(ctx, secretID[:1], insp.Version, insp.Spec)
	require.NoError(t, err)

	insp, _, err = client.SecretInspectWithRaw(ctx, secretID)
	require.NoError(t, err)
	assert.Equal(t, insp.Spec.Labels["test"], "test3")

	// test UpdateSecret in updating Data which is not supported in daemon
	// this test will produce an error in func UpdateSecret
	insp.Spec.Data = []byte("TESTINGDATA2")
	err = client.SecretUpdate(ctx, secretID, insp.Version, insp.Spec)
	testutil.ErrorContains(t, err, "only updates to Labels are allowed")
}
