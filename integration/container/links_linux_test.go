package container // import "github.com/docker/docker/integration/container"

import (
	"os"
	"testing"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/integration/internal/container"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
	"gotest.tools/v3/skip"
)

func TestLinksEtcHostsContentMatch(t *testing.T) {
	skip.If(t, testEnv.IsRemoteDaemon)
	skip.If(t, testEnv.IsRootless, "rootless mode has different view of /etc/hosts")

	hosts, err := os.ReadFile("/etc/hosts")
	skip.If(t, os.IsNotExist(err))

	ctx := setupTest(t)

	apiClient := testEnv.APIClient()

	cID := container.Run(ctx, t, apiClient, container.WithNetworkMode("host"))
	res, err := container.Exec(ctx, apiClient, cID, []string{"cat", "/etc/hosts"})
	assert.NilError(t, err)
	assert.Assert(t, is.Len(res.Stderr(), 0))
	assert.Equal(t, 0, res.ExitCode)

	assert.Check(t, is.Equal(string(hosts), res.Stdout()))
}

func TestLinksContainerNames(t *testing.T) {
	skip.If(t, testEnv.DaemonInfo.OSType == "windows")

	ctx := setupTest(t)
	apiClient := testEnv.APIClient()

	containerA := "first_" + t.Name()
	containerB := "second_" + t.Name()
	container.Run(ctx, t, apiClient, container.WithName(containerA))
	container.Run(ctx, t, apiClient, container.WithName(containerB), container.WithLinks(containerA+":"+containerA))

	containers, err := apiClient.ContainerList(ctx, types.ContainerListOptions{
		Filters: filters.NewArgs(filters.Arg("name", containerA)),
	})
	assert.NilError(t, err)
	assert.Check(t, is.Equal(1, len(containers)))
	assert.Check(t, is.DeepEqual([]string{"/" + containerA, "/" + containerB + "/" + containerA}, containers[0].Names))
}
