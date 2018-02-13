package config

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

func TestConfigList(t *testing.T) {
	skip.If(t, testEnv.DaemonInfo.OSType != "linux")

	defer setupTest(t)()
	d := swarm.NewSwarm(t, testEnv)
	defer d.Stop(t)
	client, err := client.NewClientWithOpts(client.WithHost((d.Sock())))
	require.NoError(t, err)

	ctx := context.Background()

	// This test case is ported from the original TestConfigsEmptyList
	configs, err := client.ConfigList(ctx, types.ConfigListOptions{})
	require.NoError(t, err)
	assert.Equal(t, len(configs), 0)

	testName0 := "test0"
	testName1 := "test1"
	testNames := []string{testName0, testName1}
	sort.Strings(testNames)

	// create config test0
	createConfig(ctx, t, client, testName0, []byte("TESTINGDATA0"), map[string]string{"type": "test"})

	config1ID := createConfig(ctx, t, client, testName1, []byte("TESTINGDATA1"), map[string]string{"type": "production"})

	names := func(entries []swarmtypes.Config) []string {
		values := []string{}
		for _, entry := range entries {
			values = append(values, entry.Spec.Name)
		}
		sort.Strings(values)
		return values
	}

	// test by `config ls`
	entries, err := client.ConfigList(ctx, types.ConfigListOptions{})
	require.NoError(t, err)
	assert.Equal(t, names(entries), testNames)

	testCases := []struct {
		filters  filters.Args
		expected []string
	}{
		// test filter by name `config ls --filter name=xxx`
		{
			filters:  filters.NewArgs(filters.Arg("name", testName0)),
			expected: []string{testName0},
		},
		// test filter by id `config ls --filter id=xxx`
		{
			filters:  filters.NewArgs(filters.Arg("id", config1ID)),
			expected: []string{testName1},
		},
		// test filter by label `config ls --filter label=xxx`
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
		entries, err = client.ConfigList(ctx, types.ConfigListOptions{
			Filters: tc.filters,
		})
		require.NoError(t, err)
		assert.Equal(t, names(entries), tc.expected)

	}
}

func createConfig(ctx context.Context, t *testing.T, client client.APIClient, name string, data []byte, labels map[string]string) string {
	config, err := client.ConfigCreate(ctx, swarmtypes.ConfigSpec{
		Annotations: swarmtypes.Annotations{
			Name:   name,
			Labels: labels,
		},
		Data: data,
	})
	require.NoError(t, err)
	assert.NotEqual(t, config.ID, "")
	return config.ID
}

func TestConfigsCreateAndDelete(t *testing.T) {
	skip.If(t, testEnv.DaemonInfo.OSType != "linux")

	defer setupTest(t)()
	d := swarm.NewSwarm(t, testEnv)
	defer d.Stop(t)
	client, err := client.NewClientWithOpts(client.WithHost((d.Sock())))
	require.NoError(t, err)

	ctx := context.Background()

	testName := "test_config"

	// This test case is ported from the original TestConfigsCreate
	configID := createConfig(ctx, t, client, testName, []byte("TESTINGDATA"), nil)

	insp, _, err := client.ConfigInspectWithRaw(ctx, configID)
	require.NoError(t, err)
	assert.Equal(t, insp.Spec.Name, testName)

	// This test case is ported from the original TestConfigsDelete
	err = client.ConfigRemove(ctx, configID)
	require.NoError(t, err)

	insp, _, err = client.ConfigInspectWithRaw(ctx, configID)
	testutil.ErrorContains(t, err, "No such config")
}

func TestConfigsUpdate(t *testing.T) {
	skip.If(t, testEnv.DaemonInfo.OSType != "linux")

	defer setupTest(t)()
	d := swarm.NewSwarm(t, testEnv)
	defer d.Stop(t)
	client, err := client.NewClientWithOpts(client.WithHost((d.Sock())))
	require.NoError(t, err)

	ctx := context.Background()

	testName := "test_config"

	// This test case is ported from the original TestConfigsCreate
	configID := createConfig(ctx, t, client, testName, []byte("TESTINGDATA"), nil)

	insp, _, err := client.ConfigInspectWithRaw(ctx, configID)
	require.NoError(t, err)
	assert.Equal(t, insp.ID, configID)

	// test UpdateConfig with full ID
	insp.Spec.Labels = map[string]string{"test": "test1"}
	err = client.ConfigUpdate(ctx, configID, insp.Version, insp.Spec)
	require.NoError(t, err)

	insp, _, err = client.ConfigInspectWithRaw(ctx, configID)
	require.NoError(t, err)
	assert.Equal(t, insp.Spec.Labels["test"], "test1")

	// test UpdateConfig with full name
	insp.Spec.Labels = map[string]string{"test": "test2"}
	err = client.ConfigUpdate(ctx, testName, insp.Version, insp.Spec)
	require.NoError(t, err)

	insp, _, err = client.ConfigInspectWithRaw(ctx, configID)
	require.NoError(t, err)
	assert.Equal(t, insp.Spec.Labels["test"], "test2")

	// test UpdateConfig with prefix ID
	insp.Spec.Labels = map[string]string{"test": "test3"}
	err = client.ConfigUpdate(ctx, configID[:1], insp.Version, insp.Spec)
	require.NoError(t, err)

	insp, _, err = client.ConfigInspectWithRaw(ctx, configID)
	require.NoError(t, err)
	assert.Equal(t, insp.Spec.Labels["test"], "test3")

	// test UpdateConfig in updating Data which is not supported in daemon
	// this test will produce an error in func UpdateConfig
	insp.Spec.Data = []byte("TESTINGDATA2")
	err = client.ConfigUpdate(ctx, configID, insp.Version, insp.Spec)
	testutil.ErrorContains(t, err, "only updates to Labels are allowed")
}
