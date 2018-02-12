package config

import (
	"sort"
	"testing"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	swarmtypes "github.com/docker/docker/api/types/swarm"
	"github.com/docker/docker/client"
	"github.com/docker/docker/integration/internal/swarm"
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
